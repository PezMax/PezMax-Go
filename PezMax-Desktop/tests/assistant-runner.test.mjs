import test from 'node:test'
import assert from 'node:assert/strict'
import { boundedHistory, runResourceAssistant } from '../src/renderer/utils/assistant-runner.mjs'

const toolCall = (id, name = 'search_resources', args = { kinds: ['file'], keyword: '数学' }) => ({ id, type: 'function', function: { name, arguments: JSON.stringify(args) } })
const assistant = (content = '', tool_calls) => ({ role: 'assistant', content, ...(tool_calls ? { tool_calls } : {}) })

test('a real search round returns cards, tool evidence, and usable follow-up history', async () => {
  const progress = []
  const row = { id: 'file-7', title: '高等数学', node: { url: '/local-only' } }
  let rounds = 0
  const result = await runResourceAssistant({
    history: [{ role: 'user', content: '先找数学' }, assistant('请问需要哪一年？')],
    question: '2023年',
    complete: async (messages) => {
      if (++rounds === 1) { assert.equal(messages[0].content, '先找数学'); return assistant('', [toolCall('call-1')]) }
      assert.equal(messages.at(-1).role, 'tool')
      assert.equal(messages.at(-1).content.includes('local-only'), false)
      return assistant('找到一份2023年的资料，可以打开查看。')
    },
    search: async () => ({ resources: [row], modelResult: { resources: [{ id: row.id, title: row.title }], reports: [] } }),
    onProgress: event => progress.push(event)
  })
  assert.equal(result.resources[0], row)
  assert.equal(result.messages.at(-1).content, result.content)
  assert.equal(progress.some(event => event.resources?.length === 1), true)
})

test('unknown tools and malformed arguments stay local and let the model recover', async () => {
  let round = 0, searches = 0
  const result = await runResourceAssistant({ history: [], question: '数学',
    complete: async () => ++round === 1 ? assistant('', [toolCall('a', 'delete_resource'), { ...toolCall('b'), function: { name: 'search_resources', arguments: '{private-text' } }]) : assistant('请调整搜索条件。'),
    search: async () => { searches++ }
  })
  assert.equal(searches, 0)
  const tools = result.messages.filter(message => message.role === 'tool')
  assert.equal(tools.length, 2)
  assert.equal(tools[0].content.includes('不支持'), true)
  assert.equal(JSON.stringify(tools).includes('private-text'), false)
})

test('round and search limits terminate repeated tool calls with fully matched history', async () => {
  let calls = 0
  const result = await runResourceAssistant({ history: [], question: '数学',
    complete: async () => assistant('', Array.from({ length: 4 }, (_, index) => toolCall(`c-${calls}-${index}`))),
    search: async () => { calls++; return { resources: [], modelResult: { reports: [] } } }
  })
  assert.equal(calls, 8)
  assert.equal(result.messages.filter(message => message.role === 'tool').length, 20)
  assert.equal(result.messages.at(-1).role, 'assistant')
  assert.match(result.content, /上限/)
})

test('cancelled model work never runs the returned search', async () => {
  const controller = new AbortController()
  let searched = false
  await assert.rejects(runResourceAssistant({ history: [], question: '数学', signal: controller.signal,
    complete: async () => { controller.abort(); return assistant('', [toolCall('a')]) },
    search: async () => { searched = true }
  }), /停止/)
  assert.equal(searched, false)
})

test('history bounds discard whole turns, retaining tool call pairs', () => {
  const old = [{ role: 'user', content: 'x'.repeat(21000) }, assistant('', [toolCall('a')]), { role: 'tool', tool_call_id: 'a', content: '{}' }, assistant('完成')]
  const latest = [{ role: 'user', content: '最近一次' }, assistant('最近回复')]
  assert.deepEqual(boundedHistory([...old, ...latest]), latest)
  assert.deepEqual(boundedHistory(old), [])
})
