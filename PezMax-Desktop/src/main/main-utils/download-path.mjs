import { createHash } from 'node:crypto'
import path from 'node:path'

export function safeDownloadName(value) {
  if (typeof value !== 'string') throw new Error('文件名无效')
  const name = path.win32.basename(path.posix.basename(value)).trim()
    .replace(/[<>:"/\\|?*]/g, '_').replace(/\p{Cc}/gu, '_').replace(/[. ]+$/, '')
  if (!name || name === '.' || name === '..' || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(name)) {
    throw new Error('文件名无效')
  }
  return name
}

export function downloadPartPaths(destination, url) {
  const identity = createHash('sha256').update(url).digest('hex').slice(0, 16)
  const partPath = `${destination}.${identity}.part`
  return { partPath, metaPath: `${partPath}.json` }
}

export function nextAvailableDownloadPath(destination, exists) {
  if (!exists(destination)) return destination
  const { dir, name, ext } = path.parse(destination)
  for (let index = 1; index < 10000; index++) {
    const candidate = path.join(dir, `${name} (${index})${ext}`)
    if (!exists(candidate)) return candidate
  }
  throw new Error('下载目录中同名文件过多')
}

export function strongETag(value) {
  const etag = Array.isArray(value) ? value[0] : value
  return typeof etag === 'string' && /^"[^"\r\n]+"$/.test(etag) ? etag : ''
}

export function downloadURL(value) {
  const parsed = new URL(value)
  if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) {
    throw new Error('下载地址无效')
  }
  return parsed.toString()
}
