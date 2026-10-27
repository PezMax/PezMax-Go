<template>
    <section class="knowledge-workspace" aria-label="我的知识库">
        <header class="knowledge-header">
            <div class="heading">
                <AButton aria-label="返回找资料助手" @click="emit('back')"
                    ><ArrowLeftOutlined
                /></AButton>
                <div>
                    <h1>我的知识库</h1>
                    <p>从平台已有资料寻找答案，点击引用回到原文</p>
                </div>
            </div>
            <div class="header-actions">
                <AButton :disabled="locked || !signedIn" @click="openSettings(true)"
                    >Embedding 设置</AButton
                >
                <AButton :disabled="locked || !signedIn" @click="openSettings(false)"
                    ><SettingOutlined />模型设置</AButton
                >
            </div>
        </header>
        <div class="knowledge-layout">
            <aside class="document-pane" aria-label="已导入文档">
                <ASelect
                    :value="state.library"
                    aria-label="知识来源"
                    :disabled="locked"
                    :options="[
                        { value: 'platform', label: '平台资料与电子书' },
                        { value: 'local', label: '个人本机文档' }
                    ]"
                    style="width: 100%; margin-bottom: 12px"
                    @change="session.switchLibrary($event)"
                />
                <div class="pane-heading">
                    <strong>{{ isPlatform ? '平台正文索引' : '本机文档' }}</strong
                    ><ATag>{{ state.documents.length }} / 200</ATag>
                    <AButton
                        type="text"
                        size="small"
                        aria-label="刷新文档列表"
                        :disabled="locked || !signedIn"
                        @click="session.refresh()"
                        ><ReloadOutlined
                    /></AButton>
                </div>
                <AButton type="primary" block :disabled="locked || !signedIn" @click="openImport"
                    ><PlusOutlined />{{ isPlatform ? '同步平台资料' : '导入文档' }}</AButton
                >
                <p v-if="isPlatform && state.coverage" class="evidence-note">
                    可索引 {{ state.coverage.total }} 篇，当前范围
                    {{ state.coverage.available }} 篇。
                    {{
                        state.coverage.partial
                            ? '当前仅覆盖最近更新的200篇，未检索平台全部资料。'
                            : '仅含已上架、16 MiB以内的受支持文件。'
                    }}
                </p>
                <div class="document-list">
                    <AEmpty
                        v-if="!state.documents.length && !state.busy"
                        :description="isPlatform ? '暂无已建立索引的平台资料' : '还没有导入文档'"
                        :image="AEmpty.PRESENTED_IMAGE_SIMPLE"
                    />
                    <article
                        v-for="doc in state.documents"
                        :key="doc.documentId"
                        class="document-row"
                    >
                        <FileTextOutlined class="document-icon" />
                        <div class="document-info">
                            <h2>{{ doc.title }}</h2>
                            <p>
                                {{ doc.format.toUpperCase() }} · {{ doc.chunks }} 片段 ·
                                {{ doc.characters.toLocaleString() }} 字
                            </p>
                            <ATag>{{
                                doc.dimension ? `向量 ${doc.dimension} 维` : '关键词索引'
                            }}</ATag>
                            <p v-if="doc.source">
                                {{
                                    [doc.source.subject, doc.source.school, doc.source.year || '']
                                        .filter(Boolean)
                                        .join(' · ')
                                }}
                            </p>
                        </div>
                        <APopconfirm
                            v-if="!isPlatform"
                            title="移除此文档和索引？原文件会保留。"
                            ok-text="移除"
                            cancel-text="取消"
                            :disabled="locked"
                            @confirm="session.deleteDocument(doc.documentId)"
                        >
                            <AButton
                                type="text"
                                danger
                                size="small"
                                :disabled="locked"
                                :aria-label="`移除文档 ${doc.title}`"
                                ><DeleteOutlined
                            /></AButton>
                        </APopconfirm>
                    </article>
                </div>
                <p class="storage-note">
                    <LockOutlined />{{
                        isPlatform
                            ? '从平台自动获取正文，索引缓存在本机；检索前核对上架状态与版本。'
                            : '按平台与账号隔离，正文与索引以明文保存在本机。'
                    }}
                </p>
            </aside>
            <main class="query-pane">
                <div class="query-scroll">
                    <AAlert
                        v-if="state.error"
                        type="error"
                        :message="state.error"
                        show-icon
                        role="alert"
                    />
                    <AAlert
                        v-if="state.notice"
                        type="success"
                        :message="state.notice"
                        show-icon
                        role="status"
                    />
                    <AAlert
                        v-if="state.sync?.failures.length"
                        type="warning"
                        show-icon
                        message="部分资料未能建立索引"
                        :description="state.sync.failures.map((item) => item.title).join('、')"
                    />
                    <div v-if="!state.result" class="knowledge-welcome">
                        <DatabaseOutlined class="welcome-icon" /><span class="eyebrow"
                            >你的资料，成为可查的知识</span
                        >
                        <h2>先找证据，再给答案</h2>
                        <p>
                            {{
                                isPlatform
                                    ? '直接检索平台已有资料与电子书，无需下载后重新导入。'
                                    : '导入 PDF、EPUB 或学习笔记，跨文档查找原文。'
                            }}<br />需要解释时，用你配置的模型生成带引用的回答。
                        </p>
                        <div class="welcome-steps">
                            <span>01 {{ isPlatform ? '同步平台' : '导入文档' }}</span
                            ><span>02 检索片段</span><span>03 核对引用</span>
                        </div>
                    </div>
                    <template v-else>
                        <div class="result-heading">
                            <span>{{
                                state.result.action === 'ask' ? '知识库问答' : '文档检索'
                            }}</span
                            ><ATag>{{
                                state.result.retrieval.mode === 'hybrid' ? '混合检索' : '关键词检索'
                            }}</ATag>
                        </div>
                        <h2 class="result-question">{{ state.result.question }}</h2>
                        <div
                            v-if="state.result.action === 'ask'"
                            class="answers"
                            aria-live="polite"
                        >
                            <AAlert
                                v-if="!state.result.answers.length"
                                type="info"
                                :message="state.result.content"
                                show-icon
                            />
                            <article
                                v-for="(answer, index) in state.result.answers"
                                :key="index"
                                class="answer"
                            >
                                <p>{{ answer.text }}</p>
                                <div class="citation-links">
                                    <AButton
                                        v-for="(citation, n) in answer.citations"
                                        :key="n"
                                        size="small"
                                        :disabled="locked"
                                        @click="session.readEvidence(citation)"
                                    >
                                        <FileSearchOutlined />{{ citationLabel(citation) }}
                                    </AButton>
                                </div>
                            </article>
                            <p class="evidence-note">
                                引用位置与连续原文已核对；回答是否正确，仍需结合原文判断。
                            </p>
                        </div>
                        <div class="evidence-heading">
                            <strong>检索到的原文</strong
                            ><span
                                >{{ state.result.retrieval.chunks.length }} /
                                {{ state.result.retrieval.scannedChunks }} 片段</span
                            >
                        </div>
                        <AEmpty
                            v-if="!state.result.retrieval.chunks.length"
                            description="没有匹配片段，试试原文中的关键词或导入相关文档。"
                            :image="AEmpty.PRESENTED_IMAGE_SIMPLE"
                        />
                        <div class="evidence-grid">
                            <ACard
                                v-for="chunk in state.result.retrieval.chunks"
                                :key="chunk.chunkId"
                                size="small"
                                class="evidence-card"
                            >
                                <div class="evidence-card-heading">
                                    <strong>{{ chunk.title }}</strong
                                    ><ATag>{{ knowledgeLocation(chunk) }}</ATag>
                                </div>
                                <p>{{ chunk.text }}</p>
                                <AButton
                                    type="link"
                                    size="small"
                                    :disabled="locked"
                                    @click="session.readEvidence(chunk)"
                                    >查看索引原文<ArrowRightOutlined
                                /></AButton>
                                <AButton
                                    v-if="chunk.source"
                                    type="link"
                                    size="small"
                                    :disabled="locked"
                                    @click="session.openEvidence(chunk)"
                                    >打开应用内原文<ArrowRightOutlined
                                /></AButton>
                            </ACard>
                        </div>
                        <p class="evidence-note">
                            仅显示本次检索选中的最多 6 个片段，不代表覆盖整篇或整个知识库。
                        </p>
                    </template>
                </div>
                <footer class="query-composer">
                    <div class="query-options">
                        <ASelect
                            v-model:value="state.documentIds"
                            mode="multiple"
                            placeholder="全部文档（可筛选，最多50篇）"
                            aria-label="选择检索文档"
                            option-filter-prop="label"
                            :options="documentOptions"
                            :max-tag-count="1"
                            :disabled="locked"
                            @change="session.clearResults()"
                        />
                        <ASelect
                            v-model:value="state.mode"
                            aria-label="检索模式"
                            :disabled="locked"
                            :options="modeOptions"
                            @change="session.clearResults()"
                        />
                    </div>
                    <ATextarea
                        v-model:value="question"
                        placeholder="例如：这些笔记如何解释队列的先进先出？"
                        aria-label="知识库问题"
                        :maxlength="2000"
                        :auto-size="{ minRows: 2, maxRows: 5 }"
                        :disabled="locked || !signedIn"
                    />
                    <div class="query-actions">
                        <span v-if="state.busy" role="status" aria-live="polite"
                            ><LoadingOutlined spin />{{
                                state.stopping ? '正在停止…' : state.status || '正在处理…'
                            }}</span
                        >
                        <span v-else>{{
                            state.mode === 'hybrid'
                                ? '问题将发送至你的向量服务'
                                : '关键词检索在本机完成'
                        }}</span>
                        <div>
                            <AButton
                                v-if="state.busy"
                                :disabled="state.stopping"
                                @click="session.stop()"
                                ><StopOutlined />停止</AButton
                            >
                            <template v-else
                                ><AButton :disabled="!canQuery" @click="submit(false)"
                                    ><SearchOutlined />仅检索</AButton
                                ><AButton type="primary" :disabled="!canQuery" @click="submit(true)"
                                    >检索并回答<ArrowRightOutlined /></AButton
                            ></template>
                        </div>
                    </div>
                    <p class="query-privacy">
                        问答会把问题与检索片段发送至你配置的模型服务，并可能产生费用。仅检索不会调用对话模型。
                    </p>
                </footer>
            </main>
        </div>
        <AModal
            v-model:open="importOpen"
            :title="isPlatform ? '同步平台资料索引' : '导入到本机知识库'"
            :footer="null"
            :mask-closable="false"
        >
            <p class="import-description">
                {{
                    isPlatform
                        ? '自动读取平台已上架的资料和电子书。支持 PDF、EPUB、TXT、Markdown，保留原文件关联。'
                        : '选择本机 PDF、EPUB、TXT 或 Markdown，提取文字并保留页码或分节位置。'
                }}单文件不超过 16 MiB、1000 个位置、200 万字符，暂不支持 OCR。
            </p>
            <AAlert
                type="info"
                message="默认仅在本机建立关键词索引，不发送文档给模型。正文与索引以明文保存在本机。"
                show-icon
            />
            <ACheckbox
                v-model:checked="importHybrid"
                :disabled="!state.config.embedding?.hasApiKey"
                class="embedding-consent"
                >同时建立向量索引（需发送文档文字至向量服务，可能计费）</ACheckbox
            >
            <p v-if="!state.config.embedding?.hasApiKey" class="evidence-note">
                如需混合检索，请先配置 Embedding；关键词索引可直接使用。
            </p>
            <div class="modal-actions">
                <AButton @click="importOpen = false">取消</AButton
                ><AButton type="primary" @click="chooseAndImport">{{
                    isPlatform ? '开始同步' : '选择文件并导入'
                }}</AButton>
            </div>
        </AModal>
        <AssistantSettings
            v-model:open="settingsOpen"
            :embedding="embeddingSettings"
            :allow-test="false"
            :config="settingsConfig"
            :busy="state.settingsBusy"
            :save-settings="saveSettings"
            :clear-key="clearKey"
        />
        <ADrawer
            :open="!!state.evidence"
            :title="
                state.evidence
                    ? `${state.evidence.title} · ${knowledgeLocation(state.evidence)}`
                    : ''
            "
            :width="560"
            @close="state.evidence = null"
        >
            <template v-if="state.evidence"
                ><ATag>{{ state.evidence.format.toUpperCase() }} 索引原文</ATag>
                <p class="source-text">
                    {{ sourceParts.before
                    }}<mark v-if="sourceParts.quote">{{ sourceParts.quote }}</mark
                    >{{ sourceParts.after }}
                </p>
                <AButton
                    v-if="state.evidence.source"
                    type="primary"
                    :disabled="locked"
                    @click="session.openEvidence(state.evidence)"
                    >打开应用内原文<ArrowRightOutlined
                /></AButton>
                <p class="evidence-note">
                    显示导入时保存的文字片段；PDF 为物理页码，EPUB
                    为书脊分节序号，文本为导入分段序号。
                </p>
            </template>
        </ADrawer>
    </section>
</template>

<script setup>
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import {
    Alert as AAlert,
    Button as AButton,
    Card as ACard,
    Checkbox as ACheckbox,
    Drawer as ADrawer,
    Empty as AEmpty,
    Modal as AModal,
    Popconfirm as APopconfirm,
    Select as ASelect,
    Tag as ATag,
    Textarea as ATextarea
} from 'ant-design-vue'
import {
    ArrowLeftOutlined,
    ArrowRightOutlined,
    DatabaseOutlined,
    DeleteOutlined,
    FileSearchOutlined,
    FileTextOutlined,
    LoadingOutlined,
    LockOutlined,
    PlusOutlined,
    ReloadOutlined,
    SearchOutlined,
    SettingOutlined,
    StopOutlined
} from '@ant-design/icons-vue'
import useUserStore from '@/store/modules/user'
import { getToken } from '@/utils/auth'
import { assistantCall, onAssistantProgress } from '@/api/assistant'
import {
    createKnowledgeSession,
    createKnowledgeState
} from '@/utils/knowledge-assistant-session.mjs'
import { knowledgeLocation, platformKnowledgeNode } from '../../../shared/knowledge-assistant.mjs'
import AssistantSettings from './AssistantSettings.vue'

const emit = defineEmits(['back', 'open-resource'])
const userStore = useUserStore()
const state = reactive(createKnowledgeState())
const session = createKnowledgeSession({
    call: assistantCall,
    library: 'platform',
    platformToken: getToken,
    openResource: (chunk) =>
        emit(
            'open-resource',
            platformKnowledgeNode(
                chunk.source,
                /^https?:\/\//i.test(import.meta.env.VITE_APP_BASE_API || '')
                    ? import.meta.env.VITE_APP_BASE_API
                    : import.meta.env.VITE_APP_TARGET_URL || 'http://127.0.0.1:9033',
                chunk.location
            )
        ),
    subscribe: onAssistantProgress,
    state
})
const question = ref(''),
    importOpen = ref(false),
    importHybrid = ref(false),
    settingsOpen = ref(false),
    embeddingSettings = ref(false)
const signedIn = computed(() => !!userStore.token && !!userStore.id)
const isPlatform = computed(() => state.library === 'platform')
const locked = computed(() => state.busy || state.settingsBusy || state.configLoading)
const canQuery = computed(
    () =>
        signedIn.value &&
        !locked.value &&
        state.documents.length &&
        question.value.trim() &&
        state.documentIds.length <= 50
)
const settingsConfig = computed(() =>
    embeddingSettings.value ? state.config.embedding || {} : state.config
)
const documentOptions = computed(() =>
    state.documents.map((doc) => ({
        value: doc.documentId,
        label: doc.source
            ? [doc.title, doc.source.subject, doc.source.school, doc.source.year || '']
                  .filter(Boolean)
                  .join(' · ')
            : doc.title,
        disabled: state.documentIds.length >= 50 && !state.documentIds.includes(doc.documentId)
    }))
)
const modeOptions = computed(() => [
    { value: 'keyword', label: '关键词 · 本地' },
    { value: 'hybrid', label: '混合 · 向量服务', disabled: !state.config.embedding?.hasApiKey }
])
const sourceParts = computed(() => {
    const text = state.evidence?.text || '',
        quote = state.evidence?.quote || ''
    const index = quote ? text.indexOf(quote) : -1
    return index < 0
        ? { before: text, quote: '', after: '' }
        : { before: text.slice(0, index), quote, after: text.slice(index + quote.length) }
})
const citationLabel = (citation) => {
    const chunk = state.result?.retrieval.chunks.find(
        (item) => item.chunkId === citation.chunkId && item.documentId === citation.documentId
    )
    return chunk ? `${chunk.title} · ${knowledgeLocation(chunk)}` : '查看引用'
}
function openSettings(embedding) {
    embeddingSettings.value = embedding
    settingsOpen.value = true
}
function openImport() {
    importHybrid.value = false
    importOpen.value = true
}
function chooseAndImport() {
    importOpen.value = false
    if (isPlatform.value) void session.refresh(true, importHybrid.value)
    else void session.importDocument(importHybrid.value)
}
function submit(answer) {
    if (!canQuery.value) return
    if (answer && !state.config.hasApiKey) {
        openSettings(false)
        return
    }
    void session.query(question.value, answer)
}
const saveSettings = (input) => session.saveSettings(input, embeddingSettings.value)
const clearKey = () => session.clearKey(embeddingSettings.value)
watch(
    () => [userStore.token, String(userStore.id || '')],
    () => {
        question.value = ''
        settingsOpen.value = importOpen.value = false
        void session.setIdentity(
            signedIn.value
                ? {
                      server:
                          import.meta.env.VITE_APP_TARGET_URL ||
                          import.meta.env.VITE_APP_BASE_API ||
                          'pezmax',
                      userId: String(userStore.id)
                  }
                : null
        )
    },
    { immediate: true, flush: 'sync' }
)
onUnmounted(() => session.dispose())
</script>

<style scoped>
.knowledge-workspace {
    display: flex;
    flex: 1;
    flex-direction: column;
    width: 100%;
    height: 100%;
    min-height: 0;
    min-width: 0;
    border: 1px solid var(--ide-border);
    border-radius: 8px;
    background: var(--ide-editor-bg);
    color: var(--ide-text);
    overflow: hidden;
}
.knowledge-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 18px 24px;
    border-bottom: 1px solid var(--ide-border);
}
.heading,
.header-actions {
    display: flex;
    align-items: center;
    gap: 10px;
}
.heading h1 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--ide-text-active);
}
.heading p {
    margin: 4px 0 0;
    font-size: 12px;
    color: var(--ide-text-light);
}
.knowledge-layout {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 280px minmax(0, 1fr);
}
.document-pane {
    padding: 20px 16px;
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-right: 1px solid var(--ide-border);
}
.pane-heading {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
    color: var(--ide-text-active);
}
.pane-heading > :last-child {
    margin-left: auto;
}
.document-list {
    flex: 1;
    overflow: auto;
    min-height: 80px;
    margin-top: 16px;
}
.document-row {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    padding: 16px 0;
    border-bottom: 1px solid var(--ide-border);
}
.document-icon {
    color: var(--ide-accent);
    margin-top: 4px;
}
.document-info {
    flex: 1;
    min-width: 0;
}
.document-info h2 {
    font-size: 13px;
    line-height: 1.6;
    overflow-wrap: anywhere;
    margin: 0 0 6px;
    color: var(--ide-text-active);
}
.document-info p,
.storage-note {
    font-size: 11px;
    color: var(--ide-text-light);
    line-height: 1.7;
}
.document-info p {
    margin: 0 0 6px;
}
.storage-note {
    margin: 16px 0 0;
}
.query-pane {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
}
.query-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 24px 28px;
}
.query-scroll > :deep(.ant-alert) {
    margin-bottom: 16px;
}
.knowledge-welcome {
    max-width: 600px;
    margin: 32px auto;
    padding: 24px 0;
}
.welcome-icon {
    display: block;
    font-size: 32px;
    color: var(--ide-accent);
    margin-bottom: 24px;
}
.eyebrow {
    font-size: 12px;
    color: var(--ide-accent);
}
.knowledge-welcome h2 {
    margin: 12px 0 16px;
    font-size: clamp(22px, 2.4vw, 30px);
    font-weight: 600;
    color: var(--ide-text-active);
}
.knowledge-welcome p {
    font-size: 14px;
    line-height: 1.9;
    color: var(--ide-text);
    text-wrap: pretty;
}
.welcome-steps {
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
    border-top: 1px solid var(--ide-border);
    padding-top: 20px;
    margin-top: 28px;
    font-size: 12px;
    color: var(--ide-text-light);
}
.result-heading,
.evidence-heading {
    display: flex;
    gap: 12px;
    align-items: center;
    font-size: 12px;
    color: var(--ide-text-light);
}
.result-question {
    font-size: 20px;
    line-height: 1.7;
    color: var(--ide-text-active);
    margin: 12px 0 24px;
    overflow-wrap: anywhere;
}
.answer {
    margin-bottom: 24px;
}
.answer p {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-size: 14px;
    line-height: 1.9;
    color: var(--ide-text-active);
}
.citation-links {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
}
.citation-links :deep(.ant-btn) {
    max-width: 100%;
    height: auto;
    white-space: normal;
    text-align: left;
}
.evidence-heading {
    justify-content: space-between;
    border-top: 1px solid var(--ide-border);
    padding-top: 24px;
    margin: 24px 0 12px;
}
.evidence-heading strong {
    color: var(--ide-text-active);
}
.evidence-grid {
    display: grid;
    gap: 12px;
}
.evidence-card-heading {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    justify-content: space-between;
}
.evidence-card-heading strong {
    font-size: 13px;
    overflow-wrap: anywhere;
}
.evidence-card-heading :deep(.ant-tag) {
    flex-shrink: 0;
}
.evidence-card p {
    font-size: 13px;
    line-height: 1.8;
    color: var(--ide-text);
    overflow-wrap: anywhere;
    white-space: pre-wrap;
    max-height: 7.2em;
    overflow-y: auto;
}
.evidence-note,
.query-privacy {
    font-size: 11px;
    line-height: 1.7;
    color: var(--ide-text-light);
}
.query-composer {
    border-top: 1px solid var(--ide-border);
    padding: 16px 28px;
}
.query-options {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 170px;
    gap: 8px;
    margin-bottom: 10px;
}
.query-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    margin-top: 12px;
    font-size: 11px;
    color: var(--ide-text-light);
}
.query-actions > div {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
}
.query-actions > span {
    display: flex;
    align-items: center;
    gap: 6px;
}
.query-privacy {
    margin-bottom: 0;
}
.import-description {
    color: var(--ide-text);
    line-height: 1.8;
}
.embedding-consent {
    margin: 20px 0 12px;
    line-height: 1.8;
}
.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 24px;
}
.source-text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-size: 14px;
    line-height: 2;
    color: var(--ide-text-active);
    margin-top: 20px;
}
.source-text mark {
    color: inherit;
    background: var(--ide-accent-light);
    border-bottom: 1px solid var(--ide-accent);
}
@media (max-width: 1050px) {
    .knowledge-header {
        padding: 16px;
    }
    .heading p {
        display: none;
    }
    .knowledge-layout {
        grid-template-columns: 230px minmax(0, 1fr);
    }
    .query-scroll {
        padding: 20px;
    }
    .query-composer {
        padding: 16px 20px;
    }
    .query-actions {
        flex-wrap: wrap;
    }
}
@media (max-width: 760px) {
    .knowledge-header {
        flex-wrap: wrap;
    }
    .header-actions {
        margin-left: 42px;
    }
    .knowledge-layout {
        display: flex;
        flex-direction: column;
        overflow: auto;
    }
    .document-pane {
        border-right: 0;
        border-bottom: 1px solid var(--ide-border);
        max-height: 240px;
        flex-shrink: 0;
    }
    .storage-note {
        display: none;
    }
    .query-pane {
        flex: 1;
        min-height: 400px;
    }
    .query-options {
        grid-template-columns: 1fr;
    }
    .knowledge-welcome {
        margin: 0;
    }
}
</style>
