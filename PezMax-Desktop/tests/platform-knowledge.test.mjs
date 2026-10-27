import assert from 'node:assert/strict'
import test from 'node:test'
import {
    normalizeKnowledgeInput,
    normalizeKnowledgeResult,
    platformKnowledgeNode
} from '../src/shared/knowledge-assistant.mjs'
import {
    createKnowledgeSession,
    createKnowledgeState
} from '../src/renderer/utils/knowledge-assistant-session.mjs'
import { createAssistantService } from '../src/main/main-utils/assistant-service.mjs'

const source = {
    kind: 'file',
    id: '9007199254740993',
    title: 'Synthetic notes',
    format: 'pdf',
    subject: 'Software',
    school: 'Campus',
    year: 2026,
    revision: 'a'.repeat(64)
}
const documentId = 'b'.repeat(32),
    chunkId = 'c'.repeat(32)
const coverage = { available: 1, total: 1, partial: false }
const doc = {
    documentId,
    title: source.title,
    format: 'pdf',
    characters: 40,
    chunks: 1,
    dimension: 0,
    created: '',
    source
}
const chunk = {
    documentId,
    chunkId,
    location: 2,
    title: source.title,
    format: 'pdf',
    text: 'A queue uses first in first out ordering.',
    source
}
const sync = { documents: [doc], coverage, sync: { indexed: 1, unchanged: 0, failures: [] } }

test('platform contracts retain provenance and reject arbitrary paths, actions and missing coverage', () => {
    const input = normalizeKnowledgeInput({
        action: 'sync',
        library: 'platform',
        url: 'https://outside.example',
        path: 'private',
        owner: 'spoof'
    })
    assert.deepEqual(input, { action: 'sync', library: 'platform', mode: 'keyword' })
    for (const action of ['import', 'delete'])
        assert.throws(() => normalizeKnowledgeInput({ action, library: 'platform' }))
    assert.throws(() => normalizeKnowledgeInput({ action: 'sync' }))
    assert.deepEqual(normalizeKnowledgeResult(sync, input).documents[0].source, source)
    assert.throws(() => normalizeKnowledgeResult({ ...sync, coverage: undefined }, input))
    assert.throws(() =>
        normalizeKnowledgeResult({ ...sync, documents: [{ ...doc, source: undefined }] }, input)
    )
})

test('citation links use fixed configured platform routes, string IDs and locations', () => {
    const node = platformKnowledgeNode(
        { ...source, url: 'https://outside.example' },
        'https://catalog.example/base',
        2
    )
    assert.equal(node.id, 'file-9007199254740993')
    assert.equal(node.fileInfo.fileId, source.id)
    assert.equal(new URL(node.url).origin, 'https://catalog.example')
    assert.equal(new URL(node.url).pathname, '/base/datum/knowledge/content')
    assert.equal(new URL(node.url).searchParams.get('revision'), source.revision)
    assert.equal(node.knowledgeLocation, 2)
    const ebook = platformKnowledgeNode(
        { ...source, kind: 'ebook', format: 'epub' },
        'https://catalog.example',
        3
    )
    assert.equal(ebook.originalData.ebookId, source.id)
    assert.throws(() => platformKnowledgeNode(source, 'file:///tmp', 1))
    assert.throws(() => platformKnowledgeNode(source, 'https://catalog.example', 0))
})

test('platform session automatically syncs, sends token only for platform and validates open citation', async () => {
    const calls = [],
        opened = []
    const state = createKnowledgeState()
    const session = createKnowledgeSession({
        state,
        library: 'platform',
        platformToken: () => 'private-platform-token',
        openResource: (value) => opened.push(value),
        call: async (method, _context, input) => {
            if (method === 'getConfig') return {}
            calls.push(input)
            return {
                knowledge:
                    input.knowledge.action === 'sync'
                        ? sync
                        : input.knowledge.action === 'open'
                          ? { chunk, coverage }
                          : { documents: [] }
            }
        }
    })
    await session.setIdentity({ server: 'https://catalog.example', userId: '7' })
    assert.equal(calls[0].knowledge.action, 'sync')
    assert.equal(calls[0].platformToken, 'private-platform-token')
    assert.equal(state.library, 'platform')
    await session.openEvidence(chunk)
    assert.equal(opened[0].location, 2)
    await session.switchLibrary('local')
    assert.equal(calls.at(-1).platformToken, undefined)
    assert.equal(state.documents.length, 0)
    session.dispose()
})

test('main process supplies platform origin and strips renderer storage overrides', async () => {
    const calls = []
    const service = createAssistantService({
        readConfig: () => ({}),
        writeConfig() {},
        encryption: { isAvailable: () => false },
        knowledgeDatabase: 'C:/fixture/index.sqlite3',
        catalogBaseUrl: 'https://catalog.example',
        runAgent: async (payload) => {
            calls.push(payload)
            return { knowledge: sync }
        }
    })
    await service.run(
        'a'.repeat(64),
        {
            requestId: 'platform-sync',
            task: 'knowledge',
            platformToken: 'private-platform-token',
            knowledge: {
                action: 'sync',
                library: 'platform',
                path: 'spoof',
                catalog: 'https://outside.example'
            }
        },
        1
    )
    assert.deepEqual(calls[0].catalog, {
        baseUrl: 'https://catalog.example',
        token: 'private-platform-token'
    })
    assert.equal(calls[0].storage.owner, 'a'.repeat(64))
    assert.equal(calls[0].provider, undefined)
    assert.equal(calls[0].embedding, undefined)
    await assert.rejects(
        service.run(
            'a'.repeat(64),
            {
                requestId: 'missing-auth',
                task: 'knowledge',
                knowledge: { action: 'sync', library: 'platform' }
            },
            1
        ),
        /登录/
    )
})
