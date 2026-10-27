const emptyConfig = () => ({
    baseUrl: '',
    model: '',
    hasApiKey: false,
    remember: false,
    canRemember: false
})
const requestId = () =>
    `assistant-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`}`

export function createAssistantState() {
    return {
        config: emptyConfig(),
        configLoading: false,
        configError: '',
        busy: false,
        stopping: false,
        settingsBusy: false,
        turns: []
    }
}

// The state may be Vue-reactive; credentials and provider history never enter it.
// An epoch rejects late responses after logout, account changes or unmounting.
export function createAssistantSession({
    call,
    subscribe = () => () => {},
    state = createAssistantState(),
    releaseOnDispose = true
}) {
    let context = null,
        epoch = 0,
        history = [],
        controller = null,
        activeRequest = '',
        removeProgress = null

    function abortWork() {
        controller?.abort()
        removeProgress?.()
        removeProgress = null
        if (context && activeRequest) void call('cancel', context, activeRequest).catch(() => {})
    }

    async function setIdentity(nextContext) {
        abortWork()
        const hadIdentity = !!context
        const version = ++epoch
        controller = null
        activeRequest = ''
        context = nextContext ? { ...nextContext } : null
        history = []
        Object.assign(state, createAssistantState(), { configLoading: !!context })
        try {
            // Another assistant surface may already own this account's session
            // key. Mounting a second surface must not discard it.
            if (hadIdentity) await call('release')
            if (version !== epoch || !context) return
            const config = await call('getConfig', context)
            if (version === epoch) state.config = config
        } catch (error) {
            if (version === epoch && context)
                state.configError = error.message || '无法读取模型设置'
        } finally {
            if (version === epoch) state.configLoading = false
        }
    }

    async function refreshConfig() {
        if (!context || state.busy || state.settingsBusy || state.configLoading) return
        const version = epoch
        state.configLoading = true
        try {
            const config = await call('getConfig', context)
            if (version !== epoch) return
            if (config.baseUrl !== state.config.baseUrl || config.model !== state.config.model) {
                history = []
                state.turns = []
            }
            state.config = config
            state.configError = ''
        } catch (error) {
            if (version === epoch) state.configError = error.message || '无法读取模型设置'
        } finally {
            if (version === epoch) state.configLoading = false
        }
    }

    function ensureIdle() {
        if (!context) throw new Error('请先登录再使用找资料助手')
        if (state.configLoading || state.busy || state.settingsBusy)
            throw new Error('请等待当前操作结束')
    }

    async function saveSettings(input, testConnection = false) {
        ensureIdle()
        const version = epoch,
            scope = { ...context }
        state.settingsBusy = true
        let saved = false
        try {
            const config = await call('save', scope, input)
            if (version !== epoch) throw new Error('账号已切换，请重新打开模型设置')
            if (config.baseUrl !== state.config.baseUrl || config.model !== state.config.model) {
                history = []
                state.turns = []
            }
            state.config = config
            state.configError = ''
            saved = true
            if (testConnection) {
                activeRequest = requestId()
                await call('run', scope, { requestId: activeRequest, test: true })
                if (version !== epoch) throw new Error('账号已切换，请重新打开模型设置')
            }
            return testConnection
                ? '设置已保存，连接测试通过。可以开始找资料了。'
                : '模型设置已保存。'
        } catch (error) {
            if (version !== epoch) throw new Error('账号已切换，请重新打开模型设置')
            throw new Error(
                `${saved && testConnection ? '设置已保存，但连接测试失败：' : ''}${error.message || '设置操作失败'}`
            )
        } finally {
            if (version === epoch) {
                activeRequest = ''
                state.settingsBusy = false
            }
        }
    }

    async function clearKey() {
        ensureIdle()
        const version = epoch
        state.settingsBusy = true
        try {
            const config = await call('clear', context)
            if (version === epoch) state.config = config
        } finally {
            if (version === epoch) state.settingsBusy = false
        }
    }

    async function send(question, retry = false) {
        ensureIdle()
        if (!state.config.hasApiKey) throw new Error('请先在模型设置中填写 API Key')
        const prompt = String(question || '').trim()
        if (!prompt || prompt.length > 2000) throw new Error('请输入 1 到 2000 字的需求')
        const version = epoch,
            scope = { ...context }
        const abort = new AbortController()
        controller = abort
        if (retry) state.turns.pop()
        state.turns.push({
            id: requestId(),
            question: prompt,
            content: '',
            resources: [],
            reports: [],
            phase: 'running',
            status: '',
            error: ''
        })
        if (state.turns.length > 20) state.turns.shift()
        const turn = state.turns.at(-1)
        state.busy = true
        state.stopping = false
        activeRequest = turn.id
        let unsubscribe = () => {}
        try {
            unsubscribe = subscribe((event) => {
                if (version !== epoch || abort.signal.aborted || event.requestId !== turn.id) return
                if (event.status) turn.status = event.status
                if (event.resources) turn.resources = event.resources
                if (event.report) turn.reports.push(...event.report)
            })
            removeProgress = unsubscribe
            const result = await call('run', scope, {
                requestId: activeRequest,
                history,
                question: prompt,
                platformToken: scope.token
            })
            if (version !== epoch) return
            if (abort.signal.aborted) throw new Error('已停止生成')
            history = result.messages
            turn.content = result.content
            turn.resources = result.resources
            turn.reports = result.reports
            turn.phase = 'done'
        } catch (error) {
            if (version !== epoch) return
            turn.phase = abort.signal.aborted ? 'stopped' : 'error'
            turn.error = abort.signal.aborted
                ? '已停止生成，已找到的资源仍可打开。'
                : error.message || '本次请求失败，请重试'
        } finally {
            unsubscribe()
            if (version === epoch) {
                state.busy = false
                state.stopping = false
                controller = null
                activeRequest = ''
                removeProgress = null
            }
        }
    }

    function stop() {
        if (!state.busy) return
        state.stopping = true
        abortWork()
    }

    function retryLast() {
        const last = state.turns.at(-1)
        if (!last || !['error', 'stopped'].includes(last.phase)) return
        return send(last.question, true)
    }

    function newConversation() {
        if (state.busy || state.settingsBusy) return
        history = []
        state.turns = []
    }

    function dispose() {
        abortWork()
        epoch++
        context = null
        history = []
        controller = null
        activeRequest = ''
        if (releaseOnDispose) void call('release').catch(() => {})
    }

    return {
        state,
        setIdentity,
        refreshConfig,
        saveSettings,
        clearKey,
        send,
        stop,
        retryLast,
        newConversation,
        dispose
    }
}
