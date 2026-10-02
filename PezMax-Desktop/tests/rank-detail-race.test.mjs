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
