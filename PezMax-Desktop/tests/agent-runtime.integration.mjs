import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { once } from 'node:events'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createAgentProcess, resolveAgentRuntime } from '../src/main/main-utils/assistant-process.mjs'

const appPath = fileURLToPath(new URL('../', import.meta.url))
const root = resolve(appPath, '../PezMax-Agent')

for (const packaged of [false, true]) {
  test(`real ${packaged ? 'frozen' : 'Python'} LangGraph worker searches and cancels over stdio`, { timeout: 45000 }, async () => {
    const captured = []
    let hanging = false, entered
    const server = createServer(async (request, response) => {
      response.setHeader('Content-Type', 'application/json')
      if (request.method === 'POST') {
        let data = ''
        for await (const chunk of request) data += chunk
        const body = JSON.parse(data)
        captured.push({ kind: 'model', auth: request.headers.authorization, body })
        if (hanging) { entered(); return }
        const message = body.messages.at(-1).role === 'tool'
          ? { role: 'assistant', content: '找到数学试卷 fixture-api-key' }
          : { role: 'assistant', content: '', tool_calls: [{ id: 'call-1', type: 'function', function: {
            name: 'search_resources', arguments: JSON.stringify({ kinds: ['file'], keyword: '数学' })
          } }] }
        response.end(JSON.stringify({ choices: [{ message, finish_reason: 'stop' }] }))
      } else {
        captured.push({ kind: 'catalog', auth: request.headers.authorization, url: request.url })
        response.end(JSON.stringify({ code: 0, rows: [{ fileId: 18, fileStatus: 1, fileName: '数学试卷', fileUrl: '/private/math.pdf' }], total: 1 }))
      }
    })
    server.listen(0, '127.0.0.1')
    await once(server, 'listening')
    const baseUrl = `http://127.0.0.1:${server.address().port}`
    const runAgent = createAgentProcess({
      environment: { ...process.env, HTTP_PROXY: '', HTTPS_PROXY: '', ALL_PROXY: '', NO_PROXY: '127.0.0.1' },
      resolveRuntime: () => packaged
        ? { file: resolve(root, 'dist/pezmax-agent', process.platform === 'win32' ? 'pezmax-agent.exe' : 'pezmax-agent'), args: [], cwd: root }
        : resolveAgentRuntime({ isPackaged: false, appPath })
    })
    const payload = { version: 1, requestId: 'integration',
      provider: { baseUrl: baseUrl + '/v1', model: 'fixture-model', apiKey: 'fixture-api-key' },
      catalog: { baseUrl, token: 'fixture-platform-token' }, history: [], question: '数学' }
    try {
      const progress = []
      const result = await runAgent(payload, { onProgress: event => progress.push(event) })
      assert.equal(result.resources[0].node.rawId, '18')
      assert.ok(progress.some(event => event.resources?.length))
      assert.equal(JSON.stringify(result).includes('fixture-api-key'), false)
      assert.deepEqual(captured.map(item => item.kind), ['model', 'catalog', 'model'])
      assert.equal(captured[1].auth, 'Bearer fixture-platform-token')
      const modelRequests = JSON.stringify(captured.filter(item => item.kind === 'model').map(item => item.body))
      assert.equal(modelRequests.includes('fixture-platform-token'), false)
      assert.equal(modelRequests.includes('/private/'), false)
      hanging = true
      const started = new Promise(resolveStarted => { entered = resolveStarted })
      const controller = new AbortController()
      const pending = runAgent({ ...payload, requestId: 'cancel-integration' }, { signal: controller.signal })
      const cancelled = assert.rejects(pending, /已停止/)
      await started
      controller.abort()
      await cancelled
    } finally {
      server.closeAllConnections()
      await new Promise(resolveClosed => server.close(resolveClosed))
    }
  })
}
