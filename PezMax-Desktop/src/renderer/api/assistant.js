import { handleSessionExpired } from '@/utils/request'

export function onAssistantProgress(callback) {
  return window.electronAPI?.assistant?.onProgress?.(callback) || (() => {})
}

export async function assistantCall(method, ...args) {
  const bridge = window.electronAPI?.assistant
  if (typeof bridge?.[method] !== 'function') throw new Error('请在完整的桌面客户端中使用找资料助手')
  const result = await bridge[method](...args)
  if (result?.code === 'catalog_auth_expired') return handleSessionExpired()
  if (!result?.ok) throw new Error(result?.error || '助手操作失败，请重试')
  return result.data
}
