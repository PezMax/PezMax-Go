<template>
    <AssistantTheme>
        <div class="reading-toolbar">
            <span>PDF 阅读</span>
            <AButton size="small" :disabled="!signedIn" @click="open = true">
                <BookOutlined /> 阅读问答
            </AButton>
        </div>
        <ADrawer
            :open="open"
            title="阅读问答"
            :width="440"
            :mask="false"
            :get-container="false"
            :root-style="{ position: 'absolute', top: '44px' }"
            :content-wrapper-style="{ maxWidth: '100%' }"
            @close="close"
        >
            <div class="reading-body">
                <div class="reading-heading">
                    <strong class="reading-title" :title="title">{{ title || '当前 PDF' }}</strong>
                    <AButton
                        size="small"
                        type="text"
                        :disabled="working || state.configLoading"
                        @click="openSettings"
                    >
                        <SettingOutlined /> 模型设置
                    </AButton>
                </div>
                <p class="reading-copy">
                    选取 PDF 物理页码（从 1 开始，可能与书内页码不同），每次最多 10 页。
                </p>
                <div class="reading-range">
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
                <AButton block :disabled="working || !signedIn" @click="session.extract()">{{
                    state.pages.length ? '重新本地提取' : '本地提取所选页'
                }}</AButton>
                <p v-if="state.numPages" class="reading-copy">
                    此 PDF 共 {{ state.numPages }} 页。
                </p>
                <details v-if="state.pages.length" class="reading-preview">
                    <summary>
                        已提取 {{ state.pages.length }} 页 · {{ textLength }} 字符，查看文字
                    </summary>
                    <div class="reading-extracted">
                        <div v-for="page in state.pages" :key="page.page">
                            <strong>第 {{ page.page }} 页</strong>
                            <pre>{{ page.text || '此页未提取到文字' }}</pre>
                        </div>
                    </div>
                </details>
                <p class="reading-privacy">
                    提取在本机完成。点击“发送问题”后，会将问题和所选页的相关文字片段发给你配置的模型服务，并按服务商计费。不发送文件路径或下载地址。扫描件暂不支持
                    OCR。
                </p>
                <label for="reading-question" class="reading-label"
                    >关于所选页，你想了解什么？</label
                >
                <ATextarea
                    id="reading-question"
                    v-model:value="draft"
                    :maxlength="2000"
                    :rows="3"
                    :disabled="working"
                    placeholder="例如：这些页面的核心结论是什么？"
                />
                <AButton v-if="working" block @click="session.stop()"
                    >停止{{ state.extracting ? '提取' : '生成' }}</AButton
                >
                <AButton
                    v-else
                    type="primary"
                    block
                    :loading="state.configLoading"
                    :disabled="!signedIn || !state.pages.length || !draft.trim()"
                    @click="submit"
                    >{{ state.config.hasApiKey ? '发送问题' : '配置模型后发送' }}</AButton
                >
                <p v-if="state.status" role="status" class="reading-copy">{{ state.status }}</p>
                <AAlert
                    v-if="state.error || state.configError"
                    type="warning"
                    show-icon
                    :message="state.error || state.configError"
                />
                <section v-if="state.result" class="reading-result" aria-label="阅读回答">
                    <strong>{{ state.question }}</strong>
                    <p class="reading-copy">
                        参考 PDF 第 {{ state.result.contextPages.join('、') || '—' }} 页的{{
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
                        <p>{{ answer.text }}</p>
                        <blockquote v-for="(cite, citeIndex) in answer.citations" :key="citeIndex">
                            <AButton size="small" type="link" @click="jump(cite.page)"
                                >PDF 第 {{ cite.page }} 页 <ArrowRightOutlined
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

const props = defineProps({
    documentId: { type: String, required: true },
    source: { type: String, required: true },
    title: String
})
const emit = defineEmits(['jump-page', 'open-change'])
const user = useUserStore()
const signedIn = computed(() => !!user.token && !!user.id)
const state = reactive(createReadingAssistantState())
const session = createReadingAssistantSession({
    call: assistantCall,
    subscribe: onAssistantProgress,
    state
})
const open = ref(false),
    settingsOpen = ref(false),
    draft = ref('')
const working = computed(() => state.busy || state.extracting || state.settingsBusy)
const textLength = computed(() => state.pages.reduce((sum, page) => sum + page.text.length, 0))
let intentVersion = 0
const close = () => {
    intentVersion++
    open.value = false
    settingsOpen.value = false
    draft.value = ''
    session.clear()
}
const openSettings = async () => {
    await session.refreshConfig()
    if (open.value && signedIn.value) settingsOpen.value = true
}
const submit = async () => {
    if (working.value || !state.pages.length || !draft.value.trim()) return
    const id = props.documentId,
        source = props.source
    const version = ++intentVersion,
        question = draft.value
    await session.refreshConfig()
    if (
        version !== intentVersion ||
        !open.value ||
        !signedIn.value ||
        id !== props.documentId ||
        source !== props.source ||
        !state.pages.length
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
        !state.result?.answers.some((answer) => answer.citations.some((cite) => cite.page === page))
    )
        return
    emit('jump-page', { documentId: props.documentId, source: props.source, page })
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
    () => [props.documentId, props.source],
    () => {
        close()
        session.setDocument({ id: props.documentId, source: props.source })
    },
    { immediate: true, flush: 'sync' }
)
watch(open, (value) => {
    emit('open-change', value)
    if (value) void session.refreshConfig()
})
watch(
    () => [state.from, state.to],
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
