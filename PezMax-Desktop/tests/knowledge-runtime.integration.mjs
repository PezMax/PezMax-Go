import assert from 'node:assert/strict'
import test from 'node:test'
import { once } from 'node:events'
import { createServer } from 'node:http'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import {
    createAgentProcess,
    resolveAgentRuntime
} from '../src/main/main-utils/assistant-process.mjs'
import { createAssistantService } from '../src/main/main-utils/assistant-service.mjs'

const appPath = fileURLToPath(new URL('../', import.meta.url))
const root = resolve(appPath, '../PezMax-Agent')
const owner = 'a'.repeat(64),
    other = 'b'.repeat(64)

for (const packaged of [false, true]) {
    test(
        `real ${packaged ? 'frozen' : 'source'} desktop knowledge import, hybrid RAG, citations, isolation and cancellation`,
        { timeout: 180000 },
        async () => {
            const directory = await mkdtemp(join(tmpdir(), 'pezmax-synthetic-knowledge-'))
            const runtime = resolveAgentRuntime({ isPackaged: false, appPath })
            const fixture = spawnSync(
                runtime.file,
                [join(root, 'tests/fixtures/knowledge_documents.py'), directory],
                { windowsHide: true, encoding: 'utf-8' }
            )
            assert.equal(fixture.status, 0, fixture.stderr)
            const calls = [],
                payloads = []
            let selected,
                rejectNext = false,
                hanging = false,
                entered,
                requestNumber = 0
            const server = createServer(async (request, response) => {
                let text = ''
                for await (const chunk of request) text += chunk
                const body = JSON.parse(text)
                calls.push({ url: request.url, auth: request.headers.authorization, body })
                response.setHeader('Content-Type', 'application/json')
                if (hanging) {
                    entered()
                    return
                }
                if (request.url.endsWith('/embeddings')) {
                    response.end(
                        JSON.stringify({
                            data: body.input.map((_value, index) => ({ index, embedding: [1, 0] }))
                        })
                    )
                    return
                }
                const evidence = JSON.parse(body.messages[1].content).excerpts[0]
                const citation = {
                    documentId: evidence.documentId,
                    chunkId: evidence.chunkId,
                    location: rejectNext ? 999 : evidence.location,
                    quote: 'first in first out'
                }
                rejectNext = false
                response.end(
                    JSON.stringify({
                        choices: [
                            {
                                finish_reason: 'stop',
                                message: {
                                    role: 'assistant',
                                    content: JSON.stringify({
                                        answers: [
                                            { text: 'A queue uses FIFO.', citations: [citation] }
                                        ]
                                    })
                                }
                            }
                        ]
                    })
                )
            })
            server.listen(0, '127.0.0.1')
            await once(server, 'listening')
            const baseUrl = `http://127.0.0.1:${server.address().port}/v1`
            const worker = createAgentProcess({
                resolveRuntime: () =>
                    packaged
                        ? {
                              file: join(
                                  root,
                                  'dist/pezmax-agent',
                                  process.platform === 'win32' ? 'pezmax-agent.exe' : 'pezmax-agent'
                              ),
                              args: [],
                              cwd: root
                          }
                        : runtime
            })
            const service = createAssistantService({
                encryption: { isAvailable: () => false },
                readConfig: () => ({}),
                writeConfig() {},
                knowledgeDatabase: join(directory, 'documents.sqlite3'),
                selectKnowledgeFile: async () => selected,
                runAgent: async (payload, callbacks) => {
                    payloads.push(payload)
                    return worker(payload, callbacks)
                }
            })
            const run = (
                knowledge,
                scope = owner,
                requestId = `kb-integration-${++requestNumber}`
            ) => service.run(scope, { requestId, task: 'knowledge', knowledge }, 1)
            try {
                const docs = []
                for (const format of ['pdf', 'epub', 'md']) {
                    selected = join(directory, `synthetic-queue.${format}`)
                    const imported = (await run({ action: 'import' })).knowledge.document
                    assert.equal(imported.inserted, true)
                    docs.push(imported)
                    const request = {
                        action: 'search',
                        query: { question: 'queue', documentIds: [imported.documentId] }
                    }
                    const chunk = (await run(request)).knowledge.retrieval.chunks[0]
                    assert.equal(chunk.location, format === 'md' ? 1 : 2)
                    const read = (
                        await run({
                            action: 'read',
                            documentId: chunk.documentId,
                            chunkId: chunk.chunkId
                        })
                    ).knowledge.chunk
                    assert.equal(read.text, chunk.text)
                }
                assert.equal(
                    calls.length,
                    0,
                    'keyword import/search/read must make no model requests'
                )
                assert.equal((await run({ action: 'list' })).knowledge.documents.length, 3)
                assert.equal((await run({ action: 'list' }, other)).knowledge.documents.length, 0)
                await assert.rejects(
                    run(
                        { action: 'read', documentId: docs[0].documentId, chunkId: 'c'.repeat(32) },
                        other
                    )
                )
                assert.equal((await run({ action: 'import' })).knowledge.document.inserted, false)
                service.save(owner, {
                    baseUrl,
                    model: 'synthetic-chat',
                    apiKey: 'fixture-generation-key'
                })
                rejectNext = true
                const asked = (await run({ action: 'ask', query: { question: 'queue' } })).knowledge
                assert.equal(asked.answers.length, 1)
                assert.equal(
                    calls.filter((call) => call.url.endsWith('/chat/completions')).length,
                    2,
                    'one invalid citation triggers one repair'
                )
                assert.equal(payloads.at(-1).embedding, undefined)
                const citation = asked.answers[0].citations[0]
                assert.equal(
                    asked.retrieval.chunks.some(
                        (c) =>
                            c.chunkId === citation.chunkId &&
                            c.location === citation.location &&
                            c.text.includes(citation.quote)
                    ),
                    true
                )
                assert.equal(
                    calls.some((c) => JSON.stringify(c.body).includes(directory)),
                    false
                )
                assert.equal(JSON.stringify(asked).includes('fixture-generation-key'), false)
                service.saveEmbedding(owner, {
                    baseUrl,
                    model: 'synthetic-vector',
                    apiKey: 'fixture-embedding-key'
                })
                assert.equal(
                    (await run({ action: 'delete', documentId: docs[2].documentId })).knowledge
                        .deleted,
                    true
                )
                selected = join(directory, 'synthetic-queue.md')
                assert.equal(
                    (await run({ action: 'import', mode: 'hybrid' })).knowledge.document.dimension,
                    2
                )
                const mixed = (
                    await run({
                        action: 'ask',
                        mode: 'hybrid',
                        query: { question: 'queue', documentIds: [docs[2].documentId] }
                    })
                ).knowledge
                assert.equal(mixed.retrieval.fusion, 'rrf-k60')
                assert.equal(mixed.retrieval.vectorChunks, 1)
                const embeddings = calls.filter((call) => call.url.endsWith('/embeddings'))
                assert.equal(embeddings.length, 2)
                assert.ok(embeddings.every((call) => call.auth === 'Bearer fixture-embedding-key'))
                assert.ok(
                    calls
                        .filter((call) => call.url.endsWith('/chat/completions'))
                        .every((call) => call.auth === 'Bearer fixture-generation-key')
                )
                const before = calls.length
                const empty = (await run({ action: 'ask', query: { question: 'zzzznotpresent' } }))
                    .knowledge
                assert.equal(empty.answers.length, 0)
                assert.equal(calls.length, before, 'empty lexical retrieval skips generation')
                hanging = true
                const started = new Promise((resolveEntered) => {
                    entered = resolveEntered
                })
                const pending = run(
                    { action: 'ask', query: { question: 'queue' } },
                    owner,
                    'cancel-kb'
                )
                const stopped = assert.rejects(pending, /已停止/)
                await started
                service.cancel('cancel-kb', 1)
                await stopped
                hanging = false
                for (const doc of docs)
                    assert.equal(
                        (await run({ action: 'delete', documentId: doc.documentId })).knowledge
                            .deleted,
                        true
                    )
                assert.equal((await run({ action: 'list' })).knowledge.documents.length, 0)
            } finally {
                service.cancelOwner(1)
                server.closeAllConnections()
                await new Promise((resolveClosed) => server.close(resolveClosed))
                await rm(directory, { recursive: true, force: true })
            }
        }
    )
}
