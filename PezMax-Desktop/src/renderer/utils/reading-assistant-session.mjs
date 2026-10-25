import { createAssistantSession, createAssistantState } from './assistant-session.mjs'
import { normalizeReadingInput, normalizeReadingResult } from '../../shared/reading-assistant.mjs'
import { loadPdfText } from './pdf-text.mjs'

export function createReadingAssistantState() {
    return {
        ...createAssistantState(),
        extracting: false,
        from: 1,
        to: 1,
        numPages: 0,
        pages: [],
        result: null,
        question: '',
        status: '',
        error: ''
    }
}

// Document URLs stay in this renderer controller. Only selected text and the
// question enter IPC; no reading history, platform credentials or file handles.
export function createReadingAssistantSession({
    call,
    subscribe = () => () => {},
    loadPdf = loadPdfText,
    state = createReadingAssistantState(),
    extractionTimeoutMs = 45000
}) {
    const settings = createAssistantSession({ call, state, releaseOnDispose: false })
    let context = null,
        document = null,
        epoch = 0,
        extraction = null,
        pending = null,
        draining = null

    function cancelWork() {
        epoch++
        extraction?.abort()
        extraction = null
        if (pending) {
            pending.unsubscribe()
            void call('cancel', pending.context, pending.id).catch(() => {})
        }
        pending = null
        Object.assign(state, {
            busy: false,
            extracting: false,
            result: null,
            question: '',
            status: '',
            error: ''
        })
    }
    function clear() {
        cancelWork()
        Object.assign(state, { pages: [], numPages: 0, from: 1, to: 1 })
    }
    async function setIdentity(next) {
        clear()
        context = next ? { server: next.server, userId: next.userId } : null
        await settings.setIdentity(context)
    }
    function setDocument(next) {
        clear()
        document = next ? { id: next.id, source: next.source } : null
    }
    function setRange(from, to) {
        if (from === state.from && to === state.to) return
        cancelWork()
        Object.assign(state, { from, to, pages: [] })
    }
    async function extract() {
        if (!context || !document || state.extracting || state.busy || state.settingsBusy) return
        cancelWork()
        state.pages = []
        const version = epoch,
            abort = new AbortController(),
            source = document.source
        let pdf,
            timedOut = false
        const timer = setTimeout(() => {
            timedOut = true
            abort.abort()
        }, extractionTimeoutMs)
        extraction = abort
        state.extracting = true
        state.status = '正在本机提取所选页面…'
        try {
            if (
                !Number.isInteger(state.from) ||
                !Number.isInteger(state.to) ||
                state.from < 1 ||
                state.to < state.from ||
                state.to - state.from >= 10
            )
                throw new Error('请选择连续1到10页，起始页不大于结束页')
            pdf = await loadPdf(source, { signal: abort.signal })
            if (version !== epoch) return
            state.numPages = pdf.numPages
            const pages = await pdf.pages(state.from, state.to)
            if (version !== epoch) return
            abort.signal.throwIfAborted()
            state.pages = pages
        } catch (error) {
            if (version === epoch)
                state.error = timedOut
                    ? 'PDF读取超时，请重新打开资料后重试'
                    : error.message || 'PDF文字提取失败'
        } finally {
            clearTimeout(timer)
            await pdf?.destroy().catch(() => {})
            if (version === epoch) {
                state.extracting = false
                state.status = ''
                extraction = null
            }
        }
    }
    async function ask(question) {
        if (
            !context ||
            !document ||
            state.busy ||
            state.extracting ||
            state.settingsBusy ||
            state.configLoading
        )
            return
        cancelWork()
        const version = epoch
        try {
            if (!state.config.hasApiKey) throw new Error('请先配置模型与 API Key')
            const reading = normalizeReadingInput({ question, pages: state.pages })
            state.busy = true
            state.question = reading.question
            if (draining) await draining
            if (version !== epoch) return
            const id = `reading-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`}`
            const scope = { ...context }
            pending = { id, context: scope, unsubscribe: () => {} }
            pending.unsubscribe = subscribe((event) => {
                if (version === epoch && event.requestId === id && event.status)
                    state.status = event.status
            })
            const operation = call('run', scope, { requestId: id, task: 'reading', reading })
            const barrier = operation
                .catch(() => {})
                .then(() => {
                    if (draining === barrier) draining = null
                })
            draining = barrier
            const result = await operation
            if (version !== epoch) return
            state.result = normalizeReadingResult(result, reading)
        } catch (error) {
            if (version === epoch) state.error = error.message || '阅读问答失败，请重试'
        } finally {
            if (version === epoch) {
                pending?.unsubscribe()
                pending = null
                state.busy = false
                state.status = ''
            }
        }
    }
    function stop() {
        cancelWork()
        state.status = '已停止，可重试。'
    }
    function dispose() {
        clear()
        settings.dispose()
        context = document = null
    }
    return {
        state,
        setIdentity,
        setDocument,
        setRange,
        extract,
        ask,
        stop,
        clear,
        dispose,
        refreshConfig: settings.refreshConfig,
        saveSettings: async (...args) => {
            cancelWork()
            const version = epoch
            if (draining) await draining
            if (version !== epoch) throw new Error('账号或资料已变化，请重新打开模型设置')
            return settings.saveSettings(...args)
        },
        clearKey: async () => {
            cancelWork()
            const version = epoch
            if (draining) await draining
            if (version !== epoch) throw new Error('账号或资料已变化，请重新打开模型设置')
            return settings.clearKey()
        }
    }
}
