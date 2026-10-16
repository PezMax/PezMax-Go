import assert from 'node:assert/strict'
import test from 'node:test'

import { clearApplicationCaches } from '../src/renderer/utils/appCache.mjs'
import { sortFileTree } from '../src/renderer/utils/fileTreeSort.mjs'

test('cache cleanup removes transient data but preserves preferences and notification receipts', () => {
  const local = new Map([
    ['ptmj_file_tree_cache', 'tree'],
    ['ptmj_upload_rank_cache', 'rank'],
    ['layout-setting', 'dark'],
    ['processedNotifications', '[1]'],
    ['token', 'session'],
  ])
  const session = new Map([['sessionObj', 'request'], ['pendingOpenFile', 'file']])
  const storage = (values) => ({ removeItem: (key) => values.delete(key) })
  clearApplicationCaches(storage(local), storage(session))
  assert.deepEqual([...local], [
    ['layout-setting', 'dark'],
    ['processedNotifications', '[1]'],
    ['token', 'session'],
  ])
  assert.deepEqual([...session], [['pendingOpenFile', 'file']])
})

test('tree sorting is recursive, folder-first, and numeric-aware', () => {
  const source = [
    { type: 'file', label: 'A2.pdf' },
    { type: 'folder', label: 'B10', children: [] },
    { type: 'folder', label: 'B2', children: [
      { type: 'file', label: '10.pdf' },
      { type: 'file', label: '2.pdf' },
    ] },
    { type: 'folder', label: 'A1', children: [] },
  ]
  const sorted = sortFileTree(source)
  assert.deepEqual(sorted.map((node) => node.label), ['A1', 'B2', 'B10', 'A2.pdf'])
  assert.deepEqual(sorted[1].children.map((node) => node.label), ['2.pdf', '10.pdf'])
  assert.equal(source[0].label, 'A2.pdf')
})
