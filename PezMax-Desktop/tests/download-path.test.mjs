import assert from 'node:assert/strict'
import path from 'node:path'
import test from 'node:test'
import { downloadPartPaths, downloadURL, nextAvailableDownloadPath, safeDownloadName, strongETag } from '../src/main/main-utils/download-path.mjs'

test('download names stay inside the selected directory', () => {
  const base = path.join('downloads', safeDownloadName('../outside.pdf'))
  assert.equal(base, path.join('downloads', 'outside.pdf'))
  assert.equal(safeDownloadName('..\\outside.pdf'), 'outside.pdf')
  assert.throws(() => safeDownloadName('CON.txt'))
})

test('different objects do not share partial files', () => {
  const a = downloadPartPaths('report.pdf', 'https://example.com/file?id=1')
  const b = downloadPartPaths('report.pdf', 'https://example.com/file?id=2')
  assert.notEqual(a.partPath, b.partPath)
  assert.equal(a.metaPath, `${a.partPath}.json`)
  assert.equal(strongETag('"datum-1-100"'), '"datum-1-100"')
  assert.equal(strongETag('W/"datum-1-100"'), '')
})

test('silent downloads preserve existing files and reject unsafe URLs', () => {
  const original = path.join('downloads', 'report.pdf')
  assert.equal(nextAvailableDownloadPath(original, p => p === original), path.join('downloads', 'report (1).pdf'))
  assert.equal(downloadURL('https://example.com/file?id=1'), 'https://example.com/file?id=1')
  assert.throws(() => downloadURL('file:///etc/passwd'))
})
