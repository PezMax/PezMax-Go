<template>
    <AssistantTheme>
        <div class="reading-toolbar">
            <span>{{ format.toUpperCase() }} 阅读</span>
            <AButton size="small" :disabled="!signedIn" @click="open = true">
                <BookOutlined /> 阅读助手
            </AButton>
        </div>
        <ADrawer
            :open="open"
            title="阅读助手"
            :width="440"
            :mask="false"
            :get-container="false"
            :root-style="{ position: 'absolute', top: '44px' }"
            :content-wrapper-style="{ maxWidth: '100%' }"
            @close="close"
        >
            <div class="reading-body">
                <div class="reading-heading">
                    <strong class="reading-title" :title="title">{{ title || '当前文档' }}</strong>
                    <AButton
                        size="small"
                        type="text"
                        :disabled="working || state.configLoading"
                        @click="openSettings"
                    >
                        <SettingOutlined /> 模型设置
                    </AButton>
                </div>
                <label class="reading-label">阅读方式</label>
                <ASelect
                    :value="state.mode"
                    :options="modeOptions"
                    :disabled="state.settingsBusy"
                    aria-label="阅读方式"
                    @update:value="session.setMode($event)"
                />
                <template v-if="isEpub">
                    <p class="reading-copy">
                        EPUB 按书脊顺序划分分节，每次最多选 5
                        节；分节序号不是页码，也可能与书内章节号不同。
                    </p>
                    <AButton :disabled="working" block @click="session.loadChapters()">{{
                        state.chapterOptions.length ? '重新读取分节目录' : '读取分节目录（本机）'
                    }}</AButton>
                    <ASelect
                        :value="state.selectedChapters"
                        mode="multiple"
                        :options="state.chapterOptions"
                        :max-tag-count="2"
                        :disabled="!state.chapterOptions.length || state.settingsBusy"
                        option-filter-prop="label"
                        placeholder="选择1到5节正文"
                        aria-label="EPUB所选分节"
                        @update:value="session.setChapters($event)"
                    />
                </template>
                <p v-else class="reading-copy">
                    选取 PDF 物理页码（从 1 开始，可能与书内页码不同），每次最多 10 页。
                </p>
                <div v-if="!isEpub" class="reading-range">
                    <label
                        >起始页
                        <AInputNumber
                            :value="state.from"
                            :min="1"
                            :max="state.numPages || 10000"
                            :precision="0"
                            aria-label="阅读起始页"
                            @update:value="session.setRange($event, state.to)"
                    /></label>
                    <span>至</span>
                    <label
                        >结束页
                        <AInputNumber
                            :value="state.to"
                            :min="1"
                            :max="state.numPages || 10000"
                            :precision="0"
                            aria-label="阅读结束页"
                            @update:value="session.setRange(state.from, $event)"
                    /></label>
                </div>
                <AButton
                    block
                    :disabled="
                        working ||
                        !signedIn ||
                        (isEpub &&
                            (!state.selectedChapters.length || state.selectedChapters.length > 5))
                    "
                    @click="session.extract()"
                    >{{
                        items.length
                            ? '重新本地提取'
                            : isEpub
                              ? '本地提取所选分节'
                              : '本地提取所选页'
                    }}</AButton
                >
                <p v-if="state.numPages" class="reading-copy">
                    此 PDF 共 {{ state.numPages }} 页。
                </p>
                <details v-if="items.length" class="reading-preview">
                    <summary>
                        已提取 {{ items.length }} {{ isEpub ? '节' : '页' }} ·
                        {{ textLength }} 字符，查看文字
                    </summary>
                    <div class="reading-extracted">
                        <div v-for="page in items" :key="page[locationKey]">
                            <strong>{{ locationLabel(page[locationKey]) }}</strong>
                            <pre>{{ page.text || '此处未提取到文字' }}</pre>
                        </div>
                    </div>
                </details>
                <p class="reading-privacy">
                    提取在本机完成。点击发送或生成后，会将本次任务和所选内容的相关文字片段发给你配置的模型服务，并按服务商计费。不发送本地文件路径或下载地址。图片内容暂不支持
                    OCR。通常调用 1 次模型；结构或引用校验失败时最多额外纠正 1 次。
                </p>
                <label v-if="state.mode === 'qa'" for="reading-question" class="reading-label"
                    >关于所选文字，你想了解什么？</label
                >
                <ATextarea
                    v-if="state.mode === 'qa'"
                    id="reading-question"
                    v-model:value="draft"
                    :maxlength="2000"
                    :rows="3"
                    :disabled="working"
                    placeholder="例如：这部分内容的核心结论是什么？"
                />
                <p v-else class="reading-copy">
                    {{
                        state.mode === 'summary'
                            ? '提炼所选文字的核心观点与结论'
                            : '按所选文字组织最多 5 项有序提纲'
                    }}，每项附原文依据。长内容会抽取部分片段，结果不代表完整文档。
                </p>
                <AButton v-if="working" block @click="session.stop()"
                    >停止{{ state.extracting ? '提取' : '生成' }}</AButton
                >
                <AButton
                    v-else
                    type="primary"
                    block
                    :loading="state.configLoading"
                    :disabled="!signedIn || !items.length || (state.mode === 'qa' && !draft.trim())"
                    @click="submit"
                    >{{
                        state.config.hasApiKey
                            ? state.mode === 'qa'
                                ? '发送问题'
                                : `生成${readingModes[state.mode].label}`
                            : '配置模型后使用'
                    }}</AButton
                >
                <p v-if="state.status" role="status" class="reading-copy">{{ state.status }}</p>
                <AAlert
                    v-if="state.error || state.configError"
                    type="warning"
                    show-icon
                    :message="state.error || state.configError"
                />
                <section v-if="state.result" class="reading-result" aria-label="阅读回答">
                    <strong>{{
                        state.mode === 'qa' ? state.question : readingModes[state.mode].label
                    }}</strong>
                    <p v-if="state.result.diagnostics" class="reading-copy">
                        本轮 {{ state.result.diagnostics.modelCalls }} 次模型调用 ·
                        {{ elapsedSeconds }}
                        秒（含本机处理）
                    </p>
                    <p class="reading-copy">
                        参考 {{ contextDescription }} 的{{
                            state.result.partial ? '部分' : '所选'
                        }}文字。引用已核对原文，回答仍需结合上下文检查。
                    </p>
                    <p v-if="!state.result.answers.length" role="status">
                        {{ state.result.content }}
                    </p>
                    <article
                        v-for="(answer, index) in state.result.answers"
                        :key="index"
                        class="reading-answer"
                    >
                        <p>
                            {{ state.mode === 'outline' ? `${index + 1}. ` : '' }}{{ answer.text }}
                        </p>
                        <blockquote v-for="(cite, citeIndex) in answer.citations" :key="citeIndex">
                            <AButton size="small" type="link" @click="jump(cite[locationKey])"
                                >{{ locationLabel(cite[locationKey]) }} <ArrowRightOutlined
                            /></AButton>
                            <p>{{ cite.quote }}</p>
                        </blockquote>
                    </article>
                </section>
            </div>
        </ADrawer>
        <AssistantSettings
            v-model:open="settingsOpen"
            :config="state.config"
            :busy="working"
            :save-settings="session.saveSettings"
            :clear-key="session.clearKey"
        />
    </AssistantTheme>
</template>

<script setup>
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import {
    Alert as AAlert,
    Button as AButton,
    Drawer as ADrawer,
    InputNumber as AInputNumber,
    Select as ASelect,
    Textarea as ATextarea
} from 'ant-design-vue'
import { ArrowRightOutlined, BookOutlined, SettingOutlined } from '@ant-design/icons-vue'
import useUserStore from '@/store/modules/user'
import { assistantCall, onAssistantProgress } from '@/api/assistant'
import {
    createReadingAssistantSession,
    createReadingAssistantState
} from '@/utils/reading-assistant-session.mjs'
import AssistantTheme from './AssistantTheme.vue'
import AssistantSettings from './AssistantSettings.vue'
import { readingModes } from '../../../shared/reading-assistant.mjs'

const props = defineProps({
    documentId: { type: String, required: true },
    source: { type: String, required: true },
    format: { type: String, default: 'pdf' },
    epubLoader: Function,
    title: String
})
const emit = defineEmits(['jump-page', 'jump-chapter', 'open-change'])
const user = useUserStore()
const signedIn = computed(() => !!user.token && !!user.id)
const state = reactive(createReadingAssistantState())
const session = createReadingAssistantSession({
    call: assistantCall,
    subscribe: onAssistantProgress,
    loadEpub: (options) =>
        props.epubLoader
            ? props.epubLoader(options)
            : Promise.reject(new Error('EPUB尚未载入，请稍后再试')),
    state
})
const open = ref(false),
    settingsOpen = ref(false),
    draft = ref('')
const working = computed(() => state.busy || state.extracting || state.settingsBusy)
const isEpub = computed(() => props.format === 'epub')
const locationKey = computed(() => (isEpub.value ? 'chapter' : 'page'))
const items = computed(() => (isEpub.value ? state.chapters : state.pages))
const modeOptions = Object.entries(readingModes).map(([value, item]) => ({
    value,
    label: item.label
}))
const textLength = computed(() => items.value.reduce((sum, page) => sum + page.text.length, 0))
const elapsedSeconds = computed(() =>
    ((state.result?.diagnostics?.durationMs || 0) / 1000).toFixed(1)
)
const locationLabel = (number) =>
    isEpub.value
        ? state.chapterOptions.find((option) => option.value === number)?.label ||
          `EPUB 第 ${number} 节`
        : `PDF 第 ${number} 页`
const contextDescription = computed(
    () =>
        (isEpub.value ? state.result?.contextChapters : state.result?.contextPages)
            ?.map(locationLabel)
            .join('、') || '所选文字'
)
let intentVersion = 0
const close = () => {
    intentVersion++
    open.value = false
    settingsOpen.value = false
    draft.value = ''
    session.clear()
}
const openSettings = async () => {
    const version = intentVersion
    await session.refreshConfig()
    if (version === intentVersion && open.value && signedIn.value) settingsOpen.value = true
}
const submit = async () => {
    if (working.value || !items.value.length || (state.mode === 'qa' && !draft.value.trim())) return
    const id = props.documentId,
        source = props.source
    const version = ++intentVersion,
        question = state.mode === 'qa' ? draft.value : readingModes[state.mode].prompt
    await session.refreshConfig()
    if (
        version !== intentVersion ||
        !open.value ||
        !signedIn.value ||
        id !== props.documentId ||
        source !== props.source ||
        !items.value.length
    )
        return
    if (!state.config.hasApiKey) {
        settingsOpen.value = true
        return
    }
    await session.ask(question)
}
const jump = (page) => {
    if (
        !state.result?.answers.some((answer) =>
            answer.citations.some((cite) => cite[locationKey.value] === page)
        )
    )
        return
    emit(isEpub.value ? 'jump-chapter' : 'jump-page', {
        documentId: props.documentId,
        source: props.source,
        [locationKey.value]: page
    })
}
watch(
    () => [user.token, String(user.id || '')],
    () => {
        close()
        void session.setIdentity(
            signedIn.value
                ? {
                      server:
                          import.meta.env.VITE_APP_TARGET_URL ||
                          import.meta.env.VITE_APP_BASE_API ||
                          'pezmax',
                      userId: String(user.id)
                  }
                : null
        )
    },
    { immediate: true, flush: 'sync' }
)
watch(
    () => [props.documentId, props.source, props.format],
    () => {
        close()
        session.setDocument({ id: props.documentId, source: props.source, format: props.format })
    },
    { immediate: true, flush: 'sync' }
)
watch(open, (value) => {
    emit('open-change', value)
    if (value) void session.refreshConfig()
})
watch(
    () => [state.from, state.to, state.mode, JSON.stringify(state.selectedChapters)],
    () => {
        intentVersion++
    },
    { flush: 'sync' }
)
onUnmounted(() => {
    session.dispose()
    emit('open-change', false)
})
</script>

<style scoped>
.reading-toolbar {
    height: 44px;
    padding: 0 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--ide-border);
    color: var(--ide-text-light);
    background: var(--ide-panel-bg);
    font-size: 12px;
}
.reading-body {
    display: grid;
    gap: 12px;
    color: var(--ide-text);
    font-size: 13px;
    line-height: 1.7;
    overflow-wrap: anywhere;
    text-wrap: pretty;
}
.reading-heading {
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: space-between;
}
.reading-title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
}
.reading-copy,
.reading-privacy {
    margin: 0;
    color: var(--ide-text-light);
    font-size: 12px;
}
.reading-range {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    align-items: center;
}
.reading-range label {
    display: grid;
    gap: 4px;
}
.reading-preview summary {
    cursor: pointer;
    color: var(--ide-accent);
}
.reading-extracted {
    max-height: 200px;
    overflow: auto;
    padding: 12px;
    margin-top: 8px;
    border: 1px solid var(--ide-border);
    border-radius: 10px;
}
.reading-extracted pre {
    white-space: pre-wrap;
    font: inherit;
    margin: 4px 0 12px;
}
.reading-label,
.reading-title,
.reading-result > strong {
    color: var(--ide-text-active);
}
.reading-result {
    border-top: 1px solid var(--ide-border);
    padding-top: 16px;
}
.reading-result > .reading-copy {
    margin-top: 8px;
}
.reading-answer > p {
    white-space: pre-wrap;
}
.reading-answer blockquote {
    margin: 12px 0;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--ide-bg);
    border: 1px solid var(--ide-border);
}
.reading-answer blockquote p {
    margin: 4px 0 0;
    white-space: pre-wrap;
}
</style>
