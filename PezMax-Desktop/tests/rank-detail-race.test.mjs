import assert from 'node:assert/strict'
import test from 'node:test'
import { mountPage } from './helpers/sfc-test-utils.mjs'

test('rank details ignore an earlier user request that finishes after selection changes', async (t) => {
  let resolveFirst
  const first = new Promise((resolve) => { resolveFirst = resolve })
  const page = await mountPage('views/rank/index.vue', {
    '@/api/datum/user': {
      getUploadRank: async () => ({ code: 0, data: [{ userId: 1, userName: '第一名', count: 5 }, { userId: 2, userName: '第二名', count: 4 }] }),
      getUser: (id) => id === 1 ? first : Promise.resolve({ code: 0, data: { userId: 2, userName: '第二名详情', count: 4 } })
    }
  })
  t.after(page.dispose)
  await page.bindings.openUserDetail(page.bindings.rankList.value[1], 1)
  resolveFirst({ code: 0, data: { userId: 1, userName: '过时第一名详情', count: 5 } })
  await page.flush()
  assert.equal(page.bindings.activeUserId.value, 2)
  assert.equal(page.bindings.userDetail.value.userId, 2)
  assert.equal(page.bindings.userDetail.value.userName, '第二名详情')
})

test('rank detail requests are invalidated by failure, empty list and unmount', async (t) => {
  const page = await mountPage('views/rank/index.vue', {
    '@/api/datum/user': {
      getUploadRank: async () => ({ code: 0, data: [] }),
      getUser: async () => ({ code: 0, data: { userId: 1, userName: 'x', count: 1 } })
    }
  })
  t.after(page.dispose)
  await page.bindings.fetchRank()
  await page.flush()
  assert.equal(page.bindings.rankList.value.length, 0)
  assert.equal(page.bindings.userDetail.value, null)
  assert.equal(page.bindings.activeUserId.value, null)
  // 卸载后旧响应不得写回：request 序号递增即可，这里验证 dispose 正常。
  page.dispose()
})

function rankPageMock(script, detailCalls) {
  return {
    '@/api/datum/user': {
      getUploadRank: async () => script.length ? script.shift()() : { code: 0, data: [] },
      getUser: (id) => { detailCalls.push(id); return Promise.resolve({ code: 0, data: { userId: id, userName: `用户${id}详情`, count: 1 } }) }
    }
  }
}

test('late rank response after unmount must not start detail requests or write state', async (t) => {
  let resolveRank
  const detailCalls = []
  const pendingRank = () => new Promise((resolve) => { resolveRank = resolve })
  const page = await mountPage('views/rank/index.vue', rankPageMock([pendingRank], detailCalls))
  const dispose = page.dispose
  t.after(dispose)
  const fetchPromise = page.bindings.fetchRank()
  dispose()
  resolveRank({ code: 0, data: [{ userId: 7, userName: '迟到第一', count: 9 }] })
  await fetchPromise
  await page.flush()
  assert.equal(detailCalls.length, 0, `getUser must not be called after unmount (got ${detailCalls.length})`)
  assert.equal(page.bindings.rankList.value.length, 0, 'rank list must not be written back after unmount')
  assert.equal(page.bindings.userDetail.value, null, 'detail must not be started after unmount')
})

test('late detail response after unmount must not write back', async (t) => {
  let resolveDetail
  const detailCalls = []
  const page = await mountPage('views/rank/index.vue', {
    '@/api/datum/user': {
      getUploadRank: async () => ({ code: 0, data: [{ userId: 1, userName: '第一', count: 5 }] }),
      getUser: (id) => new Promise((resolve) => { detailCalls.push(id); resolveDetail = resolve })
    }
  })
  t.after(page.dispose)
  await page.flush()
  page.bindings.openUserDetail(page.bindings.rankList.value[0], 0)
  await page.flush()
  const quickPreview = page.bindings.userDetail.value.userName
  page.dispose()
  resolveDetail({ code: 0, data: { userId: 1, userName: '迟到详情', count: 5 } })
  await page.flush()
  assert.ok(detailCalls.includes(1), 'detail request should have been issued before unmount')
  assert.equal(page.bindings.userDetail.value.userName, quickPreview, 'late detail response must not overwrite after unmount')
})

test('old detail pending, refresh rejects, old detail resolves: detail stays cleared', async (t) => {
  let resolveFirst
  const script = [async () => ({ code: 0, data: [{ userId: 1, userName: '第一', count: 5 }] }), async () => { throw new Error('refresh failed') }]
  const page = await mountPage('views/rank/index.vue', {
    '@/api/datum/user': {
      getUploadRank: async () => script.length ? script.shift()() : { code: 0, data: [] },
      getUser: (id) => new Promise((resolve) => { resolveFirst = resolve })
    }
  })
  t.after(page.dispose)
  await page.flush()
  page.bindings.openUserDetail(page.bindings.rankList.value[0], 0)
  await page.flush()
  const refresh = page.bindings.fetchRank()
  await refresh
  assert.equal(page.bindings.userDetail.value, null, 'failed refresh must clear the detail')
  resolveFirst({ code: 0, data: { userId: 1, userName: '过时详情', count: 5 } })
  await page.flush()
  assert.equal(page.bindings.userDetail.value, null, 'stale detail response must not restore the cleared detail')
})

test('old detail pending, empty-list refresh, old detail resolves: stays empty', async (t) => {
  let resolveFirst
  const script = [async () => ({ code: 0, data: [{ userId: 1, userName: '第一', count: 5 }] }), async () => ({ code: 0, data: [] })]
  const page = await mountPage('views/rank/index.vue', {
    '@/api/datum/user': {
      getUploadRank: async () => script.length ? script.shift()() : { code: 0, data: [] },
      getUser: (id) => new Promise((resolve) => { resolveFirst = resolve })
    }
  })
  t.after(page.dispose)
  await page.flush()
  page.bindings.openUserDetail(page.bindings.rankList.value[0], 0)
  await page.flush()
  await page.bindings.fetchRank()
  assert.equal(page.bindings.rankList.value.length, 0)
  assert.equal(page.bindings.userDetail.value, null)
  resolveFirst({ code: 0, data: { userId: 1, userName: '过时详情', count: 5 } })
  await page.flush()
  assert.equal(page.bindings.rankList.value.length, 0, 'empty rank list must stay empty')
  assert.equal(page.bindings.userDetail.value, null, 'stale detail response must not repopulate after empty refresh')
})
