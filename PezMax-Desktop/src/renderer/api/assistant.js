import request from '@/utils/request'
import { createResourceSearch } from '@/utils/assistant-search.mjs'

export const searchAssistantResources = createResourceSearch(request)

export async function assistantCall(method, ...args) {
  const bridge = window.electronAPI?.assistant
  if (!bridge) throw new Error('请在桌面客户端中使用找资料助手')
  const result = await bridge[method](...args)
  if (!result?.ok) throw new Error(result?.error || '助手操作失败，请重试')
  return result.data
}
