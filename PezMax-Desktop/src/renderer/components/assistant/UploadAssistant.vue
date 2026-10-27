<template>
    <AssistantTheme>
        <section class="upload-assistant" aria-label="上传命名与分类建议">
            <div class="upload-assistant-heading">
                <strong><RobotOutlined /> 上传建议</strong>
                <AButton
                    size="small"
                    type="text"
                    :disabled="busy || state.configLoading || !signedIn || locked"
                    @click="openSettings"
                >
                    <SettingOutlined />模型设置
                </AButton>
            </div>
            <p class="upload-assistant-copy">
                根据文件名整理{{ file?.isFolder ? '共同分类' : '名称和分类' }}，由你确认后填入表单。
            </p>
            <p class="upload-assistant-privacy">
                仅在点击生成时发送文件名给你的模型服务，不发送正文或本地路径。调用按服务商计费。
            </p>
            <AAlert
                v-if="inputError || state.configError || state.error"
                type="warning"
                show-icon
                :message="inputError || state.configError || state.error"
            />
            <p v-if="!signedIn" class="upload-assistant-copy">登录后可使用上传建议。</p>
            <div class="upload-assistant-actions">
                <AButton v-if="state.busy" block @click="session.stop()">停止生成</AButton>
                <AButton
                    v-else
                    type="primary"
                    block
                    :loading="state.configLoading || state.settingsBusy"
                    :disabled="!!inputError || !signedIn || locked"
                    @click="generate"
                >
                    {{
                        state.config.hasApiKey
                            ? state.suggestions.length
                                ? '重新生成建议'
                                : '生成上传建议'
                            : '配置模型后使用'
                    }}
                </AButton>
            </div>
            <p v-if="state.status" role="status" class="upload-assistant-copy">
                <ASpin size="small" /> {{ state.status }}
            </p>
            <AButton
                v-if="state.suggestions.length"
                block
                class="upload-review-button"
                :disabled="busy || locked"
                @click="reviewOpen = true"
                >查看并应用 {{ state.suggestions.length }} 项建议</AButton
            >
            <p v-if="state.feedback" role="status" class="upload-assistant-copy">
                {{ state.feedback }}
            </p>
            <AModal
                :open="reviewOpen"
                title="确认上传建议"
                :width="620"
                :footer="null"
                @cancel="reviewOpen = false"
            >
                <p class="upload-assistant-copy">
                    对照当前表单，勾选需要回填的字段。生成后手动改过的字段会被跳过。
                </p>
                <div class="upload-suggestion-list">
                    <div
                        v-for="item in state.suggestions"
                        :key="item.field"
                        class="upload-suggestion-row"
                    >
                        <ACheckbox
                            v-model:checked="item.selected"
                            :disabled="busy || locked"
                            :aria-label="`应用${label(item.field)}建议`"
                        >
                            <strong>{{ label(item.field) }}</strong>
                        </ACheckbox>
                        <div class="upload-suggestion-values">
                            <span class="upload-current"
                                >当前：{{ display(item.field, form[item.field]) }}</span
                            >
                            <span class="upload-proposed"
                                >建议：{{ display(item.field, item.value) }}</span
                            >
                            <span class="upload-evidence">文件名依据：{{ item.evidence }}</span>
                        </div>
                    </div>
                    <AButton
                        type="primary"
                        block
                        :disabled="
                            busy || locked || !state.suggestions.some((item) => item.selected)
                        "
                        @click="session.apply(form)"
                        >应用已选建议</AButton
                    >
                </div>
                <p v-if="state.feedback" role="status" class="upload-assistant-copy">
                    {{ state.feedback }}
                </p>
            </AModal>
            <AssistantSettings
                v-model:open="settingsOpen"
                :config="state.config"
                :busy="busy"
                :save-settings="session.saveSettings"
                :clear-key="session.clearKey"
            />
        </section>
    </AssistantTheme>
</template>

<script setup>
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import {
    Alert as AAlert,
    Button as AButton,
    Checkbox as ACheckbox,
    Modal as AModal,
    Spin as ASpin
} from 'ant-design-vue'
import { RobotOutlined, SettingOutlined } from '@ant-design/icons-vue'
import useUserStore from '@/store/modules/user'
import { assistantCall, onAssistantProgress } from '@/api/assistant'
import {
    createUploadAssistantSession,
    createUploadAssistantState
} from '@/utils/upload-assistant-session.mjs'
import {
    normalizeUploadInput,
    uploadFields,
    uploadTypes
} from '../../../shared/upload-assistant.mjs'
import AssistantTheme from './AssistantTheme.vue'
import AssistantSettings from './AssistantSettings.vue'

const props = defineProps({
    file: Object,
    kind: String,
    form: { type: Object, required: true },
    locked: Boolean
})
const user = useUserStore()
const signedIn = computed(() => !!user.token && !!user.id)
const state = reactive(createUploadAssistantState())
const session = createUploadAssistantSession({
    call: assistantCall,
    subscribe: onAssistantProgress,
    state
})
const settingsOpen = ref(false)
const reviewOpen = ref(false)
const busy = computed(() => state.busy || state.settingsBusy)
const metadata = computed(() => ({
    kind: props.kind,
    folder: !!props.file?.isFolder,
    names: props.file?.isFolder
        ? (props.file.files || []).map((file) => file.name)
        : [props.file?.name]
}))
const inputError = computed(() => {
    try {
        normalizeUploadInput(metadata.value)
        return ''
    } catch (error) {
        return error.message
    }
})
const label = (field) =>
    field === 'fileName' && props.kind === 'ebook' ? '书名' : uploadFields[field]
const display = (field, value) =>
    uploadTypes[field]?.[value] ||
    (value === null || value === undefined || value === '' ? '未填写' : String(value))
watch(
    () => [user.token, String(user.id || '')],
    () => {
        settingsOpen.value = false
        reviewOpen.value = false
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
    () => [props.file, JSON.stringify(metadata.value)],
    () => {
        session.reset()
        reviewOpen.value = false
    },
    { flush: 'sync' }
)
watch(
    () => props.locked,
    (locked) => {
        if (locked) {
            session.reset()
            reviewOpen.value = false
        }
    }
)
const openSettings = async () => {
    await session.refreshConfig()
    settingsOpen.value = true
}
const generate = async () => {
    if (props.locked || inputError.value) return
    await session.refreshConfig()
    if (!state.config.hasApiKey) {
        settingsOpen.value = true
        return
    }
    await session.generate(metadata.value, props.form)
    if (state.suggestions.length) reviewOpen.value = true
}
onUnmounted(() => session.dispose())
</script>

<style scoped>
.upload-assistant {
    border: 1px solid var(--ide-border, #dcdfe6);
    border-radius: 10px;
    padding: 12px;
    margin-bottom: 20px;
    background: var(--ide-panel-bg, #fff);
    color: var(--ide-text, #606266);
    font-size: 13px;
    text-align: left;
    text-wrap: pretty;
}
.upload-assistant-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 4px;
}
.upload-assistant-heading strong {
    color: var(--ide-text-active, #303133);
}
.upload-assistant-copy,
.upload-assistant-privacy {
    margin: 8px 0;
    line-height: 1.65;
}
.upload-assistant-privacy,
.upload-current,
.upload-evidence {
    color: var(--ide-text-light, #909399);
    font-size: 12px;
}
.upload-assistant-actions {
    margin-top: 12px;
}
.upload-review-button {
    margin-top: 12px;
    white-space: normal;
    height: auto;
    min-height: 32px;
}
.upload-suggestion-list {
    display: grid;
    gap: 12px;
    margin-top: 16px;
}
.upload-suggestion-row {
    border-top: 1px solid var(--ide-border, #dcdfe6);
    padding-top: 12px;
    min-width: 0;
}
.upload-suggestion-values {
    display: grid;
    gap: 4px;
    padding: 6px 0 0 24px;
    overflow-wrap: anywhere;
    line-height: 1.6;
}
.upload-proposed {
    color: var(--ide-text-active, #303133);
}
</style>
