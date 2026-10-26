import assert from 'node:assert/strict'
import test from 'node:test'
import { createAssistantService } from '../src/main/main-utils/assistant-service.mjs'
import {
    createKnowledgeSession,
    createKnowledgeState
} from '../src/renderer/utils/knowledge-assistant-session.mjs'
import {
    normalizeKnowledgeInput,
    normalizeKnowledgeResult
} from '../src/shared/knowledge-assistant.mjs'

const owner = 'a'.repeat(64),
    documentId = 'b'.repeat(32),
    chunkId = 'c'.repeat(32)
const config = {
    baseUrl: 'https://model.example/v1',
    model: 'fixture',
    apiKey: 'generation-secret',
    remember: false
}
const chunk = {
    documentId,
    chunkId,
    location: 2,
    title: 'Synthetic queue notes',
    format: 'pdf',
    text: 'A queue uses first in first out ordering.'
}
const doc = {
    documentId,
    title: chunk.title,
    format: chunk.format,
    characters: 40,
    chunks: 1,
    dimension: 0,
    created: '2026-10-26T00:00:00Z'
}
const retrieval = {
    chunks: [chunk],
    mode: 'keyword',
    scannedChunks: 1,
    vectorChunks: 0,
    partial: false,
    fusion: 'bm25'
}
const answer = {
    text: 'A queue uses FIFO.',
    citations: [{ documentId, chunkId, location: 2, quote: 'first in first out' }]
}
const deferred = () => {
    let resolve
    const promise = new Promise((r) => {
        resolve = r
    })
    return { promise, resolve }
}
const envelope = (knowledge) => ({
    content: '',
    messages: [],
    resources: [],
    reports: [],
    knowledge
})

function fixture(options = {}) {
    const calls = [],
        picked = [],
        records = {}
    const service = createAssistantService({
        readConfig: () => records,
        writeConfig: (next) => {
            Object.assign(records, next)
        },
        encryption: {
            isAvailable: () => true,
            encrypt: (value) => `encrypted:${value}`,
            decrypt: (value) => value.slice(10)
        },
        knowledgeDatabase: 'C:\\synthetic-fixture\\knowledge.sqlite3',
        selectKnowledgeFile: async () => {
            picked.push(true)
            return 'C:\\synthetic-fixture\\notes.pdf'
        },
        runAgent: async (payload, callbacks) => {
            calls.push({ payload, callbacks })
            return envelope({ documents: [doc] })
        },
        ...options
    })
    const run = (knowledge) =>
        service.run(
            owner,
            {
                requestId: 'knowledge-test',
                task: 'knowledge',
                knowledge,
                storage: { owner: 'not-allowed', path: 'not-allowed' },
                provider: { apiKey: 'not-allowed' },
                localDocument: 'not-allowed'
            },
            1
        )
    return { calls, picked, records, service, run }
}

test('keyword operations need no model keys and paths/owner are fixed by main, not renderer', async () => {
    const f = fixture()
    await f.run({ action: 'list', storage: { owner: 'not-allowed' } })
    assert.equal(f.calls[0].payload.storage.owner, owner)
    assert.equal(f.calls[0].payload.storage.path, 'C:\\synthetic-fixture\\knowledge.sqlite3')
    assert.equal(f.calls[0].payload.provider, undefined)
    assert.equal(f.calls[0].payload.localDocument, undefined)
    assert.equal(JSON.stringify(f.calls).includes('not-allowed'), false)
    assert.equal(f.picked.length, 0)
})

test('generation and embeddings use separate encrypted BYOK profiles; paid modes must be explicit', async () => {
    const calls = []
    const f = fixture({
        runAgent: async (payload) => {
            calls.push(payload)
            return envelope(
                payload.knowledge.action === 'ask'
                    ? { content: 'A queue uses FIFO.', answers: [answer], retrieval }
                    : {
                          retrieval: {
                              ...retrieval,
                              mode: payload.knowledge.mode,
                              fusion: payload.knowledge.mode === 'hybrid' ? 'rrf-k60' : 'bm25'
                          }
                      }
            )
        }
    })
    const request = { action: 'ask', query: { question: 'queue' } }
    await assert.rejects(f.run(request), /API Key/)
    f.service.save(owner, { ...config, remember: true })
    await assert.rejects(
        f.run({ action: 'search', mode: 'hybrid', query: { question: 'queue' } }),
        /Embedding/
    )
    const publicConfig = f.service.saveEmbedding(owner, {
        ...config,
        model: 'vectors',
        apiKey: 'embedding-secret',
        remember: true
    })
    assert.equal(publicConfig.embedding.hasApiKey, true)
    assert.equal(JSON.stringify(publicConfig).includes('secret'), false)
    assert.equal(f.records[`${owner}_embedding`].encryptedKey, 'encrypted:embedding-secret')
    await f.run(request)
    assert.equal(calls[0].provider.apiKey, 'generation-secret')
    assert.equal(calls[0].embedding, undefined)
    await f.run({ action: 'search', mode: 'hybrid', query: { question: 'queue' } })
    assert.equal(calls[1].provider, undefined)
    assert.equal(calls[1].embedding.apiKey, 'embedding-secret')
    f.service.clearEmbedding(owner)
    assert.equal(f.service.publicConfig(owner).hasApiKey, true)
    assert.equal(f.service.publicConfig(owner).embedding.hasApiKey, false)
    f.service.clear(owner)
    f.service.forget(owner)
    assert.equal(f.service.publicConfig(owner).hasApiKey, false)
})

test('import opens only the trusted picker, cancelled picker launches no worker, and keyword import sends no keys', async () => {
    const f = fixture({
        runAgent: async (payload) => {
            f.calls.push(payload)
            return envelope({ document: { ...doc, inserted: true } })
        }
    })
    f.service.save(owner, config)
    await f.run({ action: 'import', localDocument: 'not-allowed' })
    assert.equal(f.picked.length, 1)
    assert.equal(f.calls[0].localDocument, 'C:\\synthetic-fixture\\notes.pdf')
    assert.equal(f.calls[0].provider, undefined)
    const cancelled = fixture({ selectKnowledgeFile: async () => null })
    assert.deepEqual(await cancelled.run({ action: 'import' }), { knowledge: { cancelled: true } })
    assert.equal(cancelled.calls.length, 0)
})

test('account release cancels picker/worker and prevents late result delivery; safe errors scrub both keys', async () => {
    const gate = deferred()
    const f = fixture({ selectKnowledgeFile: () => gate.promise })
    const pending = f.run({ action: 'import' })
    f.service.forget(owner)
    gate.resolve('C:\\synthetic-fixture\\notes.pdf')
    await assert.rejects(pending, /已停止/)
    assert.equal(f.calls.length, 0)
    const g = fixture({
        runAgent: async () => {
            throw new Error('向量 generation-secret embedding-secret')
        }
    })
    g.service.save(owner, config)
    g.service.saveEmbedding(owner, { ...config, apiKey: 'embedding-secret' })
    await assert.rejects(g.run({ action: 'list' }), (error) => !error.message.includes('secret'))
})

test('results reject foreign chunks, wrong positions and discontinuous quotations', () => {
    const query = normalizeKnowledgeInput({
        action: 'ask',
        query: { question: 'queue', documentIds: [documentId] }
    })
    const value = { content: 'FIFO', answers: [answer], retrieval }
    assert.equal(normalizeKnowledgeResult(value, query).answers.length, 1)
    for (const change of [
        { location: 1 },
        { documentId: 'd'.repeat(32) },
        { quote: 'first ordering' }
    ]) {
        assert.throws(() =>
            normalizeKnowledgeResult(
                {
                    ...value,
                    answers: [{ ...answer, citations: [{ ...answer.citations[0], ...change }] }]
                },
                query
            )
        )
    }
    assert.throws(() =>
        normalizeKnowledgeResult(
            {
                ...value,
                retrieval: { ...retrieval, chunks: [{ ...chunk, documentId: 'd'.repeat(32) }] }
            },
            query
        )
    )
    assert.throws(() =>
        normalizeKnowledgeInput({ action: 'search', query: { question: ' ', documentIds: [] } })
    )
})

test('renderer session never imports or generates automatically and clears source/results on account switch', async () => {
    const calls = [],
        state = createKnowledgeState()
    const session = createKnowledgeSession({
        state,
        call: async (method, context, input) => {
            calls.push({ method, context, input })
            if (method === 'getConfig') return { hasApiKey: true, embedding: { hasApiKey: false } }
            if (input?.knowledge.action === 'list') return envelope({ documents: [doc] })
            if (input?.knowledge.action === 'search') return envelope({ retrieval })
            if (input?.knowledge.action === 'read') return envelope({ chunk })
        }
    })
    await session.setIdentity({ server: 'fixture', userId: '1', token: 'must-not-send' })
    assert.deepEqual(
        calls.filter((c) => c.method === 'run').map((c) => c.input.knowledge.action),
        ['list']
    )
    await session.query('queue')
    assert.equal(state.result.action, 'search')
    await session.readEvidence(answer.citations[0])
    assert.equal(state.evidence.location, 2)
    await session.setIdentity(null)
    assert.equal(state.result, null)
    assert.equal(state.evidence, null)
    assert.equal(state.documents.length, 0)
    assert.equal(JSON.stringify(calls).includes('must-not-send'), false)
    session.dispose()
})

test('renderer ignores late results after account change and drains cancellation before the next request', async () => {
    const gate = deferred(),
        calls = [],
        state = createKnowledgeState()
    const session = createKnowledgeSession({
        state,
        call: async (method, context, input) => {
            calls.push({ method, context, input })
            if (method === 'getConfig') return { hasApiKey: false, embedding: { hasApiKey: false } }
            if (method === 'run' && input.knowledge.action === 'search') return gate.promise
            return envelope({ documents: context?.userId === '1' ? [doc] : [] })
        }
    })
    await session.setIdentity({ server: 'fixture', userId: '1' })
    const pending = session.query('queue')
    const switched = session.setIdentity({ server: 'fixture', userId: '2' })
    gate.resolve(envelope({ retrieval }))
    await Promise.all([pending, switched])
    assert.equal(state.result, null)
    assert.equal(state.documents.length, 0)
    assert.equal(state.busy, false)
    assert.equal(calls.filter((c) => c.method === 'cancel').length, 1)
    session.dispose()
})
