import {
    normalizeKnowledgeInput,
    normalizeKnowledgeResult
} from '../../shared/knowledge-assistant.mjs'

const emptyConfig = () => ({
    baseUrl: '',
    model: '',
    hasApiKey: false,
    remember: false,
    canRemember: false
})
const requestId = () => `knowledge-${globalThis.crypto.randomUUID()}`
export function createKnowledgeState() {
    return {
        config: { ...emptyConfig(), embedding: emptyConfig() },
        configLoading: false,
        settingsBusy: false,
        busy: false,
        stopping: false,
        documents: [],
        library: 'local',
        coverage: null,
        sync: null,
        documentIds: [],
        mode: 'keyword',
        result: null,
        evidence: null,
        error: '',
        notice: '',
        status: ''
    }
}

export function createKnowledgeSession({
    call,
    library = 'local',
    platformToken = () => '',
    openResource = () => {},
    subscribe = () => () => {},
    state = createKnowledgeState()
}) {
    let context = null,
        epoch = 0,
        pending = null,
        draining = null
    function stop() {
        if (!pending || pending.cancelled) return
        pending.cancelled = true
        state.stopping = true
        void call('cancel', pending.context, pending.id).catch(() => {})
    }
    async function refreshConfig() {
        if (!context || state.busy || state.settingsBusy || state.configLoading) return
        const version = epoch
        state.configLoading = true
        try {
            const config = await call('getConfig', context)
            if (version === epoch) state.config = config
        } catch (error) {
            if (version === epoch) state.error = error.message || '无法读取模型配置'
        } finally {
            if (version === epoch) state.configLoading = false
        }
    }
    async function setIdentity(next) {
        stop()
        epoch++
        context = next ? { server: next.server, userId: next.userId } : null
        pending = null
        Object.assign(state, createKnowledgeState(), { library })
        const version = epoch
        if (!context) return
        await refreshConfig()
        if (version === epoch) await refresh(library === 'platform')
    }
    async function execute(value, apply) {
        if (!context || state.busy || state.settingsBusy || state.configLoading) return
        const version = epoch,
            scope = { ...context }
        state.busy = true
        state.error = state.notice = ''
        try {
            const request = normalizeKnowledgeInput({
                ...value,
                ...(state.library === 'platform' ? { library: 'platform' } : {})
            })
            if (draining) await draining
            if (version !== epoch) return
            const operation = { id: requestId(), context: scope, cancelled: false }
            pending = operation
            const unsubscribe = subscribe((event) => {
                if (
                    version === epoch &&
                    !operation.cancelled &&
                    event.requestId === operation.id &&
                    event.status
                )
                    state.status = event.status
            })
            try {
                const promise = call('run', scope, {
                    requestId: operation.id,
                    task: 'knowledge',
                    ...(state.library === 'platform' ? { platformToken: platformToken() } : {}),
                    knowledge: request
                })
                const barrier = promise
                    .catch(() => {})
                    .then(() => {
                        if (draining === barrier) draining = null
                    })
                draining = barrier
                const result = await promise
                if (version !== epoch) return
                if (operation.cancelled) throw new Error('已停止；导入或删除结果请刷新列表确认')
                const normalized = normalizeKnowledgeResult(result.knowledge, request)
                state.coverage = normalized.coverage || null
                apply(normalized)
            } finally {
                unsubscribe()
            }
        } catch (error) {
            if (version === epoch) {
                state.error = error.message || '知识库操作失败，请重试'
                if (state.library === 'platform') state.result = state.evidence = null
            }
        } finally {
            if (version === epoch) {
                pending = null
                state.busy = state.stopping = false
                state.status = ''
            }
        }
    }
    function refresh(sync = false, hybrid = false) {
        return execute(
            {
                action: state.library === 'platform' && sync ? 'sync' : 'list',
                mode: hybrid ? 'hybrid' : 'keyword'
            },
            (result) => {
                state.documents = result.documents
                if (result.sync) {
                    state.sync = result.sync
                    state.result = state.evidence = null
                    state.notice = `平台同步完成：更新 ${result.sync.indexed} 篇，复用 ${result.sync.unchanged} 篇，失败 ${result.sync.failures.length} 篇`
                }
                state.documentIds = state.documentIds.filter((id) =>
                    result.documents.some((doc) => doc.documentId === id)
                )
            }
        )
    }
    async function switchLibrary(next) {
        if (state.busy || state.settingsBusy || !['local', 'platform'].includes(next)) return
        state.library = next
        state.documents = state.documentIds = []
        state.result = state.evidence = state.coverage = state.sync = null
        await refresh(next === 'platform')
    }
    async function importDocument(hybrid = false) {
        await execute({ action: 'import', mode: hybrid ? 'hybrid' : 'keyword' }, (result) => {
            if (!result.cancelled) {
                state.notice = result.document.inserted
                    ? `已导入「${result.document.title}」`
                    : '此文档已在知识库中，无需重复导入'
                state.result = state.evidence = null
                state.documents = [
                    ...state.documents.filter((d) => d.documentId !== result.document.documentId),
                    result.document
                ]
            }
        })
    }
    async function deleteDocument(documentId) {
        await execute({ action: 'delete', documentId }, () => {
            state.documents = state.documents.filter((d) => d.documentId !== documentId)
            state.documentIds = state.documentIds.filter((id) => id !== documentId)
            state.result = state.evidence = null
            state.notice = '已从当前账号知识库移除文档及索引'
        })
    }
    function query(question, answer = false) {
        state.result = state.evidence = null
        return execute(
            {
                action: answer ? 'ask' : 'search',
                mode: state.mode,
                query: { question, documentIds: [...state.documentIds] }
            },
            (result) => {
                state.result = {
                    ...result,
                    question: question.trim(),
                    action: answer ? 'ask' : 'search'
                }
            }
        )
    }
    function readEvidence(citation) {
        return execute(
            { action: 'read', documentId: citation.documentId, chunkId: citation.chunkId },
            (result) => {
                if (
                    citation.location !== result.chunk.location ||
                    (citation.quote && !result.chunk.text.includes(citation.quote))
                )
                    throw new Error('文档引用已变化，请重新检索')
                state.evidence = { ...result.chunk, quote: citation.quote || '' }
            }
        )
    }
    function openEvidence(citation) {
        return execute(
            { action: 'open', documentId: citation.documentId, chunkId: citation.chunkId },
            (result) => {
                if (
                    citation.location !== result.chunk.location ||
                    (citation.quote && !result.chunk.text.includes(citation.quote))
                )
                    throw new Error('文档引用已变化，请重新检索')
                openResource(result.chunk)
            }
        )
    }
    function clearResults() {
        if (!state.busy) state.result = state.evidence = null
    }
    async function saveSettings(input, embedding = false) {
        if (!context || state.busy || state.configLoading || state.settingsBusy)
            throw new Error('请等待当前操作结束')
        const version = epoch
        state.settingsBusy = true
        try {
            const config = await call(embedding ? 'saveEmbedding' : 'save', context, input)
            if (version !== epoch) throw new Error('账号已变化，请重新打开模型设置')
            state.config = config
            clearResults()
            return '模型设置已保存'
        } finally {
            if (version === epoch) state.settingsBusy = false
        }
    }
    async function clearKey(embedding = false) {
        if (!context || state.busy || state.settingsBusy) throw new Error('请等待当前操作结束')
        const version = epoch
        state.settingsBusy = true
        try {
            const config = await call(embedding ? 'clearEmbedding' : 'clear', context)
            if (version !== epoch) throw new Error('账号已变化，请重新打开模型设置')
            state.config = config
            if (embedding) state.mode = 'keyword'
            clearResults()
        } finally {
            if (version === epoch) state.settingsBusy = false
        }
    }
    function dispose() {
        stop()
        epoch++
        context = null
        pending = null
    }
    // The enclosing AssistantWorkspace owns bridge release for this window.
    return {
        state,
        setIdentity,
        refreshConfig,
        refresh,
        switchLibrary,
        importDocument,
        deleteDocument,
        query,
        readEvidence,
        openEvidence,
        clearResults,
        saveSettings,
        clearKey,
        stop,
        dispose
    }
}
