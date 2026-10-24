import { SEARCH_TOOL, assistantSystemPrompt, normalizeProviderConfig } from '../../shared/assistant-protocol.mjs'

const MAX_RESPONSE_BYTES = 512 * 1024

function safeMessages(messages) {
  if (!Array.isArray(messages) || !messages.length || messages.length > 40) throw new Error('对话过长，请开始新对话')
  if (JSON.stringify(messages).length > 120000) throw new Error('对话内容过长，请开始新对话')
  return messages.map(message => {
    if (!['user', 'assistant', 'tool'].includes(message?.role)) throw new Error('对话格式不正确')
    const result = { role: message.role, content: typeof message.content === 'string' ? message.content : '' }
    if (message.role === 'tool') {
      if (typeof message.tool_call_id !== 'string' || message.tool_call_id.length > 160) throw new Error('工具结果格式不正确')
      result.tool_call_id = message.tool_call_id
    }
    if (message.role === 'assistant' && Array.isArray(message.tool_calls)) {
      if (message.tool_calls.length > 4) throw new Error('模型请求的搜索次数过多')
      result.tool_calls = message.tool_calls.map(call => {
        if (typeof call.id !== 'string' || call.id.length > 160 || typeof call.function?.arguments !== 'string' || call.function.arguments.length > 4000) throw new Error('模型工具调用格式不正确')
        return { id: call.id, type: 'function', function: { name: String(call.function.name).slice(0, 80), arguments: call.function.arguments } }
      })
    }
    return result
  })
}

// Dependencies are injected so all credential, cancellation and network paths
// can be tested with local fixtures. The renderer never receives the key.
export function createAssistantService({ readConfig, writeConfig, encryption, fetchImpl, timeoutMs = 60000 }) {
  const sessions = new Map()
  const requests = new Map()
  let saved
  function profile(scope) {
    if (typeof scope !== 'string' || !/^[a-zA-Z0-9_-]{1,80}$/.test(scope)) throw new Error('请先登录再配置找资料助手')
    if (!saved) saved = Object.assign(Object.create(null), readConfig() || {})
    if (!sessions.has(scope)) {
      const stored = saved[scope] || {}
      let apiKey = ''
      if (stored.encryptedKey && encryption.isAvailable()) {
        try { apiKey = encryption.decrypt(stored.encryptedKey) } catch { /* key belongs to another OS profile; require re-entry */ }
      }
      sessions.set(scope, { baseUrl: stored.baseUrl || '', model: stored.model || '', apiKey, remember: !!apiKey })
    }
    return sessions.get(scope)
  }
  function publicConfig(scope) {
    const config = profile(scope)
    return { baseUrl: config.baseUrl, model: config.model, hasApiKey: !!config.apiKey, remember: config.remember, canRemember: encryption.isAvailable() }
  }
  function persist(scope, config) {
    const record = { baseUrl: config.baseUrl, model: config.model }
    if (config.remember && config.apiKey) record.encryptedKey = encryption.encrypt(config.apiKey)
    const next = { ...saved, [scope]: record }
    writeConfig(next)
    saved = next
    sessions.set(scope, config)
    return publicConfig(scope)
  }
  function save(scope, input) {
    const previous = profile(scope)
    const config = normalizeProviderConfig(input)
    if (!config.apiKey && previous.baseUrl === config.baseUrl) config.apiKey = previous.apiKey
    if (!config.apiKey) throw new Error(previous.baseUrl && previous.baseUrl !== config.baseUrl ? '接口地址已更换，请重新输入对应服务商的 API Key' : '请输入 API Key')
    if (config.remember && !encryption.isAvailable()) throw new Error('当前系统不支持安全保存，请关闭“加密保存在本机”')
    return persist(scope, config)
  }
  function clear(scope) {
    const current = profile(scope)
    for (const request of requests.values()) if (request.scope === scope) request.controller.abort()
    return persist(scope, { ...current, apiKey: '', remember: false })
  }
  function cancel(requestId, owner) {
    const request = requests.get(requestId)
    if (request?.owner === owner) request.controller.abort()
  }
  function cancelOwner(owner) {
    for (const request of requests.values()) if (request.owner === owner) request.controller.abort()
  }
  function forget(scope) {
    for (const request of requests.values()) if (request.scope === scope) request.controller.abort()
    sessions.delete(scope)
  }
  async function complete(scope, { requestId, messages, test = false }, owner) {
    const config = { ...profile(scope) }
    if (!config.apiKey) throw new Error('请先在模型设置中填写 API Key')
    normalizeProviderConfig(config)
    if (typeof requestId !== 'string' || !/^[a-zA-Z0-9_-]{1,100}$/.test(requestId)) throw new Error('请求标识无效')
    if (requests.has(requestId) || [...requests.values()].some(request => request.owner === owner)) throw new Error('请等待当前请求结束，或先停止生成')
    const history = test ? [{ role: 'user', content: '请简短回复“连接成功”，不要调用工具。' }] : safeMessages(messages)
    const controller = new AbortController()
    let timedOut = false
    const timer = setTimeout(() => { timedOut = true; controller.abort() }, timeoutMs)
    requests.set(requestId, { controller, owner, scope })
    try {
      const response = await fetchImpl(`${config.baseUrl}/chat/completions`, {
        method: 'POST', redirect: 'error', signal: controller.signal,
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${config.apiKey}` },
        body: JSON.stringify({ model: config.model, messages: [{ role: 'system', content: assistantSystemPrompt() }, ...history], tools: [SEARCH_TOOL], tool_choice: test ? 'none' : 'auto', stream: false })
      })
      if (!response.ok) {
        await response.body?.cancel()
        const labels = { 400: '请求被模型服务拒绝，请确认模型支持工具调用和 Chat Completions', 401: 'API Key 无效，请检查模型设置', 403: 'API Key 没有访问该模型的权限', 404: '模型或接口路径不存在，请检查接口地址和模型 ID', 429: '模型服务额度不足或请求过于频繁，请稍后重试' }
        throw new Error(labels[response.status] || `模型服务暂不可用（HTTP ${response.status}），请稍后重试`)
      }
      if (!response.body) throw new Error('模型服务返回内容为空')
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let bytes = 0, body = ''
      try {
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          bytes += value.byteLength
          if (bytes > MAX_RESPONSE_BYTES) throw new Error('模型返回内容过大，请缩小搜索范围')
          body += decoder.decode(value, { stream: true })
        }
        body += decoder.decode()
      } finally { await reader.cancel().catch(() => {}) }
      let data
      try { data = JSON.parse(body) } catch { throw new Error('模型服务返回格式不正确，请确认接口兼容 Chat Completions') }
      const choice = data.choices?.[0]
      if (!choice?.message || choice.finish_reason === 'length') throw new Error('模型未返回完整结果，请简化需求后重试')
      const result = safeMessages([{ ...choice.message, role: 'assistant' }])[0]
      // Some providers include credentials in error text. Never echo provider
      // error bodies; also scrub a literal key from normal assistant output.
      result.content = result.content.split(config.apiKey).join('[已隐藏密钥]')
      if (result.tool_calls) for (const call of result.tool_calls) call.function.arguments = call.function.arguments.split(config.apiKey).join('[已隐藏密钥]')
      return result
    } catch (error) {
      if (controller.signal.aborted) throw new Error(timedOut ? '模型响应超时，请稍后重试' : '已停止生成')
      if (error instanceof TypeError) throw new Error('无法连接模型服务，请检查接口地址、网络或系统代理')
      // Only locally-created messages may reach IPC. fetch exceptions can
      // contain request internals, so do not expose arbitrary error strings.
      const message = error?.message || ''
      if (/^(模型|API Key|请求被模型|无法连接)/.test(message)) throw new Error(message.split(config.apiKey).join('[已隐藏密钥]'))
      throw new Error('模型请求失败，请检查设置后重试')
    } finally {
      clearTimeout(timer)
      requests.delete(requestId)
    }
  }
  return { publicConfig, save, clear, complete, cancel, cancelOwner, forget }
}
