import test from 'node:test'
import assert from 'node:assert/strict'
import {
    normalizeReadingInput,
    normalizeReadingResult,
    pdfPageUrl,
    readingModes
} from '../src/shared/reading-assistant.mjs'
import { createReadingAssistantSession } from '../src/renderer/utils/reading-assistant-session.mjs'
import { createAssistantService } from '../src/main/main-utils/assistant-service.mjs'

const context = { server: 'https://catalog.example', userId: 'fixture', token: 'private-token' }
const pages = [{ page: 2, text: 'The derivative measures the rate of change.' }]
const answer = {
    text: 'It measures the rate of change.',
    citations: [{ page: 2, quote: 'rate of change' }]
}
const result = { answers: [answer], contextPages: [2], partial: false }
const reading = { question: 'What does the derivative measure?', pages }
const config = { baseUrl: 'https://model.example/v1', model: 'fixture', hasApiKey: true }
const deferred = () => {
    let resolve
    const promise = new Promise((done) => {
        resolve = done
    })
    return { resolve, promise }
}
function fixture(options = {}) {
    const calls = [],
        listeners = new Set(),
        documents = []
    const session = createReadingAssistantSession({
        call: async (method, scope, input) => {
            calls.push({ method, scope, input })
            if (method === 'getConfig') return config
            if (method === 'run') return options.run ? options.run(input) : result
        },
        subscribe: (listener) => {
            listeners.add(listener)
            return () => listeners.delete(listener)
        },
        loadPdf:
            options.loadPdf ||
            (async () => {
                const document = {
                    numPages: 3,
                    destroyed: false,
                    pages: async () => pages,
                    destroy: async () => {
                        document.destroyed = true
                    }
                }
                documents.push(document)
                return document
            }),
        ...options
    })
    return { session, calls, listeners, documents }
}
async function prepare(f) {
    await f.session.setIdentity(context)
    f.session.setDocument({ id: 'local-fixture', source: 'blob:private-source' })
    f.session.setRange(2, 2)
    await f.session.extract()
}

test('reading contracts whitelist payloads and require real quotations on the claimed page', () => {
    assert.deepEqual(
        normalizeReadingInput({
            ...reading,
            url: 'private',
            pages: [{ ...pages[0], path: 'private' }]
        }),
        reading
    )
    for (const input of [
        { ...reading, question: ' ' },
        { ...reading, pages: [] },
        { ...reading, pages: [pages[0], pages[0]] },
        { ...reading, pages: [{ page: true, text: 'test' }] },
        { ...reading, pages: [{ page: 10001, text: 'test' }] },
        { ...reading, pages: [{ page: 1, text: 'x'.repeat(20001) }] },
        {
            ...reading,
            pages: Array.from({ length: 5 }, (_, i) => ({ page: i + 1, text: 'x'.repeat(20000) }))
        }
    ])
        assert.throws(() => normalizeReadingInput(input))
    assert.deepEqual(normalizeReadingResult(result, reading).answers, [answer])
    for (const citations of [
        [{ page: 1, quote: 'rate of change' }],
        [{ page: 2, quote: 'made-up quote' }],
        [...answer.citations, { page: 2, quote: 'wrong' }]
    ]) {
        const checked = normalizeReadingResult(
            { ...result, answers: [{ ...answer, citations }] },
            reading
        )
        assert.deepEqual(checked.answers, [])
        assert.match(checked.content, /没有足够/)
    }
    assert.equal(
        pdfPageUrl('/api/content?token=fixture#old', 2),
        '/api/content?token=fixture#page=2'
    )
    assert.equal(pdfPageUrl('blob:https://local/fixture', 3), 'blob:https://local/fixture#page=3')
    for (const page of [0, -1, 1.5, true, 10001]) assert.throws(() => pdfPageUrl('/a.pdf', page))
    assert.throws(() => pdfPageUrl('javascript:alert(1)', 1))
})

test('extraction is explicit, stays local and destroys the PDF before a separately requested model call', async () => {
    const f = fixture()
    await f.session.setIdentity(context)
    f.session.setDocument({ id: 'local-fixture', source: 'blob:private-source' })
    assert.equal(f.documents.length, 0)
    assert.equal(
        f.calls.some((call) => call.method === 'run'),
        false
    )
    f.session.setRange(2, 2)
    await f.session.extract()
    assert.equal(f.documents[0].destroyed, true)
    assert.equal(
        f.calls.some((call) => call.method === 'run'),
        false
    )
    await f.session.ask(reading.question)
    assert.deepEqual(f.session.state.result.answers, [answer])
    const payload = f.calls.find((call) => call.method === 'run')
    assert.deepEqual(payload.scope, { server: context.server, userId: context.userId })
    assert.deepEqual(Object.keys(payload.input).sort(), ['reading', 'requestId', 'task'])
    assert.deepEqual(payload.input.reading, reading)
    assert.equal(f.listeners.size, 0)
})

test('document, range, account, close, stop and dispose reject late answers and detach progress', async () => {
    for (const action of ['document', 'range', 'identity', 'clear', 'stop', 'dispose']) {
        const pending = deferred(),
            f = fixture({ run: () => pending.promise })
        await prepare(f)
        const running = f.session.ask(reading.question)
        const id = f.calls.find((call) => call.method === 'run').input.requestId
        for (const listener of f.listeners) listener({ requestId: 'other', status: 'wrong' })
        assert.equal(f.session.state.status, '')
        for (const listener of f.listeners) listener({ requestId: id, status: 'working' })
        assert.equal(f.session.state.status, 'working')
        if (action === 'document') f.session.setDocument({ id: 'other', source: 'blob:other' })
        else if (action === 'range') f.session.setRange(1, 1)
        else if (action === 'identity') await f.session.setIdentity({ ...context, userId: 'other' })
        else f.session[action]()
        pending.resolve(result)
        await running
        assert.equal(f.session.state.result, null, action)
        assert.equal(f.session.state.busy, false, action)
        assert.equal(f.listeners.size, 0, action)
        assert.ok(
            f.calls.some((call) => call.method === 'cancel'),
            action
        )
    }
})

test('late extraction is destroyed after a file change and cancellation retries drain the old worker', async () => {
    const extraction = deferred(),
        pending = deferred()
    let destroyed = 0,
        runs = 0
    const f = fixture({
        loadPdf: () => extraction.promise,
        run: async () => (++runs === 1 ? pending.promise : result)
    })
    await f.session.setIdentity(context)
    f.session.setDocument({ id: 'first', source: 'blob:first' })
    const work = f.session.extract()
    f.session.setDocument({ id: 'second', source: 'blob:second' })
    extraction.resolve({
        numPages: 3,
        pages: async () => pages,
        destroy: async () => {
            destroyed++
        }
    })
    await work
    assert.equal(destroyed, 1)
    assert.deepEqual(f.session.state.pages, [])
    f.session.setRange(2, 2)
    await f.session.extract()
    const first = f.session.ask(reading.question)
    f.session.stop()
    const retry = f.session.ask(reading.question)
    assert.equal(runs, 1)
    pending.resolve(result)
    await Promise.all([first, retry])
    assert.equal(runs, 2)
    assert.deepEqual(f.session.state.result.answers, [answer])
})

test('extraction timeout and failures never invoke the model; answer failures allow retry', async () => {
    const f = fixture({
        extractionTimeoutMs: 5,
        loadPdf: async (_source, { signal }) =>
            new Promise((_resolve, reject) =>
                signal.addEventListener('abort', () => reject(new Error('stopped')), { once: true })
            )
    })
    await prepare(f)
    assert.match(f.session.state.error, /超时/)
    assert.equal(
        f.calls.some((call) => call.method === 'run'),
        false
    )
    let fail = true
    const g = fixture({
        run: async () => {
            if (fail) throw new Error('模型连接失败')
            return result
        }
    })
    await prepare(g)
    await g.session.ask(reading.question)
    assert.match(g.session.state.error, /连接失败/)
    fail = false
    await g.session.ask(reading.question)
    assert.equal(g.session.state.error, '')
    assert.deepEqual(g.session.state.result.answers, [answer])
    g.session.dispose()
    assert.equal(
        g.calls.some((call) => call.method === 'release'),
        false
    )
})

test('settings queued behind a stopped worker cannot mutate another account', async () => {
    const pending = deferred(),
        f = fixture({ run: () => pending.promise })
    await prepare(f)
    const work = f.session.ask(reading.question)
    f.session.stop()
    const saving = f.session.saveSettings({ model: 'fixture', apiKey: 'synthetic-key' })
    const rejected = assert.rejects(saving, /账号或资料已变化/)
    await f.session.setIdentity({ ...context, userId: 'other' })
    pending.resolve(result)
    await Promise.all([work, rejected])
    assert.equal(
        f.calls.some((call) => call.method === 'save'),
        false
    )
})

test('EPUB summary and outline require explicit extraction and keep chapter locations separate from pages', async () => {
    const chapters = [{ chapter: 2, text: pages[0].text }]
    const chapterAnswer = {
        text: answer.text,
        citations: [{ chapter: 2, quote: 'rate of change' }]
    }
    const f = fixture({
        loadEpub: async () => ({
            units: [
                { chapter: 1, label: 'First' },
                { chapter: 2, label: 'Derivatives' }
            ],
            chapters: async () => chapters,
            destroy: async () => {}
        }),
        run: async () => ({ answers: [chapterAnswer], contextChapters: [2], partial: false })
    })
    await f.session.setIdentity(context)
    f.session.setDocument({ id: 'local-epub', source: '', format: 'epub' })
    await f.session.loadChapters()
    assert.equal(
        f.calls.some((call) => call.method === 'run'),
        false
    )
    assert.deepEqual(f.session.state.chapters, [])
    f.session.setChapters([2])
    await f.session.extract()
    for (const mode of ['summary', 'outline']) {
        f.session.setMode(mode)
        assert.equal(f.session.state.result, null)
        assert.deepEqual(f.session.state.chapters, chapters)
        await f.session.ask('private-override')
        assert.deepEqual(f.session.state.result.answers, [chapterAnswer])
        const payload = f.calls.filter((call) => call.method === 'run').at(-1).input.reading
        assert.deepEqual(payload, {
            format: 'epub',
            mode,
            question: readingModes[mode].prompt,
            chapters
        })
        assert.equal(payload.pages, undefined)
        assert.deepEqual(
            normalizeReadingResult({ answers: [answer], contextChapters: [2] }, payload).answers,
            []
        )
    }
    f.session.setChapters([1])
    assert.equal(f.session.state.result, null)
    assert.deepEqual(f.session.state.chapters, [])
})

test('overview contracts ignore injected tasks and raw EPUB metadata and enforce per-section limits', () => {
    for (const mode of ['summary', 'outline']) {
        const input = normalizeReadingInput({
            format: 'epub',
            mode,
            question: 'private-override',
            chapters: [
                {
                    chapter: 3,
                    text: 'Synthetic evidence.',
                    href: 'private-path',
                    label: 'private-label'
                }
            ]
        })
        assert.equal(input.question, readingModes[mode].prompt)
        assert.equal(JSON.stringify(input).includes('private-'), false)
    }
    for (const input of [
        { mode: 'review', pages },
        { mode: ['summary'], pages },
        { mode: 'summary', format: 'other', pages },
        { format: 'epub', mode: 'summary', chapters: [{ chapter: 1, text: 'x'.repeat(40001) }] },
        {
            format: 'epub',
            mode: 'summary',
            chapters: Array.from({ length: 6 }, (_, index) => ({
                chapter: index + 1,
                text: 'test'
            }))
        }
    ])
        assert.throws(() => normalizeReadingInput(input))
})

test('reading diagnostics expose only bounded call counts and elapsed time', () => {
    const actual = normalizeReadingResult(
        {
            ...result,
            diagnostics: {
                task: 'reading',
                modelCalls: 2,
                durationMs: 1234,
                apiKey: 'synthetic-untrusted',
                question: 'private-question',
                steps: [{ secret: true }]
            }
        },
        reading
    )
    assert.deepEqual(actual.diagnostics, { modelCalls: 2, durationMs: 1234 })
    assert.equal(JSON.stringify(actual).includes('synthetic-untrusted'), false)
    for (const diagnostics of [
        { task: 'search', modelCalls: 1, durationMs: 12 },
        { task: 'reading', modelCalls: 3, durationMs: 12 },
        { task: 'reading', modelCalls: 1, durationMs: -1 }
    ])
        assert.equal(
            normalizeReadingResult({ ...result, diagnostics }, reading).diagnostics,
            undefined
        )
})

test('main process omits catalog, token, URL and history from reading workers and rechecks evidence', async () => {
    let received
    const service = createAssistantService({
        readConfig: () => ({}),
        writeConfig() {},
        encryption: { isAvailable: () => false },
        catalogBaseUrl: 'https://private-catalog',
        runAgent: async (payload) => {
            received = payload
            return {
                ...result,
                answers: [
                    answer,
                    { text: 'wrong', citations: [{ page: 8, quote: 'invalid evidence' }] }
                ]
            }
        }
    })
    service.save('user', { ...config, apiKey: 'synthetic-key' })
    const actual = await service.run(
        'user',
        {
            requestId: 'reading-test',
            task: 'reading',
            reading: { ...reading, url: 'private-source' },
            platformToken: 'private-token',
            history: [{ private: true }],
            question: 'private-question'
        },
        1
    )
    assert.deepEqual(received.reading, reading)
    assert.deepEqual(Object.keys(received).sort(), [
        'provider',
        'reading',
        'requestId',
        'task',
        'version'
    ])
    assert.equal(JSON.stringify(received).includes('private-'), false)
    assert.deepEqual(actual.answers, [answer])
})
