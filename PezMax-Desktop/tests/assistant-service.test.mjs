import assert from 'node:assert/strict'
import test from 'node:test'
import { createAssistantService } from '../src/main/main-utils/assistant-service.mjs'
import { installAssistantHandlers, assistantScope, isAssistantDocument } from '../src/main/main-utils/assistant-ipc.mjs'
import { normalizeProviderConfig } from '../src/shared/assistant-protocol.mjs'

const settings = { baseUrl: 'https://model.example/v1/', model: 'tool-model', apiKey: 'fixture-secret', remember: false }
function fixture(options = {}) {
  let stored = {}
  const calls = []
  const encryption = { isAvailable: () => true, encrypt: key => `encrypted:${key}`, decrypt: key => key.slice(10) }
  const service = createAssistantService({
    readConfig: () => stored, writeConfig: next => { stored = next }, encryption,
    catalogBaseUrl: 'https://catalog.example/api',
    runAgent: async (input, callbacks) => {
      calls.push({ input, callbacks })
      return { content: '连接成功 fixture-secret platform-token', resources: [], reports: [], messages: [] }
    }, ...options
  })
  return { service, calls, stored: () => stored }
}
const request = (requestId = 'req-1') => ({ requestId, question: '找线性代数资料', history: [], platformToken: 'platform-token' })

test('provider validation requires secure transport and forbids credentials in URLs', () => {
  assert.equal(normalizeProviderConfig(settings).baseUrl, 'https://model.example/v1')
  assert.equal(normalizeProviderConfig({ ...settings, baseUrl: 'http://127.0.0.1:1234/v1/chat/completions' }).baseUrl, 'http://127.0.0.1:1234/v1')
  for (const baseUrl of ['http://example.com', 'https://user:pass@example.com', 'https://example.com/?key=secret', 'file:///tmp']) {
    assert.throws(() => normalizeProviderConfig({ ...settings, baseUrl }))
  }
  assert.throws(() => normalizeProviderConfig({ ...settings, apiKey: 'a\nb' }))
})

test('keys stay memory-only by default; persisted keys are encrypted and isolated by account', async () => {
  const f = fixture()
  const publicConfig = f.service.save('account-a', settings)
  assert.equal(publicConfig.hasApiKey, true)
  assert.ok(!JSON.stringify(f.stored()).includes(settings.apiKey))
  assert.ok(!JSON.stringify(publicConfig).includes(settings.apiKey))
  assert.equal(f.service.publicConfig('account-b').hasApiKey, false)
  await f.service.run('account-a', request(), 1)
  assert.equal(f.calls[0].input.provider.baseUrl, 'https://model.example/v1')
  assert.equal(f.calls[0].input.provider.apiKey, 'fixture-secret')
  assert.equal(f.calls[0].input.catalog.token, 'platform-token')
  assert.equal(f.calls[0].input.catalog.baseUrl, 'https://catalog.example/api')
  assert.ok(!JSON.stringify(await f.service.run('account-a', request(), 1)).includes(settings.apiKey))
  assert.ok(!JSON.stringify(await f.service.run('account-a', request(), 1)).includes('platform-token'))
  f.service.forget('account-a')
  assert.equal(f.service.publicConfig('account-a').hasApiKey, false)
  f.service.save('account-a', { ...settings, remember: true })
  assert.equal(f.stored()['account-a'].encryptedKey, 'encrypted:fixture-secret')
  f.service.forget('account-a')
  assert.equal(f.service.publicConfig('account-a').hasApiKey, true)
  f.service.clear('account-a')
  assert.equal(f.stored()['account-a'].encryptedKey, undefined)
})

test('provider changes cannot silently reuse a key; unavailable encryption cannot persist keys', () => {
  const { service } = fixture({ encryption: { isAvailable: () => false } })
  service.save('a', settings)
  service.save('a', { ...settings, apiKey: '', model: 'new-model' })
  assert.throws(() => service.save('a', { ...settings, apiKey: '', baseUrl: 'https://other.example/v1' }), /重新输入/)
  assert.throws(() => service.save('a', { ...settings, remember: true }), /不支持安全保存/)
})

test('worker failures are sanitized and connection tests do not receive platform credentials', async () => {
  let payload
  const { service } = fixture({ runAgent: async (input) => { payload = input; throw new Error('internal fixture-secret details') } })
  service.save('a', settings)
  await assert.rejects(service.run('a', { requestId: 'test', test: true, platformToken: 'not-needed' }, 1), /助手请求失败/)
  assert.equal(payload.catalog, undefined)
  assert.equal(JSON.stringify(payload).includes('not-needed'), false)
  await assert.rejects(service.run('a', { ...request(), platformToken: '' }, 1), /登录/)
})

test('cancellation belongs to the owning window, and releases permit retry', async () => {
  const { service } = fixture({ runAgent: async (_payload, { signal }) => new Promise((_resolve, reject) => {
    signal.addEventListener('abort', () => reject(new Error('abort')), { once: true })
  }) })
  service.save('a', settings)
  const pending = service.run('a', request(), 1)
  service.cancel('req-1', 2)
  await assert.rejects(service.run('a', request('req-2'), 1), /等待/)
  service.cancel('req-1', 1)
  await assert.rejects(pending, /已停止/)
  const retry = service.run('a', request(), 1)
  service.forget('a')
  await assert.rejects(retry, /已停止/)
})

test('overall worker timeout is bounded', async () => {
  const slow = fixture({ timeoutMs: 5, runAgent: (_payload, { signal }) => new Promise((_resolve, reject) => {
    signal.addEventListener('abort', () => reject(new Error('timeout')), { once: true })
  }) }).service
  slow.save('a', settings)
  await assert.rejects(slow.run('a', request(), 1), /超时/)
})

test('IPC denies preview iframes and other windows; switching account forgets session key', async () => {
  const handlers = new Map()
  const { service } = fixture()
  const frame = { url: 'http://localhost:80/#/index' }
  const webContents = { id: 1, mainFrame: frame }
  const bridge = installAssistantHandlers({ ipcMain: { handle: (name, run) => handlers.set(name, run) },
    getWindow: () => ({ isDestroyed: () => false, webContents }), service, rendererUrl: 'http://localhost:80' })
  assert.ok(bridge.attachWindow)
  const context = { server: '/api', userId: 'user-1' }
  const event = { sender: webContents, senderFrame: frame }
  const save = handlers.get('assistant:save')
  assert.equal((await save({ ...event, senderFrame: {} }, context, settings)).ok, false)
  assert.equal((await save({ ...event, sender: {} }, context, settings)).ok, false)
  frame.url = 'https://untrusted.example/'
  assert.equal((await save(event, context, settings)).ok, false)
  frame.url = 'http://localhost:80/#/index'
  assert.equal((await save(event, context, settings)).ok, true)
  await handlers.get('assistant:getConfig')(event, { ...context, userId: 'user-2' })
  assert.equal(service.publicConfig(assistantScope(context)).hasApiKey, false)
  assert.notEqual(assistantScope(context), assistantScope({ ...context, server: '/other' }))
})

test('trusted document matching allows app routes but denies other files, origins and ports', () => {
  assert.equal(isAssistantDocument('file:///C:/app/index.html#/index?view=assistant', 'file:///C:/app/index.html'), true)
  assert.equal(isAssistantDocument('http://localhost:80/#/index', 'http://localhost:80'), true)
  assert.equal(isAssistantDocument('file:///C:/other/index.html', 'file:///C:/app/index.html'), false)
  assert.equal(isAssistantDocument('http://localhost:81/', 'http://localhost:80'), false)
  assert.equal(isAssistantDocument('http://localhost:80/remote.html', 'http://localhost:80'), false)
})

test('IPC fixes the catalog host at build configuration and only returns sanitized worker progress', async () => {
  const handlers = new Map(), sent = []
  const f = fixture({ runAgent: async (_payload, { onProgress }) => {
    onProgress({ status: 'fixture-secret platform-token' })
    return { content: '完成', resources: [], reports: [], messages: [] }
  } })
  const frame = { url: 'file:///app/index.html#/index' }
  const webContents = { id: 1, mainFrame: frame, send: (name, data) => sent.push({ name, data }) }
  installAssistantHandlers({ ipcMain: { handle: (name, run) => handlers.set(name, run) },
    getWindow: () => ({ isDestroyed: () => false, webContents }), service: f.service,
    rendererUrl: 'file:///app/index.html', catalogBaseUrl: 'https://catalog.example/api' })
  const event = { sender: webContents, senderFrame: frame }
  const context = { server: 'https://renderer-cannot-pick.example', userId: 'user-1' }
  await handlers.get('assistant:save')(event, context, settings)
  const result = await handlers.get('assistant:run')(event, context, request())
  assert.equal(result.ok, true)
  assert.equal(handlers.has('assistant:complete'), false)
  assert.equal(sent[0].name, 'assistant:progress')
  assert.equal(sent[0].data.requestId, 'req-1')
  assert.equal(JSON.stringify(sent).includes('fixture-secret'), false)
  assert.equal(JSON.stringify(sent).includes('platform-token'), false)
  assert.equal(f.service.publicConfig(assistantScope({ ...context, server: 'https://catalog.example/api' })).hasApiKey, true)
})

test('hash navigation retains a session key; reloads and renderer loss release it', async () => {
  const handlers = new Map(), listeners = new Map()
  const f = fixture()
  const frame = { url: 'file:///app/index.html#/index' }
  const webContents = { id: 1, mainFrame: frame, on: (event, run) => listeners.set(event, run), once: (event, run) => listeners.set(event, run) }
  const win = { isDestroyed: () => false, webContents }
  const bridge = installAssistantHandlers({ ipcMain: { handle: (name, run) => handlers.set(name, run) }, getWindow: () => win, service: f.service, rendererUrl: 'file:///app/index.html' })
  bridge.attachWindow(win)
  const context = { server: '/api', userId: 'user-1' }
  await handlers.get('assistant:save')({ sender: webContents, senderFrame: frame }, context, settings)
  const scope = assistantScope(context)
  listeners.get('did-start-navigation')({}, 'file:///app/index.html#/index?view=ebook', true, true)
  assert.equal(f.service.publicConfig(scope).hasApiKey, true)
  listeners.get('did-start-navigation')({}, 'file:///app/index.html', false, true)
  assert.equal(f.service.publicConfig(scope).hasApiKey, false)
})
