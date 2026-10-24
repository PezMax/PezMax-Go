import test from 'node:test'
import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import { PassThrough } from 'node:stream'
import { agentEnvironment, createAgentProcess, resolveAgentRuntime } from '../src/main/main-utils/assistant-process.mjs'

const payload = { version: 1, requestId: 'r1', provider: { apiKey: 'only-in-stdin' } }
const result = { content: '完成', messages: [], resources: [], reports: [] }
const line = (type, fields = {}) => Buffer.from(JSON.stringify({ version: 1, requestId: 'r1', type, ...fields }) + '\n')
function fixture(plan) {
  const calls = []
  const run = createAgentProcess({
    resolveRuntime: () => ({ file: '/runtime/python', args: ['-m', 'pezmax_agent'], cwd: '/module' }),
    environment: { PATH: '/bin', OPENAI_API_KEY: 'inherited-secret', LANGSMITH_API_KEY: 'trace-secret', PYTHONPATH: '/unsafe' },
    spawnImpl(file, args, options) {
      const child = new EventEmitter()
      child.stdout = new PassThrough()
      child.stderr = new PassThrough()
      child.stdin = new PassThrough()
      child.input = ''
      child.killed = false
      child.stdin.on('data', value => { child.input += value.toString() })
      child.kill = () => { child.killed = true; queueMicrotask(() => child.emit('close', null)); return true }
      calls.push({ file, args, options, child })
      queueMicrotask(() => plan(child))
      return child
    }
  })
  return { run, calls }
}

test('stdio keeps credentials out of argv/env and decodes fragmented UTF-8 progress', async () => {
  const f = fixture(child => {
    const data = Buffer.concat([line('progress', { data: { status: '正在检索目录' } }), line('result', { data: result })])
    for (const byte of data) child.stdout.write(Buffer.from([byte]))
    child.emit('close', 0)
  })
  const events = []
  assert.deepEqual(await f.run(payload, { onProgress: event => events.push(event) }), result)
  assert.equal(events[0].status, '正在检索目录')
  const call = f.calls[0]
  assert.equal(call.options.windowsHide, true)
  assert.equal(call.options.shell, false)
  assert.equal(JSON.parse(call.child.input).provider.apiKey, 'only-in-stdin')
  assert.equal(JSON.stringify(call.args).includes('only-in-stdin'), false)
  assert.equal(JSON.stringify(call.options.env).includes('inherited-secret'), false)
  assert.equal(JSON.stringify(call.options.env).includes('trace-secret'), false)
  assert.equal(call.options.env.PYTHONPATH, undefined)
  assert.equal(call.options.env.LANGSMITH_TRACING, 'false')
})

test('aborting the owning request terminates its worker and rejects after exit', async () => {
  const f = fixture(() => {})
  const controller = new AbortController()
  const pending = f.run(payload, { signal: controller.signal })
  controller.abort()
  await assert.rejects(pending, /已停止/)
  assert.equal(f.calls[0].child.killed, true)
  await assert.rejects(f.run(payload, { signal: controller.signal }), /已停止/)
  assert.equal(f.calls.length, 1)
})

test('protocol mismatch, malformed output and oversized lines terminate the worker', async () => {
  const badLines = [Buffer.from('null\n'), Buffer.from('internal error with secret\n'),
    line('progress', { requestId: 'other', data: {} }), Buffer.from('x'.repeat(8 * 1024 * 1024 + 1))]
  for (const value of badLines) {
    const f = fixture(child => child.stdout.write(value))
    await assert.rejects(f.run(payload), /格式不正确/)
    assert.equal(f.calls[0].child.killed, true)
  }
})

test('stderr is drained without echoing secrets and auth errors retain only the known code', async () => {
  const f = fixture(child => {
    child.stderr.write('traceback contains secret-data')
    child.stdout.write(line('error', { error: '平台登录已失效', code: 'catalog_auth_expired' }))
    child.emit('close', 1)
  })
  await assert.rejects(f.run(payload), error => error.code === 'catalog_auth_expired' && !error.message.includes('secret'))
  const tooBig = fixture(() => {})
  await assert.rejects(tooBig.run({ ...payload, question: 'x'.repeat(600000) }), /过长/)
  assert.equal(tooBig.calls.length, 0)
})

test('packaged builds require the matching bundled runtime and ignore developer overrides', () => {
  const options = { isPackaged: true, resourcesPath: '/app/resources', appPath: '/app', platform: 'win32', arch: 'x64',
    environment: { PEZMAX_AGENT_PYTHON: '/other/python' }, exists: () => true,
    readFile: () => JSON.stringify({ protocolVersion: 1, engine: 'langgraph', platform: 'win32', arch: 'x64' }) }
  const runtime = resolveAgentRuntime(options)
  assert.ok(runtime.file.endsWith('pezmax-agent.exe'))
  assert.deepEqual(runtime.args, [])
  assert.throws(() => resolveAgentRuntime({ ...options, arch: 'arm64' }), /不匹配/)
  assert.throws(() => resolveAgentRuntime({ ...options, exists: () => false }), /缺失/)
  const dev = resolveAgentRuntime({ ...options, isPackaged: false, environment: {} })
  assert.deepEqual(dev.args, ['-m', 'pezmax_agent'])
  assert.equal(agentEnvironment({ LANGCHAIN_TRACING_V2: 'true' }).LANGCHAIN_TRACING_V2, 'false')
})
