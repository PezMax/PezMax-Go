import assert from 'node:assert/strict'
import test from 'node:test'
import { emptyMessage, mountPage } from './helpers/sfc-test-utils.mjs'
import { fetchAllPages } from '../src/renderer/utils/pagination.js'

test('ebook and bookmark explorer search includes rows after the first 100', async (t) => {
  const calls = { ebook: [], bookmark: [] }
  const paginate = (kind, makeRow) => async (query) => {
    calls[kind].push(query)
    return { code: 0, rows: query.pageNum === 1 ? Array.from({ length: 100 }, (_, i) => makeRow(i + 1)) : [makeRow(101)], total: 101 }
  }
  const page = await mountPage('views/home/components/SidePanel.vue', {
    './ReportTimelinePanel.vue': { default: {} }, './UploadPanel.vue': { default: {} },
    'element-plus': { ElMessage: emptyMessage },
    '@/utils/url': { normalizeFileUrl: value => value },
    '@/utils/pagination': { fetchAllPages },
    '@/api/datum/bookmark': { listBookmark: paginate('bookmark', id => ({ id, title: `书签${id}`, url: `https://example.test/${id}` })), addBookmark() {}, updateBookmark() {} },
    '@/api/datum/ebook': { listEbook: paginate('ebook', ebookId => ({ ebookId, ebookName: `电子书${ebookId}` })) },
    '@/store/modules/user': { default: () => ({ id: 7 }) }, axios: { default: {} }
  }, { addEventListener() {}, removeEventListener() {} })
  t.after(page.dispose)
  await page.bindings.fetchEbooks()
  await page.bindings.fetchBookmarks()
  assert.equal(page.bindings.ebookList.value.length, 101)
  assert.equal(page.bindings.bookmarkList.value.length, 101)
  assert.deepEqual(calls.ebook.map(q => q.pageNum), [1, 2])
  assert.deepEqual(calls.bookmark.map(q => q.pageNum), [1, 2])
  page.bindings.ebookSearchQuery.value = '电子书101'
  page.bindings.bookmarkSearchQuery.value = '书签101'
  assert.equal(page.bindings.ebookTreeData.value[0].ebookId, 101)
  assert.ok(JSON.stringify(page.bindings.bookmarkTreeData.value).includes('书签101'))
  // 刷新后换关键词：基础加载不带关键词，完整来源始终可检索。
  assert.equal(calls.ebook[0].title, undefined)
  assert.equal(calls.bookmark[0].title, undefined)
})

test('side panel keeps failure and empty handling intact', async (t) => {
  let calls = 0
  const page = await mountPage('views/home/components/SidePanel.vue', {
    './ReportTimelinePanel.vue': { default: {} }, './UploadPanel.vue': { default: {} },
    'element-plus': { ElMessage: emptyMessage },
    '@/utils/url': { normalizeFileUrl: value => value },
    '@/utils/pagination': { fetchAllPages: async (fetcher) => { calls++; throw new Error('network down') } },
    '@/api/datum/bookmark': { listBookmark: async () => ({ code: 0, rows: [], total: 0 }), addBookmark() {}, updateBookmark() {} },
    '@/api/datum/ebook': { listEbook: async () => ({ code: 0, rows: [], total: 0 }) },
    '@/store/modules/user': { default: () => ({ id: 7 }) }, axios: { default: {} }
  }, { addEventListener() {}, removeEventListener() {} })
  t.after(page.dispose)
  const before = page.bindings.ebookList.value
  await page.bindings.fetchEbooks()
  await page.bindings.fetchBookmarks()
  assert.equal(calls, 2, 'fetchAllPages failure must propagate without fallback list')
  assert.equal(page.bindings.ebookList.value, before)
})
