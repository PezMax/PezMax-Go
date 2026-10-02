import assert from 'node:assert/strict'
import test from 'node:test'
import { emptyMessage, mountPage } from './helpers/sfc-test-utils.mjs'

test('download history restores missing ebooks through the ebook endpoint and retains negative local IDs', async (t) => {
  for (const fileId of [-42, 42]) await t.test(`fileId=${fileId}`, async (t) => {
    const downloads = [], records = []
    const page = await mountPage('views/datum/download/index.vue', {
      'element-plus': { ElMessage: emptyMessage, ElMessageBox: {} },
      '@/store/modules/user': { default: () => ({ id: 7 }) }
    }, { electronAPI: {
      getSettings: async () => ({}),
      openPath: async () => '',
      downloadFileDirectly: async (data) => { downloads.push(data); return { success: true, filePath: 'C:/restored/file.pdf' } },
      downloadRecords: { list: async () => ({ success: true, rows: [] }), add: async (record) => { records.push(record) }, flush: async () => {} }
    } })
    t.after(page.dispose)
    await page.bindings.openFile({ fileId, fileName: 'file.pdf' })
    assert.equal(downloads.length, 1)
    assert.equal(downloads[0].url, fileId < 0 ? '/dev-api/datum/download/ebook?ebookId=42' : '/dev-api/datum/download/file?fileId=42')
    assert.equal(records[0].fileId, fileId)
  })
})
