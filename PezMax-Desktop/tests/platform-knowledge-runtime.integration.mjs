import assert from 'node:assert/strict'
import test from 'node:test'
import { once } from 'node:events'
import { createServer } from 'node:http'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
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
for (const packaged of [false, true]) {
    test(
        `real ${packaged ? 'frozen' : 'source'} platform file sync, version refresh, cited RAG and withdrawal`,
        { timeout: 180000 },
        async () => {
            const directory = await mkdtemp(join(tmpdir(), 'pezmax-platform-synthetic-'))
            const sourceRuntime = resolveAgentRuntime({ isPackaged: false, appPath })
            const fixture = spawnSync(
                sourceRuntime.file,
                [join(root, 'tests/fixtures/knowledge_documents.py'), directory],
                { windowsHide: true, encoding: 'utf-8' }
            )
            assert.equal(fixture.status, 0, fixture.stderr)
            const pdf = await readFile(join(directory, 'synthetic-queue.pdf'))
            let sources = [
                {
                    kind: 'file',
                    id: '9007199254740993',
                    title: 'Synthetic queue PDF',
                    format: 'pdf',
                    subject: 'Software',
                    school: 'Synthetic campus',
                    year: 2026,
                    revision: 'a'.repeat(64)
                }
            ]
            let sequence = 0,
                downloads = 0,
                modelCalls = 0,
                embeddingCalls = 0,
                withdrawDuringGeneration = false
            const server = createServer(async (request, response) => {
                const url = new URL(request.url, 'http://fixture')
                response.setHeader('Content-Type', 'application/json')
                if (url.pathname.startsWith('/datum/knowledge/')) {
                    assert.equal(request.headers.authorization, 'Bearer synthetic-platform-token')
                    if (url.pathname.endsWith('/manifest'))
                        return response.end(
                            JSON.stringify({
                                code: 0,
                                data: { sources, total: sources.length, partial: false }
                            })
                        )
                    if (!sources.length) {
                        response.writeHead(404)
                        return response.end('{}')
                    }
                    if (url.searchParams.get('revision') !== sources[0].revision) {
                        response.writeHead(409)
                        return response.end('{}')
                    }
                    if (url.pathname.endsWith('/source'))
                        return response.end(JSON.stringify({ code: 0, data: sources[0] }))
                    downloads++
                    response.setHeader('Content-Type', 'application/pdf')
                    return response.end(pdf)
                }
                let raw = ''
                for await (const piece of request) raw += piece
                const body = JSON.parse(raw)
                assert.ok(!raw.includes('synthetic-platform-token'))
                assert.equal(
                    request.headers.authorization,
                    url.pathname.endsWith('/embeddings')
                        ? 'Bearer synthetic-embedding-key'
                        : 'Bearer synthetic-chat-key'
                )
                if (url.pathname.endsWith('/embeddings')) {
                    embeddingCalls++
                    return response.end(
                        JSON.stringify({
                            data: body.input.map((_, index) => ({ index, embedding: [1, 0] }))
                        })
                    )
                }
                modelCalls++
                const evidence = JSON.parse(body.messages[1].content).excerpts[0]
                if (withdrawDuringGeneration) sources = []
                return response.end(
                    JSON.stringify({
                        choices: [
                            {
                                finish_reason: 'stop',
                                message: {
                                    role: 'assistant',
                                    content: JSON.stringify({
                                        answers: [
                                            {
                                                text: 'Queues use FIFO.',
                                                citations: [
                                                    {
                                                        documentId: evidence.documentId,
                                                        chunkId: evidence.chunkId,
                                                        location: evidence.location,
                                                        quote: 'first in first out'
                                                    }
                                                ]
                                            }
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
            const baseUrl = `http://127.0.0.1:${server.address().port}`
            const worker = createAgentProcess({
                resolveRuntime: () =>
                    packaged
                        ? {
                              file: join(root, 'dist/pezmax-agent/pezmax-agent.exe'),
                              args: [],
                              cwd: root
                          }
                        : sourceRuntime
            })
            const service = createAssistantService({
                readConfig: () => ({}),
                writeConfig() {},
                encryption: { isAvailable: () => false },
                knowledgeDatabase: join(directory, 'index.sqlite3'),
                catalogBaseUrl: baseUrl,
                runAgent: worker
            })
            const owner = 'a'.repeat(64)
            const run = (knowledge, scope = owner) =>
                service.run(
                    scope,
                    {
                        requestId: `platform-integration-${++sequence}`,
                        task: 'knowledge',
                        platformToken: 'synthetic-platform-token',
                        knowledge: { ...knowledge, library: 'platform' }
                    },
                    1
                )
            try {
                const initial = (await run({ action: 'sync' })).knowledge
                assert.equal(initial.sync.indexed, 1)
                const id = initial.documents[0].documentId
                assert.equal(initial.documents[0].source.id, sources[0].id)
                assert.equal((await run({ action: 'sync' })).knowledge.sync.unchanged, 1)
                assert.equal(downloads, 1)
                assert.equal(modelCalls, 0)
                const retrieved = (await run({ action: 'search', query: { question: 'queue' } }))
                    .knowledge.retrieval.chunks[0]
                assert.equal(retrieved.location, 2)
                const opened = (
                    await run({ action: 'open', documentId: id, chunkId: retrieved.chunkId })
                ).knowledge.chunk
                assert.equal(opened.source.revision, sources[0].revision)
                service.save(owner, {
                    baseUrl: `${baseUrl}/v1`,
                    model: 'synthetic-chat',
                    apiKey: 'synthetic-chat-key'
                })
                service.saveEmbedding(owner, {
                    baseUrl: `${baseUrl}/v1`,
                    model: 'synthetic-embedding',
                    apiKey: 'synthetic-embedding-key'
                })
                await run({ action: 'sync', mode: 'hybrid' })
                assert.ok(embeddingCalls > 0)
                const asked = (
                    await run({ action: 'ask', mode: 'hybrid', query: { question: 'queue' } })
                ).knowledge
                assert.equal(asked.answers.length, 1)
                assert.equal(asked.retrieval.chunks[0].source.id, sources[0].id)
                sources = [
                    {
                        ...sources[0],
                        title: 'Updated synthetic queue PDF',
                        revision: 'b'.repeat(64)
                    }
                ]
                await assert.rejects(
                    run({ action: 'open', documentId: id, chunkId: retrieved.chunkId }),
                    /变化|不存在/
                )
                assert.equal((await run({ action: 'sync' })).knowledge.sync.indexed, 1)
                assert.equal(
                    (await run({ action: 'list' }, 'b'.repeat(64))).knowledge.documents.length,
                    0
                )
                withdrawDuringGeneration = true
                await assert.rejects(run({ action: 'ask', query: { question: 'queue' } }), /变化/)
                assert.equal((await run({ action: 'list' })).knowledge.documents.length, 0)
                assert.equal(
                    (
                        await service.run(
                            owner,
                            {
                                requestId: 'local-separate',
                                task: 'knowledge',
                                knowledge: { action: 'list' }
                            },
                            1
                        )
                    ).knowledge.documents.length,
                    0
                )
            } finally {
                service.forget(owner)
                server.closeAllConnections()
                await new Promise((resolve) => server.close(resolve))
                await rm(directory, { recursive: true, force: true })
            }
        }
    )
}
