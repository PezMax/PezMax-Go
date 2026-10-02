import assert from 'node:assert/strict'
import test from 'node:test'
import { mountUpload, plain } from './helpers/sfc-test-utils.mjs'

test('upload suggestions consume Go code=0, school strings and Subject/Total counts', async (t) => {
  const page = await mountUpload()
  t.after(page.dispose)
  let schools, subjects
  await page.bindings.querySearchSchool('示例', (rows) => { schools = rows })
  await page.bindings.querySearchSubject('数学', (rows) => { subjects = rows })
  assert.deepEqual(plain(schools), [{ value: '示例大学', count: 0 }])
  assert.deepEqual(plain(subjects), [{ value: '高等数学', count: 3 }])
  await page.bindings.submitUpload()
  assert.equal(page.confirms.length, 0, 'an existing subject must not be reported as newly created')
})

test('upload suggestions preserve legacy code=200 value/count responses', async (t) => {
  const page = await mountUpload({ code: 200, schools: [{ value: '示例大学', count: 4 }], subjects: [{ value: '高等数学', count: 3 }] })
  t.after(page.dispose)
  let schools, subjects
  await page.bindings.querySearchSchool('示例', (rows) => { schools = rows })
  await page.bindings.querySearchSubject('数学', (rows) => { subjects = rows })
  assert.deepEqual(plain(schools), [{ value: '示例大学', count: 4 }])
  assert.deepEqual(plain(subjects), [{ value: '高等数学', count: 3 }])
})

test('school duplicate check reads data.exists instead of object truthiness', async (t) => {
  for (const code of [0, 200]) for (const exists of [false, true]) {
    await t.test(`code=${code} exists=${exists}`, async (t) => {
      const page = await mountUpload({ code, exists })
      t.after(page.dispose)
      await page.bindings.handleSchoolBlur({ target: { value: '示例大学' } })
      assert.equal(page.confirms.length, exists ? 1 : 0)
    })
  }
})
