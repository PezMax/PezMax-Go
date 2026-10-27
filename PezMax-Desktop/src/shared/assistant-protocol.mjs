// Desktop settings validation only. Agent prompts, tools and orchestration live
// in PezMax-Agent; the renderer cannot submit system prompts or provider calls.
export function normalizeProviderConfig(input) {
  if (!input || typeof input !== 'object') throw new Error('请填写模型设置')
  let url
  try { url = new URL(String(input.baseUrl || '').trim()) } catch { throw new Error('请填写完整的接口地址，例如 https://服务商域名/v1') }
  const local = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
  if (url.protocol !== 'https:' && !(local && url.protocol === 'http:')) throw new Error('接口地址需使用 HTTPS；本机服务可使用 HTTP')
  if (url.username || url.password || url.search || url.hash || url.toString().length > 2048) throw new Error('接口地址不能包含账号、密码、查询参数或片段，长度不能超过 2048 字符')
  url.pathname = url.pathname.replace(/\/+$/, '').replace(/\/chat\/completions$/, '')
  const baseUrl = url.toString().replace(/\/$/, '')
  const model = String(input.model || '').trim()
  if (!model || model.length > 160 || /[\r\n]/.test(model)) throw new Error('请填写服务商提供的模型 ID（最多 160 个字符）')
  const apiKey = typeof input.apiKey === 'string' ? input.apiKey.trim() : ''
  if (apiKey.length > 4096 || /[\r\n\x00]/.test(apiKey)) throw new Error('API Key 格式不正确')
  return { baseUrl, model, apiKey, remember: input.remember === true }
}
