import { createAssistantSession, createAssistantState } from './assistant-session.mjs'
import { normalizeUploadInput, normalizeUploadSuggestions } from '../../shared/upload-assistant.mjs'

export function createUploadAssistantState() {
    return { ...createAssistantState(), suggestions: [], status: '', error: '', feedback: '' }
}

// No upload APIs or filesystem handles enter this controller. Applying a result
// is a separate, explicit operation on the existing form.
export function createUploadAssistantSession({
    call,
    subscribe = () => () => {},
    state = createUploadAssistantState()
}) {
    const settings = createAssistantSession({ call, state, releaseOnDispose: false })
    let context = null,
        epoch = 0,
        pending = null,
        draining = null,
        snapshot = null
    function reset() {
        epoch++
        if (pending) {
            pending.unsubscribe()
            void call('cancel', pending.context, pending.id).catch(() => {})
        }
        pending = null
        snapshot = null
        Object.assign(state, { busy: false, suggestions: [], status: '', error: '', feedback: '' })
    }
    async function setIdentity(next) {
        reset()
        context = next ? { ...next } : null
        await settings.setIdentity(next)
    }
    async function generate(value, form) {
        if (!context || state.configLoading || state.settingsBusy || state.busy) return
        reset()
        if (!state.config.hasApiKey) {
            state.error = '请先配置模型与 API Key'
            return
        }
        const version = epoch
        try {
            const upload = normalizeUploadInput(value)
            snapshot = { ...form }
            state.busy = true
            // A cancelled process can take a moment to exit. Queue this retry
            // until its run promise settles, respecting the one-worker owner.
            if (draining) await draining
            if (epoch !== version) return
            const id = `upload-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`}`
            pending = { id, context: { ...context }, unsubscribe: () => {} }
            state.busy = true
            pending.unsubscribe = subscribe((event) => {
                if (epoch === version && event.requestId === id && event.status)
                    state.status = event.status
            })
            // Deliberately omit platform token, form values, paths and search history.
            const operation = call('run', context, { requestId: id, task: 'upload', upload })
            const barrier = operation
                .catch(() => {})
                .then(() => {
                    if (draining === barrier) draining = null
                })
            draining = barrier
            const result = await operation
            if (epoch !== version) return
            const normalized = normalizeUploadSuggestions(result.suggestions, upload)
            state.suggestions = normalized
                .filter((item) => form[item.field] !== item.value)
                .map((item) => ({ ...item, selected: false }))
            state.feedback = state.suggestions.length
                ? '勾选需要回填的字段，再点击应用。'
                : normalized.length
                  ? '建议与当前表单一致，无需修改。'
                  : '文件名信息不足，未生成可靠建议，请手动填写。'
        } catch (error) {
            if (epoch === version) state.error = error.message || '生成失败，请重试'
        } finally {
            if (epoch === version) {
                pending?.unsubscribe()
                pending = null
                state.busy = false
                state.status = ''
            }
        }
    }
    function stop() {
        reset()
        state.feedback = '已停止生成，可重新生成建议。'
    }
    function apply(form) {
        if (state.busy || state.settingsBusy || !snapshot) return
        let applied = 0,
            skipped = 0
        for (const item of state.suggestions) {
            if (!item.selected) continue
            if (form[item.field] !== snapshot[item.field]) {
                skipped++
                continue
            }
            form[item.field] = item.value
            snapshot[item.field] = item.value
            item.selected = false
            applied++
        }
        state.feedback = `已应用 ${applied} 项建议。${skipped ? `${skipped} 项在生成后已被修改，已跳过；请重新生成后再确认。` : '请检查表单后自行提交上传。'}`
    }
    function dispose() {
        reset()
        settings.dispose()
        context = null
    }
    return {
        state,
        setIdentity,
        generate,
        stop,
        reset,
        apply,
        dispose,
        saveSettings: async (...args) => {
            reset()
            if (draining) await draining
            return settings.saveSettings(...args)
        },
        clearKey: async () => {
            reset()
            if (draining) await draining
            return settings.clearKey()
        },
        refreshConfig: settings.refreshConfig
    }
}
