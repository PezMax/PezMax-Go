import test from 'node:test'
import assert from 'node:assert/strict'
import { createAssistantSession } from '../src/renderer/utils/assistant-session.mjs'

const config = {
    baseUrl: 'https://model.example/v1', model: 'tool-model',
    hasApiKey: true, remember: false, canRemember: true
}
const identity = { server: 'https://pezmax.example', userId: '7', token: 'platform-token' }
const answer = (content, history = [], question = '数学') => ({
    content, resources: [], reports: [],
    messages: [...history, { role: 'user', content: question }, { role: 'assistant', content }]
})
const deferred = () => {
    let resolve, reject
    const promise = new Promise((a, b) => { resolve = a; reject = b })
    return { promise, resolve, reject }
}

test('one worker call handles each turn; follow-ups use completed history and new conversations drop it', async () => {
    const calls = []
    const session = createAssistantSession({
        call: async (method, _scope, input) => {
            if (method === 'getConfig') return { ...config }
            if (method === 'run') {
                calls.push(structuredClone(input))
                return answer('找到了', input.history, input.question)
            }
        }
    })
    await session.setIdentity(identity)
    await session.send('高等数学')
    await session.send('只看2023年的')
    assert.deepEqual(calls[1].history.map(item => item.content), ['高等数学', '找到了'])
    assert.equal(calls[1].question, '只看2023年的')
    assert.equal(calls[0].platformToken, 'platform-token')
    assert.equal(JSON.stringify(session.state).includes('platform-token'), false)
    assert.equal(session.state.turns.length, 2)
    session.newConversation()
    await session.send('大学物理')
    assert.deepEqual(calls[2].history, [])
})

test('logout cancels work, releases keys, and ignores late responses from the old account', async () => {
    const pending = deferred(), events = []
    const session = createAssistantSession({
        call: async (method, scope) => {
            events.push([method, scope])
            if (method === 'getConfig') return { ...config, hasApiKey: scope.userId === '7' }
            if (method === 'run') return pending.promise
        }
    })
    await session.setIdentity(identity)
    const running = session.send('旧账号的问题')
    await session.setIdentity({ ...identity, userId: '8' })
    pending.resolve(answer('旧账号的私有回复'))
    await running
    assert.equal(session.state.turns.length, 0)
    assert.equal(session.state.config.hasApiKey, false)
    assert.equal(events.some(([method]) => method === 'cancel'), true)
    assert.equal(events.filter(([method]) => method === 'release').length, 1)
    await session.setIdentity(null)
    await assert.rejects(session.send('不能发送'), /登录/)
})

test('progress is request scoped; stop preserves real cards and retry drops failed history', async () => {
    const pending = deferred()
    let emit, runs = 0, cancelled = false, unsubscribed = 0
    const session = createAssistantSession({
        subscribe: (listener) => { emit = listener; return () => { unsubscribed++ } },
        call: async (method, _scope, input) => {
            if (method === 'getConfig') return { ...config }
            if (method === 'cancel') { cancelled = true; pending.reject(new Error('已停止生成')); return }
            if (method === 'run') {
                runs++
                if (runs === 1) {
                    emit({ requestId: 'some-other-request', resources: [{ id: 'wrong' }] })
                    assert.equal(session.state.turns[0].resources.length, 0)
                    emit({ requestId: input.requestId, resources: [{ id: 'file-1', title: '真实资料' }], report: [] })
                    return pending.promise
                }
                assert.deepEqual(input.history, [])
                return answer('重试成功', input.history, input.question)
            }
        }
    })
    await session.setIdentity(identity)
    const running = session.send('数学')
    session.stop()
    await running
    assert.equal(cancelled, true)
    assert.equal(session.state.turns[0].phase, 'stopped')
    assert.equal(session.state.turns[0].resources[0].id, 'file-1')
    assert.ok(unsubscribed)
    await session.retryLast()
    assert.equal(session.state.turns.length, 1)
    assert.equal(session.state.turns[0].content, '重试成功')
})

test('settings keep the key out of public state and provider changes clear conversation', async () => {
    let sentKey
    const session = createAssistantSession({
        call: async (method, _scope, input) => {
            if (method === 'getConfig') return { ...config }
            if (method === 'run') return answer('完成', input.history, input.question)
            if (method === 'save') { sentKey = input.apiKey; return { ...config, baseUrl: input.baseUrl } }
            if (method === 'clear') return { ...config, hasApiKey: false }
        }
    })
    await session.setIdentity(identity)
    await session.send('第一家服务的上下文')
    await session.saveSettings({ baseUrl: 'https://another.example/v1', model: 'another', apiKey: 'secret-test-key' }, true)
    assert.equal(sentKey, 'secret-test-key')
    assert.equal(JSON.stringify(session.state).includes('secret-test-key'), false)
    assert.equal(session.state.turns.length, 0)
    await session.clearKey()
    await assert.rejects(session.send('无密钥'), /API Key/)
})

test('failed connection test keeps saved settings and reports that distinction', async () => {
    const session = createAssistantSession({
        call: async (method) => {
            if (method === 'getConfig') return { ...config, hasApiKey: false }
            if (method === 'save') return { ...config }
            if (method === 'run') throw new Error('API Key 无效')
        }
    })
    await session.setIdentity(identity)
    await assert.rejects(session.saveSettings({ apiKey: 'invalid' }, true), /设置已保存，但连接测试失败/)
    assert.equal(session.state.config.hasApiKey, true)
    assert.equal(session.state.settingsBusy, false)
})

test('late settings reads and progress cannot overwrite a new identity or disposed session', async () => {
    const pending = deferred()
    const session = createAssistantSession({
        call: async (method, scope) => {
            if (method === 'getConfig') return scope.userId === '7' ? pending.promise : { ...config, model: 'account-8' }
        }
    })
    const first = session.setIdentity(identity)
    await Promise.resolve()
    await session.setIdentity({ ...identity, userId: '8' })
    pending.resolve(config)
    await first
    assert.equal(session.state.config.model, 'account-8')
    assert.equal(session.state.configLoading, false)
    session.dispose()
})
