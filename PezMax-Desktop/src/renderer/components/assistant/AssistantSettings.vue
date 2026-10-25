<template>
    <AModal
        :open="open"
        title="模型设置"
        :width="560"
        :mask-closable="false"
        :closable="!busy"
        :keyboard="!busy"
        :destroy-on-close="true"
        @cancel="close"
    >
        <p class="settings-intro">
            连接你自己的模型服务，即可使用找资料、上传建议与 PDF 阅读问答。
        </p>
        <AForm layout="vertical" class="assistant-settings" @finish="save(false)">
            <AFormItem
                label="接口地址"
                :required="true"
                extra="填写兼容 Chat Completions 的基础地址，例如 https://服务商域名/v1。"
            >
                <AInput
                    v-model:value="form.baseUrl"
                    aria-label="接口地址"
                    placeholder="https://服务商域名/v1"
                    :maxlength="2048"
                    :disabled="busy"
                    autocomplete="off"
                />
            </AFormItem>
            <AFormItem
                label="模型 ID"
                :required="true"
                extra="使用服务商提供的模型 ID；找资料需支持工具调用，上传建议与阅读问答需按要求返回 JSON。"
            >
                <AInput
                    v-model:value="form.model"
                    aria-label="模型 ID"
                    placeholder="填写支持工具调用的模型 ID"
                    :maxlength="160"
                    :disabled="busy"
                    autocomplete="off"
                />
            </AFormItem>
            <AFormItem
                label="API Key"
                :required="!config.hasApiKey"
                :extra="
                    config.hasApiKey
                        ? '已配置密钥。地址不变时，留空可继续使用；更换服务商地址需重新输入。'
                        : '密钥仅用于本机直连你填写的模型服务，不经过 PezMax 服务器。'
                "
            >
                <AInputPassword
                    v-model:value="form.apiKey"
                    aria-label="API Key"
                    :placeholder="config.hasApiKey ? '已配置 · 留空保留' : '输入你的 API Key'"
                    :maxlength="4096"
                    :disabled="busy"
                    autocomplete="new-password"
                />
            </AFormItem>
            <AFormItem>
                <ACheckbox v-model:checked="form.remember" :disabled="busy || !config.canRemember"
                    >加密保存在本机</ACheckbox
                >
                <p class="settings-hint">
                    {{
                        config.canRemember
                            ? '默认仅在当前登录会话使用。勾选后由操作系统加密保存，当前账号下次可直接使用。'
                            : '当前系统不支持安全保存，密钥仅在当前登录会话使用。'
                    }}
                </p>
            </AFormItem>
            <AAlert
                type="info"
                show-icon
                message="发送范围与费用"
                description="找资料发送提问、近期对话和目录信息；上传建议仅发送文件名；阅读问答在你发送问题时发送问题和所选 PDF 页面的相关文字片段。不发送本地路径或下载地址。调用费用由服务商按你的账号计费，连接测试也会发起一次调用。"
            />
            <AAlert
                v-if="feedback"
                :type="feedbackType"
                show-icon
                :message="feedback"
                class="settings-feedback"
                role="status"
            />
        </AForm>
        <template #footer>
            <div class="settings-footer">
                <AButton danger type="text" :disabled="busy || !config.hasApiKey" @click="clear"
                    >移除密钥</AButton
                >
                <div class="settings-actions">
                    <AButton :disabled="busy" @click="close">关闭</AButton>
                    <AButton :disabled="busy" @click="save(false)">保存</AButton>
                    <AButton type="primary" :loading="busy" @click="save(true)">保存并测试</AButton>
                </div>
            </div>
        </template>
    </AModal>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import {
    Alert as AAlert,
    Button as AButton,
    Checkbox as ACheckbox,
    Form as AForm,
    FormItem as AFormItem,
    Input as AInput,
    InputPassword as AInputPassword,
    Modal as AModal
} from 'ant-design-vue'

const props = defineProps({
    open: Boolean,
    config: { type: Object, required: true },
    busy: Boolean,
    saveSettings: { type: Function, required: true },
    clearKey: { type: Function, required: true }
})
const emit = defineEmits(['update:open'])
const form = reactive({ baseUrl: '', model: '', apiKey: '', remember: false })
const feedback = ref('')
const feedbackType = ref('success')
let version = 0
watch(
    () => props.open,
    (open) => {
        version++
        form.apiKey = ''
        feedback.value = ''
        if (open)
            Object.assign(form, {
                baseUrl: props.config.baseUrl,
                model: props.config.model,
                remember: props.config.remember
            })
    },
    { immediate: true }
)

const close = () => {
    if (props.busy) return
    form.apiKey = ''
    emit('update:open', false)
}
const save = async (test) => {
    if (props.busy) return
    const current = version
    feedback.value = ''
    try {
        const message = await props.saveSettings({ ...form }, test)
        if (current !== version) return
        form.apiKey = ''
        feedbackType.value = 'success'
        feedback.value = message
        if (!test) close()
    } catch (error) {
        if (current !== version) return
        // A successful save followed by a failed connection still retains the key.
        if (props.config.hasApiKey) form.apiKey = ''
        feedbackType.value = 'error'
        feedback.value = error.message || '保存失败，请重试'
    }
}
const clear = async () => {
    const current = version
    try {
        await props.clearKey()
        if (current !== version) return
        form.apiKey = ''
        form.remember = false
        feedbackType.value = 'success'
        feedback.value = '密钥已从本次会话和本机保存记录中移除。'
    } catch (error) {
        if (current !== version) return
        feedbackType.value = 'error'
        feedback.value = error.message || '移除失败，请重试'
    }
}
</script>

<style scoped>
.settings-intro {
    margin: 8px 0 24px;
    color: var(--ide-text, #606266);
    line-height: 1.7;
}
.settings-hint {
    margin: 8px 0 0;
    color: var(--ide-text-light, #909399);
    font-size: 12px;
    line-height: 1.6;
}
.settings-feedback {
    margin-top: 16px;
}
.settings-footer,
.settings-actions {
    display: flex;
    gap: 8px;
    align-items: center;
}
.settings-footer {
    justify-content: space-between;
    flex-wrap: wrap;
    margin-top: 24px;
}
.assistant-settings :deep(.ant-form-item-extra) {
    margin-top: 6px;
    font-size: 12px;
}
</style>
