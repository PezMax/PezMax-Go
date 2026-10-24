export function boundedHistory(history) {
  let result = [...history]
  while (result.length && (result.length > 10 || JSON.stringify(result).length > 20000)) {
    const nextUser = result.findIndex((message, index) => index > 0 && message.role === 'user')
    if (nextUser < 0) return []
    result = result.slice(nextUser)
  }
  return result
}

// Tool execution is deliberately read-only and bounded, independent of prompts.
export async function runResourceAssistant({ history, question, complete, search, signal, onProgress = () => {} }) {
  const messages = [...boundedHistory(history), { role: 'user', content: question }]
  const resources = new Map()
  let searches = 0
  const checkStop = () => { if (signal?.aborted) throw new Error('已停止生成') }
  for (let round = 0; round < 5; round++) {
    checkStop()
    onProgress({ status: round ? '正在整理检索结果…' : '正在理解你的需求…' })
    const message = await complete(messages)
    checkStop()
    if (message?.role !== 'assistant' || typeof message.content !== 'string') throw new Error('模型返回格式不正确，请重试')
    messages.push(message)
    if (!message.tool_calls?.length) {
      return { messages, content: message.content || '未收到文字回复，可调整条件重试。', resources: [...resources.values()] }
    }
    if (message.tool_calls.length > 4) throw new Error('模型请求的搜索次数过多')
    for (const call of message.tool_calls) {
      checkStop()
      let result
      if (call.function?.name !== 'search_resources') result = { error: '不支持此工具；只能使用 search_resources 检索目录' }
      else if (searches >= 8) result = { error: '达到本轮8次查询上限，请向用户总结已有结果' }
      else {
        searches++
        onProgress({ status: `正在检索目录 · 第 ${searches} 次查询…` })
        try {
          const input = JSON.parse(call.function.arguments)
          const found = await search(input, { signal })
          checkStop()
          for (const item of found.resources) resources.set(item.id, item)
          result = found.modelResult
          onProgress({ resources: [...resources.values()], report: found.modelResult.reports })
        } catch (error) {
          checkStop()
          // JSON parse errors may contain model-supplied text. Do not echo it.
          result = { error: error instanceof SyntaxError ? '搜索参数不是有效JSON，请修正后重试' : '搜索参数无效或目录请求失败，请核对筛选条件后重试' }
        }
      }
      messages.push({ role: 'tool', tool_call_id: call.id, content: JSON.stringify(result) })
    }
  }
  // End with a local message; never leave unmatched tool calls in history.
  const content = '已达到本轮查询上限。你可以先查看已找到的资源，或缩小范围后继续提问。'
  messages.push({ role: 'assistant', content })
  return { messages, content, resources: [...resources.values()] }
}
