import { normalizeSearchArguments } from '../../shared/assistant-protocol.mjs'

const labels = { file: '资料', ebook: '电子书', bookmark: '书签' }
const text = (value, length = 160) => String(value ?? '').slice(0, length)
const validId = value => /^[1-9]\d{0,18}$/.test(String(value))
const httpUrl = value => { try { return ['https:', 'http:'].includes(new URL(value).protocol) } catch { return false } }

function resource(kind, row) {
  const rawId = kind === 'file' ? row.fileId : kind === 'ebook' ? row.ebookId : row.id
  const status = kind === 'file' ? row.fileStatus : kind === 'ebook' ? row.ebookStatus : row.status
  if (!validId(rawId) || Number(status) !== 1) return null
  const title = text(kind === 'file' ? row.fileName : kind === 'ebook' ? row.ebookName : row.title)
  if (!title) return null
  if (kind === 'bookmark' && !httpUrl(row.url)) return null
  if (kind === 'file' && (!row.fileUrl || (!httpUrl(row.fileUrl) && !/^\/(?!\/)/.test(row.fileUrl)))) return null
  const id = `${kind}-${rawId}`
  const metadata = { id, kind, title, subject: text(row.fileSubject || row.ebookSubject || row.subject, 80) }
  if (kind === 'file') Object.assign(metadata, { school: text(row.fileSchool, 80), year: Number(row.fileYear) || null, fileType: Number(row.fileType), format: text(row.fileFormat, 12) })
  if (kind === 'ebook') Object.assign(metadata, { author: text(row.author, 80), publisher: text(row.publisher, 80), format: text(row.ebookFormat, 12) })
  if (kind === 'bookmark') Object.assign(metadata, { description: text(row.description, 200), collection: text(row.collection, 80) })
  const node = kind === 'file'
    ? { type: kind, id, rawId, label: title, url: row.fileUrl, fileExt: row.fileFormat, fileInfo: row }
    : kind === 'ebook'
      ? { type: kind, id, rawId, label: title, fileExt: row.ebookFormat, originalData: row }
      : { ...row, type: kind, id, rawId, label: title }
  // node stays local; only metadata is passed to the model.
  return { ...metadata, node }
}

export function createResourceSearch(request) {
  return async function search(input, { signal } = {}) {
    const args = normalizeSearchArguments(input)
    const resources = []
    const reports = []
    for (const kind of args.kinds) {
      if (signal?.aborted) throw new Error('已停止生成')
      if (kind !== 'file' && (args.school || args.yearFrom || args.yearTo || args.fileType)) {
        reports.push({ kind, skipped: true, note: `${labels[kind]}不支持学校、年份或试卷类型筛选，本次未查询` })
        continue
      }
      const params = { keyword: args.keyword, pageSize: 50 }
      if (kind === 'file') {
        if (args.subject) params.fileSubject = args.subject
        if (args.school) params.fileSchool = args.school
        if (args.fileType) params.fileType = args.fileType
        if (args.yearFrom) params.yearFrom = args.yearFrom
        if (args.yearTo) params.yearTo = args.yearTo
      } else if (args.subject) params.subject = args.subject
      let scanned = 0, total = 0, matches = 0, malformed = 0
      const found = new Map()
      try {
        for (let pageNum = 1; pageNum <= 3; pageNum++) {
          if (signal?.aborted) throw new Error('已停止生成')
          const result = await request({ url: `/datum/${kind}/list`, method: 'get', params: { ...params, pageNum }, signal, skipErrorMsg: true })
          const page = result?.data?.rows ? result.data : result
          if (![0, 200].includes(Number(result?.code)) || !Array.isArray(page?.rows)) throw new Error('目录响应无效')
          total = Number(page.total)
          if (!Number.isFinite(total) || total < 0) throw new Error('目录数量无效')
          scanned += page.rows.length
          for (const row of page.rows) {
            const item = resource(kind, row)
            if (!item) { malformed++; continue }
            if (kind === 'file' && (
              (args.school && row.fileSchool !== args.school) || (args.subject && row.fileSubject !== args.subject) ||
              (args.fileType && Number(row.fileType) !== args.fileType) ||
              (args.yearFrom && !(Number(row.fileYear) >= args.yearFrom)) ||
              (args.yearTo && !(Number(row.fileYear) <= args.yearTo))
            )) continue
            if (kind !== 'file' && args.subject && (row.ebookSubject || row.subject) !== args.subject) continue
            found.set(item.id, item)
          }
          if (scanned >= total || !page.rows.length) break
        }
        matches = found.size
        resources.push(...[...found.values()].slice(0, 5))
        reports.push({ kind, scanned, catalogTotal: total, matchedInScanned: matches, shown: Math.min(matches, 5), partial: scanned < total || matches > 5, omittedInvalidRows: malformed })
      } catch {
        if (signal?.aborted) throw new Error('已停止生成')
        resources.push(...[...found.values()].slice(0, 5))
        reports.push({ kind, failed: true, partial: true, shown: Math.min(found.size, 5), note: `${labels[kind]}检索未完成，请稍后重试；不能据此判断没有匹配资源` })
      }
    }
    return {
      resources,
      modelResult: { filters: args, resources: resources.map(({ node, ...metadata }) => metadata), reports, note: '仅返回真实目录元数据，未读取文件正文。每类最多检查150条并展示5项，年份范围在分页前筛选；partial 表示部分结果。' }
    }
  }
}
