import assert from 'node:assert/strict'
import test from 'node:test'
import { mountUpload } from './helpers/sfc-test-utils.mjs'

test('new subject confirmation renders the subject as text', async (t) => {
  const subject = '<img src=x onerror=alert(1)>&"'
  const page = await mountUpload({ subjects: [], subject })
  t.after(page.dispose)
  await page.bindings.submitUpload()
  assert.equal(page.confirms.length, 1)
  assert.ok(page.confirms[0][0].includes('&lt;img src=x onerror=alert(1)&gt;&amp;&quot;'))
  assert.ok(!page.confirms[0][0].includes(subject))
  // 业务提交仍使用原始学科名，转义只发生在显示层。
  assert.equal(page.store.uploadForm.fileSubject, subject)
})
