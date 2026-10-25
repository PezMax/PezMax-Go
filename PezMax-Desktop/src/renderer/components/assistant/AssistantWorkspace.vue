<template>
    <AssistantTheme>
        <section class="assistant-workspace" aria-label="找资料助手">
            <header class="assistant-header">
                <div class="assistant-heading">
                    <span class="assistant-mark"><RobotOutlined /></span>
                    <div>
                        <h1>找资料助手</h1>
                        <span class="assistant-caption">用你的模型，查找平台里的真实资源</span>
                    </div>
                </div>
                <div class="assistant-header-actions">
                    <ATag
                        v-if="state.config.hasApiKey"
                        class="model-tag"
                        :title="state.config.model"
                        >{{ state.config.model }}</ATag
                    >
                    <AButton
                        :disabled="state.busy || state.settingsBusy || !state.turns.length"
                        @click="newConversation"
                    >
                        <template #icon><PlusOutlined /></template>新对话
                    </AButton>
                    <AButton
                        :disabled="
                            state.busy || state.settingsBusy || state.configLoading || !signedIn
                        "
                        @click="settingsOpen = true"
                    >
                        <template #icon><SettingOutlined /></template>模型设置
                    </AButton>
                </div>
            </header>

            <div ref="scrollArea" class="assistant-scroll" @scroll="trackScroll">
                <div class="assistant-thread">
                    <AAlert
                        v-if="state.configError"
                        :message="state.configError"
                        type="warning"
                        show-icon
                        class="config-alert"
                    />
                    <div v-if="!state.turns.length" class="assistant-welcome">
                        <div class="welcome-kicker"><SearchOutlined /> 学习资源，一处查找</div>
                        <h2>想找什么学习资料？</h2>
                        <p class="welcome-description">
                            告诉我学科、学校或年份，<br
                                class="small-break"
                            />我来帮你筛选试卷资料、电子书和学习书签。
                        </p>
                        <div
                            v-if="!state.config.hasApiKey && !state.configLoading"
                            class="setup-card"
                        >
                            <KeyOutlined class="setup-icon" />
                            <div>
                                <strong>先连接你的模型</strong>
                                <p>填写接口地址、模型 ID 和 API Key，即可开始。</p>
                            </div>
                            <AButton
                                type="primary"
                                :disabled="!signedIn"
                                @click="settingsOpen = true"
                                >配置模型<ArrowRightOutlined
                            /></AButton>
                        </div>
                        <div v-else-if="state.configLoading" class="config-loading">
                            <ASpin size="small" /> 正在读取模型设置…
                        </div>
                        <div class="suggestion-grid">
                            <AButton
                                v-for="suggestion in suggestions"
                                :key="suggestion.title"
                                class="suggestion-button"
                                :disabled="state.configLoading || !signedIn"
                                @click="chooseSuggestion(suggestion.prompt)"
                            >
                                <component :is="suggestion.icon" class="suggestion-icon" />
                                <strong>{{ suggestion.title }}</strong>
                                <span>{{ suggestion.description }}</span>
                                <ArrowRightOutlined class="suggestion-arrow" />
                            </AButton>
                        </div>
                        <div class="welcome-note">
                            <SafetyCertificateOutlined /> 仅检索已上架目录，找到的资源可直接打开
                        </div>
                    </div>

                    <article
                        v-for="(turn, index) in state.turns"
                        :key="turn.id"
                        class="conversation-turn"
                    >
                        <div class="question-row">
                            <p class="question-bubble" aria-label="你的提问">{{ turn.question }}</p>
                        </div>
                        <div class="answer-heading">
                            <span class="answer-avatar"><RobotOutlined /></span
                            ><strong>找资料助手</strong
                            ><span v-if="turn.phase === 'done'" class="answer-label">本轮完成</span>
                        </div>
                        <div class="answer-body">
                            <div
                                v-if="turn.phase === 'running'"
                                class="assistant-progress"
                                role="status"
                                aria-live="polite"
                            >
                                <LoadingOutlined spin />
                                {{ state.stopping ? '正在停止…' : turn.status || '正在准备…' }}
                            </div>
                            <p v-if="turn.content" class="answer-text">{{ turn.content }}</p>
                            <template v-if="turn.resources.length">
                                <div class="resource-heading">
                                    <span>找到的资源</span
                                    ><ATag>{{ turn.resources.length }} 项</ATag
                                    ><span class="resource-caption">点击卡片按钮打开预览</span>
                                </div>
                                <div class="resource-grid">
                                    <ACard
                                        v-for="item in turn.resources"
                                        :key="item.id"
                                        size="small"
                                        class="resource-card"
                                    >
                                        <div class="resource-topline">
                                            <span class="resource-kind"
                                                ><component :is="kindIcons[item.kind]" />{{
                                                    kindLabels[item.kind]
                                                }}</span
                                            ><span v-if="item.format" class="resource-format">{{
                                                item.format.toUpperCase()
                                            }}</span>
                                        </div>
                                        <h3>{{ item.title }}</h3>
                                        <p class="resource-meta">{{ resourceMetadata(item) }}</p>
                                        <div class="resource-footer">
                                            <span v-if="item.subject" :title="item.subject">{{
                                                item.subject
                                            }}</span
                                            ><span v-else>平台资源</span
                                            ><AButton
                                                size="small"
                                                @click="emit('open-resource', item.node)"
                                                >打开{{ kindLabels[item.kind] }}<ArrowRightOutlined
                                            /></AButton>
                                        </div>
                                    </ACard>
                                </div>
                            </template>
                            <AAlert
                                v-if="searchNotice(turn)"
                                :message="searchNotice(turn)"
                                type="info"
                                show-icon
                                class="search-notice"
                            />
                            <AAlert
                                v-if="turn.error"
                                :message="turn.error"
                                :type="turn.phase === 'stopped' ? 'info' : 'error'"
                                show-icon
                                class="search-notice"
                                role="status"
                            />
                            <AButton
                                v-if="
                                    index === state.turns.length - 1 &&
                                    ['error', 'stopped'].includes(turn.phase)
                                "
                                size="small"
                                class="retry-button"
                                :disabled="
                                    state.busy || state.settingsBusy || !state.config.hasApiKey
                                "
                                @click="retry"
                                ><template #icon><ReloadOutlined /></template>重试本轮</AButton
                            >
                        </div>
                    </article>
                </div>
            </div>

            <footer class="assistant-composer-area">
                <div class="assistant-composer-container">
                    <AAlert
                        v-if="submitError"
                        :message="submitError"
                        type="warning"
                        show-icon
                        closable
                        @close="submitError = ''"
                        class="composer-error"
                    />
                    <div class="assistant-composer">
                        <ATextarea
                            ref="composer"
                            v-model:value="draft"
                            aria-label="描述你想找的学习资料"
                            placeholder="例如：帮我找近三年高等数学的期末试卷"
                            :maxlength="2000"
                            :auto-size="{ minRows: 2, maxRows: 5 }"
                            :bordered="false"
                            :disabled="!signedIn || state.configLoading || state.settingsBusy"
                            @keydown="handleComposerKey"
                        />
                        <div class="composer-toolbar">
                            <span class="composer-scope"
                                ><DatabaseOutlined /> 平台目录<span class="composer-shortcut">
                                    · Enter 发送，Shift + Enter 换行</span
                                ></span
                            >
                            <AButton
                                v-if="state.busy"
                                :disabled="state.stopping"
                                @click="session.stop()"
                                ><template #icon><StopOutlined /></template
                                >{{ state.stopping ? '正在停止' : '停止生成' }}</AButton
                            >
                            <AButton
                                v-else
                                type="primary"
                                :disabled="
                                    !draft.trim() ||
                                    state.configLoading ||
                                    state.settingsBusy ||
                                    !signedIn
                                "
                                @click="submit"
                                ><template #icon><ArrowUpOutlined /></template>发送</AButton
                            >
                        </div>
                    </div>
                    <p class="composer-privacy">
                        <LockOutlined />
                        密钥留在本机；问题与目录信息发送至你配置的模型服务。当前不读取文件正文。
                    </p>
                </div>
            </footer>
            <AssistantSettings
                v-model:open="settingsOpen"
                :config="state.config"
                :busy="state.settingsBusy"
                :save-settings="session.saveSettings"
                :clear-key="session.clearKey"
            />
        </section>
    </AssistantTheme>
</template>

<script setup>
import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue'
import {
    Alert as AAlert,
    Button as AButton,
    Card as ACard,
    Spin as ASpin,
    Tag as ATag,
    Textarea as ATextarea
} from 'ant-design-vue'
import {
    ArrowRightOutlined,
    ArrowUpOutlined,
    BookOutlined,
    DatabaseOutlined,
    FileSearchOutlined,
    FileTextOutlined,
    KeyOutlined,
    LinkOutlined,
    LoadingOutlined,
    LockOutlined,
    PlusOutlined,
    ReloadOutlined,
    RobotOutlined,
    SafetyCertificateOutlined,
    SearchOutlined,
    SettingOutlined,
    StopOutlined
} from '@ant-design/icons-vue'
import useUserStore from '@/store/modules/user'
import { assistantCall, onAssistantProgress } from '@/api/assistant'
import { createAssistantSession, createAssistantState } from '@/utils/assistant-session.mjs'
import AssistantSettings from './AssistantSettings.vue'
import AssistantTheme from './AssistantTheme.vue'

const props = defineProps({ active: Boolean })
const emit = defineEmits(['open-resource'])
const userStore = useUserStore()
const signedIn = computed(() => !!userStore.token && !!userStore.id)
const state = reactive(createAssistantState())
const session = createAssistantSession({
    call: assistantCall,
    subscribe: onAssistantProgress,
    state
})
const settingsOpen = ref(false)
const draft = ref('')
const submitError = ref('')
const scrollArea = ref(null)
const composer = ref(null)
let followBottom = true

const kindLabels = { file: '资料', ebook: '电子书', bookmark: '书签' }
const kindIcons = { file: FileTextOutlined, ebook: BookOutlined, bookmark: LinkOutlined }
const fileTypes = {
    1: '期末试卷',
    2: '期中试卷',
    3: '补考试卷',
    4: '学习资料',
    5: '其他学校',
    6: '神秘文件'
}
const suggestions = [
    {
        icon: FileSearchOutlined,
        title: '准备考试',
        description: '按学科、年份找试卷',
        prompt: '帮我找近三年高等数学的期末试卷'
    },
    {
        icon: BookOutlined,
        title: '寻找教材',
        description: '查找相关电子书',
        prompt: '帮我找线性代数相关的电子书'
    },
    {
        icon: LinkOutlined,
        title: '拓展学习',
        description: '发现有用的学习网站',
        prompt: '有没有适合 Python 入门的学习网站？'
    }
]
const resourceMetadata = (item) => {
    if (item.kind === 'file')
        return (
            [item.school, item.year ? `${item.year}年` : '', fileTypes[item.fileType]]
                .filter(Boolean)
                .join(' · ') || '资料目录'
        )
    if (item.kind === 'ebook')
        return [item.author, item.publisher].filter(Boolean).join(' · ') || '电子书目录'
    return item.description || item.collection || '学习书签'
}
const searchNotice = (turn) => {
    const failed = [
        ...new Set(turn.reports.filter((item) => item.failed).map((item) => kindLabels[item.kind]))
    ]
    const skipped = [
        ...new Set(turn.reports.filter((item) => item.skipped).map((item) => kindLabels[item.kind]))
    ]
    const notes = []
    if (failed.length)
        notes.push(`${failed.join('、')}的部分检索未完成，不能据此判断没有匹配资源。`)
    if (skipped.length)
        notes.push(`${skipped.join('、')}不支持本轮的学校、年份或试卷类型条件，未查询。`)
    if (turn.reports.some((item) => item.partial && !item.failed))
        notes.push('当前展示部分结果，每次检索每类最多显示 5 项；可补充条件继续查找。')
    return notes.join(' ')
}

watch(
    () => [userStore.token, String(userStore.id || '')],
    () => {
        settingsOpen.value = false
        draft.value = ''
        submitError.value = ''
        followBottom = true
        void session.setIdentity(
            signedIn.value
                ? {
                      server:
                          import.meta.env.VITE_APP_TARGET_URL ||
                          import.meta.env.VITE_APP_BASE_API ||
                          'pezmax',
                      userId: String(userStore.id),
                      token: userStore.token
                  }
                : null
        )
    },
    { immediate: true, flush: 'sync' }
)
onUnmounted(() => session.dispose())

const focusComposer = () => nextTick(() => composer.value?.focus())
watch(
    () => props.active,
    (active) => {
        if (active && !settingsOpen.value) focusComposer()
        if (active) void session.refreshConfig()
    }
)
const trackScroll = () => {
    const area = scrollArea.value
    if (area) followBottom = area.scrollHeight - area.scrollTop - area.clientHeight < 120
}
watch(
    () => [
        state.turns.length,
        state.turns.at(-1)?.status,
        state.turns.at(-1)?.resources.length,
        state.turns.at(-1)?.phase
    ],
    async () => {
        await nextTick()
        if (followBottom && scrollArea.value)
            scrollArea.value.scrollTop = scrollArea.value.scrollHeight
    }
)
const submit = async () => {
    if (state.busy || !draft.value.trim()) return
    submitError.value = ''
    if (!state.config.hasApiKey) {
        settingsOpen.value = true
        return
    }
    const question = draft.value
    draft.value = ''
    followBottom = true
    try {
        await session.send(question)
    } catch (error) {
        draft.value = question
        submitError.value = error.message
    }
    if (props.active) focusComposer()
}
const chooseSuggestion = (question) => {
    draft.value = question
    if (!state.config.hasApiKey) settingsOpen.value = true
    else focusComposer()
}
const handleComposerKey = (event) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && event.keyCode !== 229) {
        event.preventDefault()
        void submit()
    }
}
const retry = async () => {
    followBottom = true
    try {
        await session.retryLast()
    } catch (error) {
        submitError.value = error.message
    }
}
const newConversation = () => {
    session.newConversation()
    submitError.value = ''
    focusComposer()
}
</script>

<style scoped lang="scss">
.assistant-workspace {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    height: 100%;
    background: var(--ide-editor-bg);
    color: var(--ide-text);
    border: 1px solid var(--ide-border);
    border-radius: 8px;
    overflow: hidden;
}
.assistant-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 20px 24px;
    border-bottom: 1px solid var(--ide-border);
}
.assistant-heading {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
}
.assistant-heading h1 {
    margin: 0 0 4px;
    color: var(--ide-text-active);
    font-size: 18px;
    font-weight: 600;
    line-height: 1.4;
}
.assistant-mark {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border: 1px solid var(--ide-border);
    border-radius: 12px;
    color: var(--ide-accent);
    background: var(--ide-accent-light);
    font-size: 23px;
}
.assistant-caption {
    font-size: 12px;
    color: var(--ide-text-light);
}
.assistant-header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
}
.model-tag {
    max-width: 180px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-inline-end: 4px;
}
.assistant-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
}
.assistant-thread {
    width: 100%;
    max-width: 920px;
    margin: 0 auto;
    padding: 32px 32px 16px;
    box-sizing: border-box;
}
.config-alert {
    margin-bottom: 20px;
}
.assistant-welcome {
    padding: 12px 0 24px;
}
.welcome-kicker {
    color: var(--ide-accent);
    font-size: 13px;
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 20px;
}
.assistant-welcome h2 {
    font-size: clamp(25px, 2.7vw, 34px);
    line-height: 1.4;
    color: var(--ide-text-active);
    letter-spacing: -0.6px;
    margin: 0 0 12px;
    font-weight: 600;
}
.welcome-description {
    font-size: 14px;
    line-height: 1.8;
    color: var(--ide-text);
    margin: 0 0 28px;
}
.small-break {
    display: none;
}
.setup-card {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 20px;
    margin-bottom: 20px;
    background: var(--ide-bg);
    border: 1px solid var(--ide-border);
    border-radius: 12px;
}
.setup-icon {
    color: var(--ide-accent);
    font-size: 26px;
}
.setup-card > div {
    flex: 1;
}
.setup-card strong {
    color: var(--ide-text-active);
    font-size: 14px;
    font-weight: 600;
}
.setup-card p {
    font-size: 12px;
    line-height: 1.6;
    margin: 6px 0 0;
    color: var(--ide-text);
}
.config-loading {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 0;
    font-size: 13px;
}
.suggestion-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
}
.suggestion-button {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    position: relative;
    height: auto;
    min-height: 122px;
    padding: 16px;
    text-align: left;
    white-space: normal;
}
.suggestion-icon {
    color: var(--ide-accent);
    font-size: 20px;
    margin-bottom: 16px;
}
.suggestion-button strong {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 6px;
}
.suggestion-button > span:not(.anticon) {
    font-size: 12px;
    color: var(--ide-text-light);
    line-height: 1.6;
}
.suggestion-arrow {
    position: absolute;
    right: 16px;
    top: 20px;
    color: var(--ide-text-light);
    font-size: 12px;
}
.welcome-note {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 24px;
    font-size: 12px;
    color: var(--ide-text-light);
}
.conversation-turn {
    margin-bottom: 32px;
}
.question-row {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 24px;
}
.question-bubble {
    max-width: 85%;
    padding: 12px 16px;
    border-radius: 12px 12px 3px 12px;
    background: var(--ide-bg);
    color: var(--ide-text-active);
    border: 1px solid var(--ide-border);
    font-size: 14px;
    line-height: 1.8;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    margin: 0;
}
.answer-heading {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
    font-size: 13px;
    color: var(--ide-text-active);
}
.answer-avatar {
    color: var(--ide-accent);
    background: var(--ide-accent-light);
    border-radius: 8px;
    width: 28px;
    height: 28px;
    display: grid;
    place-items: center;
    font-size: 17px;
}
.answer-heading strong {
    font-weight: 600;
}
.answer-label {
    font-size: 11px;
    font-weight: 400;
    color: var(--ide-text-light);
    margin-left: 4px;
}
.answer-body {
    padding-left: 36px;
}
.answer-text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    font-size: 14px;
    line-height: 1.9;
    margin: 0 0 16px;
    color: var(--ide-text-active);
}
.assistant-progress {
    display: flex;
    gap: 8px;
    align-items: center;
    color: var(--ide-text);
    font-size: 13px;
    padding: 4px 0 16px;
}
.assistant-progress .anticon {
    color: var(--ide-accent);
}
.resource-heading {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 16px 0 12px;
    font-size: 12px;
    color: var(--ide-text-active);
}
.resource-heading :deep(.ant-tag) {
    margin: 0;
    font-size: 11px;
}
.resource-caption {
    color: var(--ide-text-light);
    margin-left: auto;
    font-size: 11px;
}
.resource-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
}
.resource-card {
    height: 100%;
}
.resource-card :deep(.ant-card-body) {
    display: flex;
    flex-direction: column;
    height: 100%;
    box-sizing: border-box;
    padding: 16px;
}
.resource-topline {
    display: flex;
    justify-content: space-between;
    align-items: center;
    color: var(--ide-text-light);
    font-size: 11px;
    margin-bottom: 12px;
}
.resource-kind {
    display: inline-flex;
    align-items: center;
    gap: 6px;
}
.resource-kind .anticon {
    color: var(--ide-accent);
    font-size: 14px;
}
.resource-format {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.5px;
}
.resource-card h3 {
    font-size: 14px;
    font-weight: 600;
    color: var(--ide-text-active);
    line-height: 1.7;
    margin: 0 0 8px;
    overflow-wrap: anywhere;
}
.resource-meta {
    color: var(--ide-text);
    font-size: 12px;
    line-height: 1.7;
    margin: 0 0 16px;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}
.resource-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    margin-top: auto;
    padding-top: 12px;
    border-top: 1px solid var(--ide-border);
}
.resource-footer > span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ide-text-light);
    font-size: 11px;
}
.resource-footer :deep(.ant-btn) {
    flex-shrink: 0;
    font-size: 12px;
}
.search-notice {
    margin-top: 16px;
}
.search-notice :deep(.ant-alert-message) {
    font-size: 12px;
    line-height: 1.7;
}
.retry-button {
    margin-top: 12px;
}
.assistant-composer-area {
    flex-shrink: 0;
    padding: 12px 32px 16px;
}
.assistant-composer-container {
    max-width: 856px;
    margin: 0 auto;
}
.assistant-composer {
    padding: 12px;
    border: 1px solid var(--ide-border-hover);
    border-radius: 12px;
    background: var(--ide-editor-bg);
    transition:
        border-color 160ms ease,
        box-shadow 160ms ease;
}
.assistant-composer:focus-within {
    border-color: var(--ide-accent);
    box-shadow: 0 0 0 2px rgba(var(--ide-accent-rgb), 0.1);
}
.assistant-composer :deep(textarea) {
    resize: none;
    padding: 4px;
    font-size: 14px;
    line-height: 1.8;
    background: transparent;
}
.composer-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
}
.composer-scope {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--ide-text-light);
    padding-left: 4px;
}
.composer-scope > .anticon {
    color: var(--ide-accent);
}
.composer-shortcut {
    margin-left: 0;
}
.composer-privacy {
    text-align: center;
    color: var(--ide-text-light);
    font-size: 11px;
    line-height: 1.6;
    margin: 10px 0 0;
}
.composer-privacy .anticon {
    margin-right: 4px;
}
.composer-error {
    margin-bottom: 10px;
}
@media (max-width: 1050px) {
    .assistant-caption,
    .model-tag {
        display: none;
    }
    .assistant-header {
        padding: 16px 20px;
    }
    .assistant-thread {
        padding: 24px 24px 12px;
    }
    .assistant-composer-area {
        padding: 12px 24px;
    }
}
@media (max-width: 720px) {
    .assistant-heading h1 {
        font-size: 16px;
    }
    .assistant-mark {
        display: none;
    }
    .assistant-header {
        gap: 8px;
        padding: 12px 16px;
    }
    .assistant-header-actions {
        gap: 6px;
    }
    .assistant-header-actions :deep(.ant-btn) {
        padding-inline: 10px;
    }
    .assistant-thread {
        padding: 24px 16px 8px;
    }
    .assistant-welcome {
        padding-top: 0;
    }
    .setup-card {
        flex-wrap: wrap;
        padding: 16px;
    }
    .setup-card > .ant-btn {
        margin-left: 42px;
    }
    .suggestion-grid {
        grid-template-columns: 1fr;
        gap: 8px;
    }
    .suggestion-button {
        min-height: 72px;
        padding-left: 48px;
        justify-content: center;
    }
    .suggestion-icon {
        position: absolute;
        left: 16px;
        top: 20px;
    }
    .suggestion-button strong {
        margin-bottom: 4px;
    }
    .resource-grid {
        grid-template-columns: 1fr;
    }
    .answer-body {
        padding-left: 0;
    }
    .resource-caption,
    .composer-shortcut {
        display: none;
    }
    .assistant-composer-area {
        padding: 8px 16px 12px;
    }
    .composer-privacy {
        font-size: 10px;
    }
}
@media (prefers-reduced-motion: reduce) {
    .assistant-composer {
        transition: none;
    }
}
</style>
