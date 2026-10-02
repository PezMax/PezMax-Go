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

function sidePanelPage(listEbook, listBookmark = async () => ({ code: 0, rows: [], total: 0 })) {
  return mountPage('views/home/components/SidePanel.vue', {
    './ReportTimelinePanel.vue': { default: {} }, './UploadPanel.vue': { default: {} },
    'element-plus': { ElMessage: emptyMessage },
    '@/utils/url': { normalizeFileUrl: value => value },
    '@/utils/pagination': { fetchAllPages },
    '@/api/datum/bookmark': { listBookmark, addBookmark() {}, updateBookmark() {} },
    '@/api/datum/ebook': { listEbook },
    '@/store/modules/user': { default: () => ({ id: 7 }) }, axios: { default: {} }
  }, { addEventListener() {}, removeEventListener() {} })
}

test('exactly one full page loads with a single request', async (t) => {
  const calls = []
  const listEbook = async (query) => {
    calls.push(query)
    return { code: 0, rows: Array.from({ length: 100 }, (_, i) => ({ ebookId: i + 1, ebookName: `电子书${i + 1}` })), total: 100 }
  }
  const page = await sidePanelPage(listEbook)
  t.after(page.dispose)
  await page.bindings.fetchEbooks()
  assert.equal(calls.length, 1, 'a full first page must not trigger a second request')
  assert.equal(page.bindings.ebookList.value.length, 100)
})

test('successful empty refresh clears previously loaded rows', async (t) => {
  let payload = { code: 0, rows: [{ ebookId: 1, ebookName: '电子书1' }, { ebookId: 2, ebookName: '电子书2' }, { ebookId: 3, ebookName: '电子书3' }], total: 3 }
  const listEbook = async () => payload
  const page = await sidePanelPage(listEbook)
  t.after(page.dispose)
  await page.bindings.fetchEbooks()
  assert.equal(page.bindings.ebookList.value.length, 3)
  payload = { code: 0, rows: [], total: 0 }
  await page.bindings.fetchEbooks()
  assert.equal(page.bindings.ebookList.value.length, 0, 'a successful empty refresh must clear the list')
})

test('second-page failure must not write a partial first page', async (t) => {
  const listEbook = async (query) => {
    if (query.pageNum === 1) return { code: 0, rows: Array.from({ length: 100 }, (_, i) => ({ ebookId: i + 1, ebookName: `电子书${i + 1}` })), total: 101 }
    throw new Error('second page failed')
  }
  const page = await sidePanelPage(listEbook)
  t.after(page.dispose)
  page.bindings.ebookList.value = [{ ebookId: 999, ebookName: '旧数据' }]
  await page.bindings.fetchEbooks()
  assert.equal(page.bindings.ebookList.value.length, 1, 'failed pagination must keep the previous list')
  assert.equal(page.bindings.ebookList.value[0].ebookId, 999, 'partial first page must not replace existing rows')
})

test('refresh then a different keyword still searches the fully loaded source', async (t) => {
  const paginate = async (query) => ({
    code: 0,
    rows: query.pageNum === 1 ? Array.from({ length: 100 }, (_, i) => ({ ebookId: i + 1, ebookName: `电子书${i + 1}` })) : [{ ebookId: 101, ebookName: '电子书101' }],
    total: 101
  })
  const page = await sidePanelPage(paginate)
  t.after(page.dispose)
  await page.bindings.fetchEbooks()
  page.bindings.ebookSearchQuery.value = '电子书101'
  assert.equal(page.bindings.ebookTreeData.value[0].ebookId, 101)
  page.bindings.ebookSearchQuery.value = '电子书5'
  const visible = JSON.stringify(page.bindings.ebookTreeData.value)
  assert.ok(visible.includes('电子书5'), 'switching keywords must keep searching the full source')
  assert.ok(!visible.includes('电子书101'), 'previous keyword result must not persist')
})
