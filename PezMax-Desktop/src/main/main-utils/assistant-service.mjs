import { normalizeProviderConfig } from '../../shared/assistant-protocol.mjs'

function redact(value, secrets) {
  if (typeof value === 'string') {
    for (const secret of secrets) if (secret) value = value.split(secret).join('[已隐藏密钥]')
    return value
  }
  if (Array.isArray(value)) return value.map(item => redact(item, secrets))
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, redact(item, secrets)]))
  return value
}

// Electron owns the OS key vault and worker lifecycle. Model calls, tools,
// history validation and graph execution belong to the Python module.
export function createAssistantService({ readConfig, writeConfig, encryption, runAgent, catalogBaseUrl, timeoutMs = 180000 }) {
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
        try { apiKey = encryption.decrypt(stored.encryptedKey) } catch { /* another OS profile; require re-entry */ }
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
  async function run(scope, input, owner, onProgress) {
    const current = profile(scope)
    if (!current.apiKey) throw new Error('请先在模型设置中填写 API Key')
    const config = normalizeProviderConfig({ ...current })
    const { requestId, question, history = [], test = false, platformToken = '' } = input || {}
    if (typeof requestId !== 'string' || !/^[a-zA-Z0-9_-]{1,100}$/.test(requestId)) throw new Error('请求标识无效')
    if (requests.has(requestId) || [...requests.values()].some(request => request.owner === owner)) throw new Error('请等待当前请求结束，或先停止生成')
    if (!test && (typeof platformToken !== 'string' || !platformToken || platformToken.length > 8192 || /[\r\n\x00]/.test(platformToken))) throw new Error('请先登录再使用找资料助手')
    const controller = new AbortController()
    let timedOut = false
    const timer = setTimeout(() => { timedOut = true; controller.abort() }, timeoutMs)
    requests.set(requestId, { controller, owner, scope })
    const secrets = [config.apiKey, platformToken]
    try {
      const result = await runAgent({
        version: 1, requestId,
        provider: { baseUrl: config.baseUrl, model: config.model, apiKey: config.apiKey },
        ...(test ? { test: true } : { catalog: { baseUrl: catalogBaseUrl, token: platformToken }, history, question })
      }, { signal: controller.signal, onProgress: (event) => {
        if (!controller.signal.aborted) onProgress?.(redact(event, secrets))
      } })
      if (controller.signal.aborted) throw new Error('已停止生成')
      return redact(result, secrets)
    } catch (error) {
      if (controller.signal.aborted) throw new Error(timedOut ? '助手处理超时，请缩小范围后重试' : '已停止生成')
      const message = error?.message || ''
      const safe = new Error(/^(模型|API Key|请求|无法连接|助手|接口|对话|搜索|资源|平台|请)/.test(message)
        ? redact(message, secrets) : '助手请求失败，请检查设置后重试')
      if (error?.code === 'catalog_auth_expired') safe.code = error.code
      throw safe
    } finally {
      clearTimeout(timer)
      requests.delete(requestId)
    }
  }
  return { publicConfig, save, clear, run, cancel, cancelOwner, forget }
}
