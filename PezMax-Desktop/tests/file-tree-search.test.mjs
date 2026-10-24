import test from 'node:test'
import assert from 'node:assert/strict'
import { searchFileTree } from '../src/renderer/utils/file-tree-search.mjs'

const math = { id: 'file-7', type: 'file', label: '2023高等数学期末.pdf', fileInfo: { fileId: 7 } }
const physics = { id: 'file-8', type: 'file', label: 'Physics 2024.pdf', fileInfo: { fileId: 8 } }
const tree = [
    {
        id: 'school',
        type: 'folder',
        label: '示例大学',
        children: [{ id: 'subject', type: 'folder', label: '历年试卷', children: [math, physics] }]
    }
]

test('filename matches retain the school and subject path and the original preview entity', () => {
    const before = JSON.stringify(tree)
    const matches = searchFileTree(tree, '高等数学')
    assert.equal(matches[0].label, '示例大学')
    assert.equal(matches[0].children[0].label, '历年试卷')
    assert.deepEqual(matches[0].children[0].children, [math])
    assert.equal(matches[0].children[0].children[0], math)
    assert.equal(JSON.stringify(tree), before)
})

test('folder matches keep all their files, without duplicate descendant results', () => {
    const matches = searchFileTree(tree, '历年试卷')
    assert.equal(matches.length, 1)
    assert.equal(matches[0].children[0], tree[0].children[0])
    assert.equal(matches[0].children[0].children.length, 2)
})

test('case, whitespace, clearing the input, and empty results behave predictably', () => {
    assert.equal(searchFileTree(tree, ' PHYSICS ')[0].children[0].children[0], physics)
    assert.equal(searchFileTree(tree, '  '), tree)
    assert.deepEqual(searchFileTree(tree, '找不到'), [])
    assert.deepEqual(searchFileTree(null, '数学'), [])
})
