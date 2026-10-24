import test from 'node:test'
import assert from 'node:assert/strict'
import { createResourceSearch } from '../src/renderer/utils/assistant-search.mjs'

const file = (id, extra = {}) => ({ fileId: id, fileName: `数学试卷 ${id}`, fileUrl: `/files/${id}.pdf`, fileFormat: 'pdf', fileStatus: 1, fileSchool: '示例大学', fileSubject: '高等数学', fileYear: 2023, fileType: 1, ...extra })
const page = (rows, total = rows.length) => ({ code: 0, data: { rows, total } })

test('range filters reach the server and cards contain only approved, matching real resources', async () => {
  const calls = []
  const search = createResourceSearch(async (options) => {
    calls.push(options)
    return page([
      file(1), file(2, { fileStatus: 0 }), file(3, { fileYear: 2021 }),
      file(4, { fileSchool: '其他大学' }), file(5, { fileUrl: 'javascript:alert(1)' }), file(0)
    ])
  })
  const result = await search({ kinds: ['file'], keyword: '数学', subject: '高等数学', school: '示例大学', yearFrom: 2022, yearTo: 2024, fileType: 1 })
  assert.deepEqual(calls[0].params, { keyword: '数学', pageSize: 50, pageNum: 1, fileSubject: '高等数学', fileSchool: '示例大学', fileType: 1, yearFrom: 2022, yearTo: 2024 })
  assert.deepEqual(result.resources.map(item => item.id), ['file-1'])
  assert.equal(result.resources[0].node.fileInfo.fileId, 1)
  assert.equal(result.modelResult.resources[0].node, undefined)
  assert.equal(JSON.stringify(result.modelResult).includes('/files/'), false)
  assert.equal(calls[0].params.userId, undefined)
})

test('ebooks and bookmarks preserve their preview nodes while sending only catalog metadata', async () => {
  const calls = []
  const search = createResourceSearch(async (options) => {
    calls.push(options)
    return options.url.includes('ebook')
      ? page([{ ebookId: 8, ebookName: '线性代数', ebookStatus: 1, ebookSubject: '数学', ebookFormat: 'epub', author: '作者', privateField: 'never-forward' }])
      : { code: 200, rows: [{ id: 9, title: '数学课程', status: 1, subject: '数学', url: 'https://example.com/learn?token=private', description: '课程目录' }], total: 1 }
  })
  const result = await search({ kinds: ['ebook', 'bookmark'], keyword: '数学', subject: '数学' })
  assert.equal(calls.every(item => item.params.subject === '数学'), true)
  assert.equal(result.resources[0].node.originalData.ebookId, 8)
  assert.equal(result.resources[1].node.rawId, 9)
  assert.equal(result.resources[1].node.url.startsWith('https://'), true)
  assert.equal(JSON.stringify(result.modelResult).includes('private'), false)
})

test('search caps pagination and cards, reports truncation, and never broadens unsupported filters', async () => {
  let requests = 0
  const result = await createResourceSearch(async () => {
    requests++
    return page(Array.from({ length: 50 }, (_, index) => file((requests - 1) * 50 + index + 1)), 300)
  })({ kinds: ['file', 'ebook', 'bookmark'], keyword: '数学', yearFrom: 2022 })
  assert.equal(requests, 3)
  assert.equal(result.resources.length, 5)
  assert.equal(result.modelResult.reports[0].partial, true)
  assert.equal(result.modelResult.reports[0].scanned, 150)
  assert.equal(result.modelResult.reports.slice(1).every(item => item.skipped), true)
})

test('a failed catalog is reported separately from a successful empty search', async () => {
  const result = await createResourceSearch(async ({ url }) => {
    if (url.includes('file')) throw new Error('private-server-error')
    return page([])
  })({ kinds: ['file', 'ebook'], keyword: '数学' })
  assert.equal(result.modelResult.reports[0].failed, true)
  assert.equal(result.modelResult.reports[1].failed, undefined)
  assert.equal(result.modelResult.reports[1].catalogTotal, 0)
  assert.equal(JSON.stringify(result.modelResult).includes('private-server-error'), false)
})

test('invalid tool arguments and cancellation never trigger catalog calls', async () => {
  let calls = 0
  const search = createResourceSearch(async () => { calls++; return page([]) })
  await assert.rejects(search({ kinds: ['file'], keyword: '数学', userId: 42 }))
  await assert.rejects(search({ kinds: ['file'], keyword: '数学' }, { signal: AbortSignal.abort() }), /停止/)
  assert.equal(calls, 0)
})
