<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>RAG 模型配置</h1>
        <p>在线调整服务端模型端点；API Key 只写入加密文档，从不回传。</p>
      </div>
      <a-space wrap>
        <a-button :loading="loading" @click="loadConfig">重新加载</a-button>
        <a-button type="primary" :loading="saving" @click="saveConfig">保存并立即应用</a-button>
      </a-space>
    </section>

    <a-alert
      v-if="errorMessage"
      class="form-alert"
      type="error"
      show-icon
      closable
      :message="errorMessage"
      @close="errorMessage = ''"
    />
    <a-alert
      v-if="notice"
      class="form-alert"
      type="success"
      show-icon
      :message="notice"
    />
    <a-alert
      class="form-alert"
      type="warning"
      show-icon
      message="保存的 API Key 使用 AES-256-GCM 加密。请在服务端配置独立的 KADMIN_RAG_CONFIG_ENCRYPTION_KEY，并持久化模型配置文件；密钥丢失后无法解密。"
    />

    <a-card title="运行状态与存储">
      <a-descriptions bordered :column="2" size="small">
        <a-descriptions-item label="RAG 模型层">
          <a-switch v-model:checked="form.enabled" checked-children="启用" un-checked-children="停用" />
        </a-descriptions-item>
		<a-descriptions-item label="加密文档">
		  {{ form.encryptedStore ? '已保存到服务器' : '尚未保存（当前使用启动配置）' }}
		</a-descriptions-item>
        <a-descriptions-item label="Embedding 维度">
          {{ form.dimensions }}（数据库列固定为 1024 维）
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <a-row :gutter="16">
      <a-col v-for="item in providers" :key="item.key" :xs="24" :xl="12">
        <a-card class="model-card" :title="item.title">
          <p class="muted-text">{{ item.description }}</p>
          <a-form layout="vertical">
            <a-form-item label="模型名称">
              <a-input v-model:value="form[item.key].model" :placeholder="item.modelPlaceholder" />
            </a-form-item>
            <a-form-item label="Base URL">
              <a-input v-model:value="form[item.key].baseUrl" placeholder="https://api.example.com/v1" />
            </a-form-item>
            <a-form-item label="API Key">
              <a-input-password
                v-model:value="form[item.key].apiKey"
                autocomplete="new-password"
                :placeholder="form[item.key].apiKeySet ? '已配置；留空保持原密钥' : '输入 API Key'"
              />
              <a-checkbox v-model:checked="form[item.key].clearApiKey" :disabled="!form[item.key].apiKeySet">
                清除已保存的密钥
              </a-checkbox>
            </a-form-item>
            <a-form-item label="请求超时（秒）">
              <a-input-number v-model:value="form[item.key].timeoutSeconds" :min="1" :max="600" />
            </a-form-item>
            <a-tag v-if="form[item.key].apiKeySet" color="green">API Key 已加密保存</a-tag>
            <a-tag v-else color="default">尚未设置 API Key</a-tag>
          </a-form>
        </a-card>
      </a-col>
    </a-row>

    <section class="panel muted-text">
      OCR 使用独立的视觉语言模型；Vision 配置供视觉理解调用。Rerank 字段保存端点配置，当前检索管线尚未接入重排调用。
      密钥留空表示保留已保存的密钥；勾选“清除”才会删除。
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { message } from 'ant-design-vue';
import {
  getRagModelConfig,
  updateRagModelConfig,
  type RagModelConfig,
  type RagModelConfigInput,
  type RagProviderConfig,
} from '../../../api/kadmin/ragModels';

type ProviderKey = 'chat' | 'embedding' | 'vision' | 'ocr' | 'rerank';
type ProviderForm = RagProviderConfig & { apiKey: string; clearApiKey: boolean };
type ModelForm = Omit<RagModelConfig, ProviderKey | 'encryptedStore'> & Record<ProviderKey, ProviderForm> & { encryptedStore: boolean };

const form = reactive<ModelForm>({
  enabled: false,
  dimensions: 1024,
  encryptedStore: false,
  chat: { model: '', baseUrl: '', apiKeySet: false, apiKey: '', clearApiKey: false, timeoutSeconds: 60 },
  embedding: { model: '', baseUrl: '', apiKeySet: false, apiKey: '', clearApiKey: false, timeoutSeconds: 30 },
  vision: { model: '', baseUrl: '', apiKeySet: false, apiKey: '', clearApiKey: false, timeoutSeconds: 60 },
  ocr: { model: '', baseUrl: '', apiKeySet: false, apiKey: '', clearApiKey: false, timeoutSeconds: 60 },
  rerank: { model: '', baseUrl: '', apiKeySet: false, apiKey: '', clearApiKey: false, timeoutSeconds: 60 },
});

const providers: { key: ProviderKey; title: string; description: string; modelPlaceholder: string }[] = [
  { key: 'chat', title: 'Chat / 审核', description: 'Eino ChatModel；用于问答、审核和 Agent 对话。', modelPlaceholder: '例如 gpt-4.1-mini' },
  { key: 'embedding', title: 'Embedding', description: '向量化模型；数据库当前要求输出 1024 维。', modelPlaceholder: '例如 text-embedding-3-large' },
  { key: 'vision', title: 'Vision', description: 'Eino 视觉语言模型，可用于图像理解。', modelPlaceholder: '例如 gpt-4.1-mini' },
  { key: 'ocr', title: 'OCR', description: '独立的 Eino 视觉语言模型，用于 OCR/试卷识别。', modelPlaceholder: '例如 gpt-4.1-mini' },
  { key: 'rerank', title: 'Rerank（可选）', description: '保存兼容端点配置；重排调用尚未接入当前检索管线。', modelPlaceholder: '例如 rerank-v3.5' },
];

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref('');
const notice = ref('');

onMounted(() => {
  void loadConfig();
});

async function loadConfig() {
  loading.value = true;
  errorMessage.value = '';
  notice.value = '';
  try {
    const result = await getRagModelConfig();
    form.enabled = result.enabled;
    form.dimensions = result.dimensions || 1024;
    form.encryptedStore = result.encryptedStore;
    for (const key of ['chat', 'embedding', 'vision', 'ocr', 'rerank'] as const) {
      Object.assign(form[key], result[key], { apiKey: '', clearApiKey: false });
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '加载模型配置失败';
  } finally {
    loading.value = false;
  }
}

async function saveConfig() {
  saving.value = true;
  errorMessage.value = '';
  notice.value = '';
  const payload: RagModelConfigInput = {
    enabled: form.enabled,
    dimensions: form.dimensions,
    chat: { model: form.chat.model, baseUrl: form.chat.baseUrl, apiKey: form.chat.apiKey, clearApiKey: form.chat.clearApiKey, timeoutSeconds: form.chat.timeoutSeconds },
    embedding: { model: form.embedding.model, baseUrl: form.embedding.baseUrl, apiKey: form.embedding.apiKey, clearApiKey: form.embedding.clearApiKey, timeoutSeconds: form.embedding.timeoutSeconds },
    vision: { model: form.vision.model, baseUrl: form.vision.baseUrl, apiKey: form.vision.apiKey, clearApiKey: form.vision.clearApiKey, timeoutSeconds: form.vision.timeoutSeconds },
    ocr: { model: form.ocr.model, baseUrl: form.ocr.baseUrl, apiKey: form.ocr.apiKey, clearApiKey: form.ocr.clearApiKey, timeoutSeconds: form.ocr.timeoutSeconds },
    rerank: { model: form.rerank.model, baseUrl: form.rerank.baseUrl, apiKey: form.rerank.apiKey, clearApiKey: form.rerank.clearApiKey, timeoutSeconds: form.rerank.timeoutSeconds },
  };
  try {
    const result = await updateRagModelConfig(payload);
    form.enabled = result.enabled;
    form.dimensions = result.dimensions || 1024;
    form.encryptedStore = result.encryptedStore;
    for (const key of ['chat', 'embedding', 'vision', 'ocr', 'rerank'] as const) {
      Object.assign(form[key], result[key], { apiKey: '', clearApiKey: false });
    }
    notice.value = '配置已加密保存并立即应用到在线模型运行时。';
    message.success('RAG 模型配置已更新');
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '保存模型配置失败';
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.model-card {
  margin-bottom: 16px;
}

.muted-text {
  color: var(--kadmin-muted, #8c8c8c);
}
</style>
