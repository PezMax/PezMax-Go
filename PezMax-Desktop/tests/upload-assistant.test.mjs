import assert from 'node:assert/strict'
import test from 'node:test'
import { createUploadAssistantSession } from '../src/renderer/utils/upload-assistant-session.mjs'
import { createAssistantSession } from '../src/renderer/utils/assistant-session.mjs'
import { createAssistantService } from '../src/main/main-utils/assistant-service.mjs'
import {
    normalizeUploadInput,
    normalizeUploadSuggestions
} from '../src/shared/upload-assistant.mjs'

const upload = { kind: 'exam', folder: false, names: ['示例大学2026高等数学期末.pdf'] }
const context = { server: 'https://catalog.example', userId: 'test-user' }
const config = { hasApiKey: true, model: 'fixture', baseUrl: 'https://model.example' }
const suggestions = [
    { field: 'fileSchool', value: '示例大学', evidence: '示例大学' },
    { field: 'fileYear', value: '2026', evidence: '2026' },
    { field: 'fileType', value: 1, evidence: '期末' }
]
function deferred() {
    let resolve
    const promise = new Promise((done) => {
        resolve = done
    })
    return { resolve, promise }
}
function fixture(run = async () => ({ suggestions })) {
    const calls = [],
        listeners = new Set()
    const session = createUploadAssistantSession({
        call: async (method, scope, input) => {
            calls.push({ method, scope, input })
            if (method === 'getConfig') return config
            if (method === 'run') return run(input)
        },
        subscribe: (callback) => {
            listeners.add(callback)
            return () => listeners.delete(callback)
        }
    })
    return { session, calls, listeners }
}

test('suggestions require explicit selection and preserve edits made during or after generation', async () => {
    const f = fixture(),
        form = { fileSchool: '我的默认学校', fileYear: '', fileType: null }
    await f.session.setIdentity(context)
    await f.session.generate(upload, form)
    assert.equal(form.fileSchool, '我的默认学校')
    assert.equal(
        f.session.state.suggestions.every((item) => !item.selected),
        true
    )
    f.session.apply(form)
    assert.equal(form.fileYear, '')
    for (const item of f.session.state.suggestions) item.selected = true
    form.fileSchool = '手动改过的学校'
    f.session.apply(form)
    assert.deepEqual(form, { fileSchool: '手动改过的学校', fileYear: '2026', fileType: 1 })
    assert.match(f.session.state.feedback, /已跳过/)
    const payload = f.calls.find((item) => item.method === 'run').input
    assert.deepEqual(Object.keys(payload).sort(), ['requestId', 'task', 'upload'])
    assert.equal(payload.task, 'upload')
    assert.equal(f.listeners.size, 0)
})

test('file resets, stop, identity changes and unmount ignore late results and detach progress', async () => {
    for (const action of ['reset', 'stop', 'identity', 'dispose']) {
        const pending = deferred(),
            f = fixture(() => pending.promise)
        await f.session.setIdentity(context)
        const running = f.session.generate(upload, {})
        const id = f.calls.find((item) => item.method === 'run').input.requestId
        for (const listener of f.listeners) listener({ requestId: 'other', status: 'wrong' })
        assert.equal(f.session.state.status, '')
        for (const listener of f.listeners) listener({ requestId: id, status: 'working' })
        assert.equal(f.session.state.status, 'working')
        if (action === 'identity') await f.session.setIdentity({ ...context, userId: 'other' })
        else f.session[action]()
        pending.resolve({ suggestions })
        await running
        assert.equal(f.session.state.suggestions.length, 0, action)
        assert.equal(f.session.state.busy, false, action)
        assert.equal(f.listeners.size, 0, action)
        assert.ok(
            f.calls.some((item) => item.method === 'cancel'),
            action
        )
    }
})

test('errors permit retry and empty results tell users to fill the form manually', async () => {
    let fail = true
    const f = fixture(async () => {
        if (fail) throw new Error('API Key 无效')
        return { suggestions: [] }
    })
    await f.session.setIdentity(context)
    await f.session.generate(upload, {})
    assert.match(f.session.state.error, /API Key/)
    fail = false
    await f.session.generate(upload, {})
    assert.equal(f.session.state.error, '')
    assert.match(f.session.state.feedback, /手动填写/)
})

test('retry waits for the cancelled worker to exit before starting another request', async () => {
    const pending = deferred()
    let runs = 0
    const f = fixture(async () => {
        runs++
        return runs === 1 ? pending.promise : { suggestions }
    })
    await f.session.setIdentity(context)
    const first = f.session.generate(upload, {})
    f.session.stop()
    const retry = f.session.generate(upload, {})
    assert.equal(runs, 1)
    assert.equal(f.session.state.busy, true)
    pending.resolve({ suggestions: [] })
    await Promise.all([first, retry])
    assert.equal(runs, 2)
    assert.equal(f.session.state.suggestions.length, 3)
})

test('input boundaries strip extra properties and reject paths, excessive folders and mixed modes', () => {
    assert.deepEqual(
        normalizeUploadInput({ ...upload, path: 'C:/private', contents: 'secret' }),
        upload
    )
    for (const input of [
        { ...upload, names: ['C:\\private\\a.pdf'] },
        { ...upload, names: ['../a.pdf'] },
        { ...upload, folder: true, names: Array(21).fill('a.pdf') },
        { ...upload, folder: true, kind: 'ebook' }
    ]) {
        assert.throws(() => normalizeUploadInput(input))
    }
    const folder = { kind: 'exam', folder: true, names: ['2024数学期末.pdf', '2025数学期末.pdf'] }
    assert.deepEqual(
        normalizeUploadSuggestions(
            [
                { field: 'fileYear', value: '2024', evidence: '20' },
                { field: 'fileName', value: 'new', evidence: '数学' },
                { field: 'fileSchool', value: '编造学校', evidence: '数学' },
                { field: 'fileType', value: true, evidence: '期末' },
                { field: 'fileType', value: 1, evidence: '期末' }
            ],
            folder
        ),
        [{ field: 'fileType', value: 1, evidence: '期末' }]
    )
})

test('main process sends filenames alone to upload workers and validates returned suggestions', async () => {
    let received
    const service = createAssistantService({
        readConfig: () => ({}),
        writeConfig() {},
        encryption: { isAvailable: () => false },
        catalogBaseUrl: 'https://catalog.example',
        runAgent: async (value) => {
            received = value
            return { suggestions }
        }
    })
    service.save('user', { ...config, apiKey: 'synthetic-key' })
    const result = await service.run(
        'user',
        {
            requestId: 'upload-test',
            task: 'upload',
            upload: { ...upload, path: 'C:/private' },
            platformToken: 'synthetic-platform-token',
            question: 'private-question',
            history: [{ private: true }]
        },
        1
    )
    assert.deepEqual(received.upload, upload)
    assert.equal(received.task, 'upload')
    for (const value of ['synthetic-platform-token', 'private-question', 'C:/private'])
        assert.equal(JSON.stringify(received).includes(value), false)
    assert.equal(received.catalog, undefined)
    assert.deepEqual(result.suggestions, suggestions)
    await assert.rejects(service.run('user', { requestId: 'bad', task: 'other' }, 1), /任务无效/)
})

test('mounting and closing upload UI preserves the memory-only key for the search workspace', async () => {
    const service = createAssistantService({
        readConfig: () => ({}),
        writeConfig() {},
        encryption: { isAvailable: () => false }
    })
    service.save('user', { ...config, apiKey: 'synthetic-key' })
    const call = async (method) => {
        if (method === 'getConfig') return service.publicConfig('user')
        if (method === 'release') service.forget('user')
    }
    const uploader = createUploadAssistantSession({ call })
    await uploader.setIdentity(context)
    assert.equal(uploader.state.config.hasApiKey, true)
    uploader.dispose()
    const search = createAssistantSession({ call })
    await search.setIdentity(context)
    assert.equal(search.state.config.hasApiKey, true)
    search.dispose()
    assert.equal(service.publicConfig('user').hasApiKey, false)
})
