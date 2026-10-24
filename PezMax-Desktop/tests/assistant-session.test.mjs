import test from 'node:test'
import assert from 'node:assert/strict'
import { createAssistantSession } from '../src/renderer/utils/assistant-session.mjs'

const config = {
    baseUrl: 'https://model.example/v1',
    model: 'tool-model',
    hasApiKey: true,
    remember: false,
    canRemember: true
}
const identity = { server: 'https://pezmax.example', userId: '7' }
const answer = (content) => ({ role: 'assistant', content })
const deferred = () => {
    let resolve, reject
    const promise = new Promise((a, b) => {
        resolve = a
        reject = b
    })
    return { promise, resolve, reject }
}
const search = async () => ({ resources: [], modelResult: { reports: [] } })

test('follow-up messages use completed history; new conversations drop it', async () => {
    const calls = []
    const session = createAssistantSession({
        search,
        call: async (method, _scope, input) => {
            if (method === 'getConfig') return { ...config }
            if (method === 'complete') {
                calls.push(structuredClone(input))
                return answer('找到了')
            }
        }
    })
    await session.setIdentity(identity)
    await session.send('高等数学')
    await session.send('只看2023年的')
    assert.deepEqual(
        calls[1].messages.map((item) => item.content),
        ['高等数学', '找到了', '只看2023年的']
    )
    assert.equal(session.state.turns.length, 2)
    session.newConversation()
    await session.send('大学物理')
    assert.deepEqual(
        calls[2].messages.map((item) => item.content),
        ['大学物理']
    )
})

test('logout cancels work, releases memory keys, and ignores a late response from the old account', async () => {
    const pending = deferred()
    const events = []
    const session = createAssistantSession({
        search,
        call: async (method, scope) => {
            events.push([method, scope])
            if (method === 'getConfig') return { ...config, hasApiKey: scope.userId === '7' }
            if (method === 'complete') return pending.promise
        }
    })
    await session.setIdentity(identity)
    const running = session.send('旧账号的问题')
    await session.setIdentity({ ...identity, userId: '8' })
    pending.resolve(answer('旧账号的私有回复'))
    await running
    assert.equal(session.state.turns.length, 0)
    assert.equal(session.state.config.hasApiKey, false)
    assert.equal(
        events.some(([method]) => method === 'cancel'),
        true
    )
    assert.equal(events.filter(([method]) => method === 'release').length, 2)
    await session.setIdentity(null)
    await assert.rejects(session.send('不能发送'), /登录/)
})

test('stop preserves real cards and retry does not duplicate failed user history', async () => {
    const pending = deferred()
    let completions = 0,
        cancelled = false
    const session = createAssistantSession({
        search: async () => ({
            resources: [{ id: 'file-1', title: '真实资料' }],
            modelResult: { resources: [{ id: 'file-1' }], reports: [] }
        }),
        call: async (method, _scope, input) => {
            if (method === 'getConfig') return { ...config }
            if (method === 'cancel') {
                cancelled = true
                pending.reject(new Error('已停止生成'))
                return
            }
            if (method === 'complete') {
                completions++
                if (completions === 1)
                    return {
                        ...answer(''),
                        tool_calls: [
                            {
                                id: 'c1',
                                type: 'function',
                                function: {
                                    name: 'search_resources',
                                    arguments: '{"kinds":["file"],"keyword":"数学"}'
                                }
                            }
                        ]
                    }
                if (completions === 2) return pending.promise
                assert.equal(input.messages.filter((message) => message.role === 'user').length, 1)
                return answer('重试成功')
            }
        }
    })
    await session.setIdentity(identity)
    const running = session.send('数学')
    for (let index = 0; index < 10 && completions < 2; index++) await Promise.resolve()
    session.stop()
    await running
    assert.equal(cancelled, true)
    assert.equal(session.state.turns[0].phase, 'stopped')
    assert.equal(session.state.turns[0].resources[0].id, 'file-1')
    await session.retryLast()
    assert.equal(session.state.turns.length, 1)
    assert.equal(session.state.turns[0].content, '重试成功')
})

test('saved settings never place the API Key into public session state; provider changes clear context', async () => {
    let sentKey
    const session = createAssistantSession({
        search,
        call: async (method, _scope, input) => {
            if (method === 'getConfig') return { ...config }
            if (method === 'complete') return answer('完成')
            if (method === 'save') {
                sentKey = input.apiKey
                return { ...config, baseUrl: input.baseUrl }
            }
            if (method === 'clear') return { ...config, hasApiKey: false }
        }
    })
    await session.setIdentity(identity)
    await session.send('第一家服务的上下文')
    await session.saveSettings(
        { baseUrl: 'https://another.example/v1', model: 'another', apiKey: 'secret-test-key' },
        true
    )
    assert.equal(sentKey, 'secret-test-key')
    assert.equal(JSON.stringify(session.state).includes('secret-test-key'), false)
    assert.equal(session.state.turns.length, 0)
    await session.clearKey()
    await assert.rejects(session.send('无密钥'), /API Key/)
})

test('a failed connection test keeps the successfully saved config and reports that distinction', async () => {
    const session = createAssistantSession({
        search,
        call: async (method) => {
            if (method === 'getConfig') return { ...config, hasApiKey: false }
            if (method === 'save') return { ...config }
            if (method === 'complete') throw new Error('API Key 无效')
        }
    })
    await session.setIdentity(identity)
    await assert.rejects(
        session.saveSettings({ apiKey: 'invalid' }, true),
        /设置已保存，但连接测试失败/
    )
    assert.equal(session.state.config.hasApiKey, true)
    assert.equal(session.state.settingsBusy, false)
})

test('a late settings read cannot overwrite the active identity and a disposed session ignores pending work', async () => {
    const pending = deferred()
    const session = createAssistantSession({
        search,
        call: async (method, scope) => {
            if (method === 'getConfig')
                return scope.userId === '7' ? pending.promise : { ...config, model: 'account-8' }
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
