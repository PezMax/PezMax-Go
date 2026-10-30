<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>RAG 模型配置</h1>
        <p>
          左侧选择模型分工，进入该分工的模型列表；每个分工可保存多个模型，同一时刻只有一个启用，API Key 只写入加密文档，从不回传。
        </p>
      </div>
      <a-space wrap>
        <a-tag v-if="hasUnsavedChanges" color="orange">有未保存修改</a-tag>
        <a-button :loading="loading" @click="loadConfig">重新加载</a-button>
        <a-button type="primary" :loading="saving" @click="saveConfig">
          保存并立即应用
        </a-button>
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

    <a-row :gutter="[16, 16]">
      <a-col :xs="24" :lg="8">
        <section class="panel">
          <div class="panel-title">
            <h2>模型分工</h2>
          </div>

          <div class="runtime-status">
            <div class="runtime-row">
              <span>RAG 模型层</span>
              <a-switch
                v-model:checked="enabled"
                checked-children="启用"
                un-checked-children="停用"
              />
            </div>
            <div class="runtime-row">
              <span>加密文档</span>
              <span class="muted-text">{{ storeText }}</span>
            </div>
            <div class="runtime-row">
              <span>Embedding 维度</span>
              <span class="muted-text">{{ dimensions }}（数据库列固定 1024 维）</span>
            </div>
          </div>

          <a-list
            class="role-list model-list"
            :data-source="kindRows"
            :loading="loading"
            item-layout="horizontal"
          >
            <template #renderItem="{ item }">
              <a-list-item
                :class="[
                  'role-list-item',
                  { active: item.kind === selectedKind },
                ]"
                @click="selectKind(item.kind)"
              >
                <a-list-item-meta>
                  <template #avatar>
                    <a-avatar
                      :style="{
                        backgroundColor: item.activeModel
                          ? 'hsl(var(--primary))'
                          : 'hsl(var(--muted-foreground))',
                      }"
                    >
                      {{ kindAvatar(item) }}
                    </a-avatar>
                  </template>
                  <template #title>
                    <a-space :size="6" wrap>
                      <span>{{ kindLabel(item) }}</span>
                      <a-tag v-if="item.activeModel" color="green">启用中</a-tag>
                      <a-tag>{{ item.count }}</a-tag>
                    </a-space>
                  </template>
                  <template #description>
                    <span v-if="item.activeModel">
                      {{ item.activeModel }}
                    </span>
                    <span v-else class="muted-text">未启用模型</span>
                  </template>
                </a-list-item-meta>
              </a-list-item>
            </template>
          </a-list>
        </section>
      </a-col>

      <a-col :xs="24" :lg="16">
        <section class="panel">
          <div class="panel-title">
            <div>
              <h2>{{ kindLabels[selectedKind] }}</h2>
              <p class="muted-text">{{ kindDescriptions[selectedKind] }}</p>
            </div>
            <a-space wrap>
              <a-button type="primary" @click="createModel">
                <PlusOutlined />
                新增模型
              </a-button>
              <template v-if="selectedModel">
                <a-button
                  v-if="!selectedModel.active"
                  @click="activateModel(selectedModel)"
                >
                  启用该模型
                </a-button>
                <a-popconfirm
                  v-else
                  title="停用后该分工将没有启用模型，确认停用？"
                  @confirm="selectedModel.active = false"
                >
                  <a-button>停用</a-button>
                </a-popconfirm>
                <a-popconfirm
                  title="确认删除该模型配置？"
                  @confirm="removeModel(selectedModel)"
                >
                  <a-button danger>删除</a-button>
                </a-popconfirm>
              </template>
            </a-space>
          </div>

          <a-empty
            v-if="kindModels.length === 0"
            class="model-empty"
            :description="`${kindLabels[selectedKind]} 暂无模型，点击“新增模型”创建`"
          />

          <a-tabs
            v-else
            v-model:active-key="activeTabKey"
            type="card"
            class="model-tabs"
          >
            <a-tab-pane
              v-for="model in kindModels"
              :key="model.clientKey"
            >
              <template #tab>
                <a-space :size="4">
                  <span>{{ displayName(model) }}</span>
                  <a-tag v-if="model.active" color="green">启用</a-tag>
                  <a-tag v-if="isDirty(model)" color="orange">未保存</a-tag>
                </a-space>
              </template>

              <a-form layout="vertical" class="model-form">
                <a-row :gutter="16">
                  <a-col :xs="24" :md="12">
                    <a-form-item label="备注名称">
                      <a-input
                        v-model:value="model.name"
                        :placeholder="`例如 ${kindLabels[selectedKind]} 主模型`"
                      />
                    </a-form-item>
                  </a-col>
                  <a-col :xs="24" :md="12">
                    <a-form-item label="模型名称" required>
                      <a-input
                        v-model:value="model.model"
                        :placeholder="modelPlaceholders[selectedKind]"
                      />
                    </a-form-item>
                  </a-col>
                  <a-col :xs="24" :md="12">
                    <a-form-item label="请求超时（秒）">
                      <a-input-number
                        v-model:value="model.timeoutSeconds"
                        class="control-md"
                        :min="1"
                        :max="600"
                      />
                    </a-form-item>
                  </a-col>
                  <a-col :xs="24" :md="12">
                    <a-form-item label="Base URL" required>
                      <a-input
                        v-model:value="model.baseUrl"
                        placeholder="https://api.example.com/v1"
                      />
                    </a-form-item>
                  </a-col>
                  <a-col :span="24">
                    <a-form-item label="API Key">
                      <a-input-password
                        v-model:value="model.apiKey"
                        autocomplete="new-password"
                        :placeholder="
                          model.apiKeySet
                            ? '已加密保存，留空保持原密钥'
                            : '输入 API Key，保存后单向加密、永不回传'
                        "
                      />
                      <a-checkbox
                        v-model:checked="model.clearApiKey"
                        class="clear-key"
                        :disabled="!model.apiKeySet"
                      >
                        清除已保存的密钥
                      </a-checkbox>
                    </a-form-item>
                  </a-col>
                  <a-col v-if="model.kind === 'embedding'" :span="24">
                    <a-form-item label="向量维度">
                      <a-input-number
                        class="control-md"
                        :value="dimensions"
                        disabled
                      />
                      <span class="muted-text dimension-hint">
                        依据 rag_chunk.embedding vector(1024) 固定
                      </span>
                    </a-form-item>
                  </a-col>
                </a-row>

                <div class="model-tags">
                  <a-tag :color="model.apiKeySet ? 'green' : 'default'">
                    {{
                      model.apiKeySet ? 'API Key 已加密保存' : '尚未设置 API Key'
                    }}
                  </a-tag>
                  <a-tag v-if="model.active" color="blue">运行时启用</a-tag>
                  <a-tag v-if="isDirty(model)" color="orange">未保存修改</a-tag>
                  <a-tag v-if="!model.id" color="purple">新建未保存</a-tag>
                </div>
              </a-form>
            </a-tab-pane>
          </a-tabs>
        </section>
      </a-col>
    </a-row>

    <section class="panel muted-text">
      OCR 使用独立的视觉语言模型；Vision 配置供视觉理解调用。Rerank
      字段保存端点配置，当前检索管线尚未接入重排调用。 每种分工同一时刻只启用一个模型，其余配置仅保存备用；密钥留空表示保留已保存的密钥，勾选“清除”才会删除。
    </section>
  </div>
</template>

<script setup lang="ts">
import { PlusOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import { computed, onMounted, ref } from 'vue';

import {
  getRagModelConfig,
  updateRagModelConfig,
} from '#/api/kadmin/ragModels';
import type {
  RagModelConfig,
  RagModelConfigInput,
  RagModelInput,
  RagModelItem,
  RagModelKind,
} from '#/api/kadmin/ragModels';

type ModelDraft = RagModelItem & {
  clientKey: string;
  apiKey: string;
  clearApiKey: boolean;
};

const kindOrder: RagModelKind[] = [
  'chat',
  'embedding',
  'vision',
  'ocr',
  'rerank',
];
const kindLabels: Record<RagModelKind, string> = {
  chat: 'Chat / 审核',
  embedding: 'Embedding',
  vision: 'Vision',
  ocr: 'OCR',
  rerank: 'Rerank（可选）',
};
const kindAvatars: Record<RagModelKind, string> = {
  chat: '聊',
  embedding: '嵌',
  vision: '视',
  ocr: '识',
  rerank: '排',
};
const kindDescriptions: Record<RagModelKind, string> = {
  chat: 'Eino ChatModel；用于问答、审核和 Agent 对话。',
  embedding: '向量化模型；数据库当前要求输出 1024 维。',
  vision: 'Eino 视觉语言模型，可用于图像理解。',
  ocr: '独立的 Eino 视觉语言模型，用于 OCR/试卷识别。',
  rerank: '保存兼容端点配置；重排调用尚未接入当前检索管线。',
};
const modelPlaceholders: Record<RagModelKind, string> = {
  chat: '例如 gpt-4.1-mini',
  embedding: '例如 text-embedding-3-large',
  vision: '例如 gpt-4.1-mini',
  ocr: '例如 gpt-4.1-mini',
  rerank: '例如 rerank-v3.5',
};
const defaultTimeouts: Record<RagModelKind, number> = {
  chat: 60,
  embedding: 30,
  vision: 60,
  ocr: 60,
  rerank: 60,
};

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref('');
const notice = ref('');

const enabled = ref(false);
const dimensions = ref(1024);
const encryptedStore = ref(false);
const models = ref<ModelDraft[]>([]);
const selectedKind = ref<RagModelKind>('chat');
const activeTabKey = ref('');
const signatures = ref<Record<string, string>>({});
const savedRuntime = ref({ enabled: false, dimensions: 1024 });

let draftCounter = 0;

const kindRows = computed(() =>
  kindOrder.map((kind) => {
    const list = models.value.filter((item) => item.kind === kind);
    const active = list.find((item) => item.active);
    return {
      kind,
      count: list.length,
      activeModel: active ? displayName(active) : '',
    };
  }),
);
const kindModels = computed(() => models.value.filter(
  (item) => item.kind === selectedKind.value,
));
const selectedModel = computed(
  () =>
    kindModels.value.find((item) => item.clientKey === activeTabKey.value) ??
    null,
);
const storeText = computed(() =>
  encryptedStore.value ? '已保存到服务器' : '尚未保存（当前使用启动配置）',
);
const hasUnsavedChanges = computed(() => {
  if (
    savedRuntime.value.enabled !== enabled.value ||
    savedRuntime.value.dimensions !== dimensions.value
  ) {
    return true;
  }
  return models.value.some((item) => isDirty(item));
});

onMounted(() => {
  void loadConfig();
});

function draftSignature(model: ModelDraft) {
  return JSON.stringify({
    kind: model.kind,
    name: model.name,
    model: model.model,
    baseUrl: model.baseUrl,
    apiKey: model.apiKey,
    clearApiKey: model.clearApiKey,
    timeoutSeconds: model.timeoutSeconds,
    active: model.active,
  });
}

function isDirty(model: ModelDraft) {
  return signatures.value[model.clientKey] !== draftSignature(model);
}

function displayName(model: ModelDraft) {
  return model.name.trim() || model.model || kindLabels[model.kind];
}

function kindLabel(model: { kind: RagModelKind }) {
  return kindLabels[model.kind];
}

function kindAvatar(model: { kind: RagModelKind }) {
  return kindAvatars[model.kind];
}

function toDraft(item: RagModelItem, index: number): ModelDraft {
  const id = item.id || `draft-${index}`;
  return {
    ...item,
    clientKey: `model-${index}-${id}`,
    apiKey: '',
    clearApiKey: false,
  };
}

function selectKind(kind: RagModelKind) {
  selectedKind.value = kind;
  activeTabKey.value =
    models.value.find((item) => item.kind === kind)?.clientKey ?? '';
}

function applyConfig(config: RagModelConfig) {
  const previousKind = selectedKind.value;
  const previousIndex = models.value
    .filter((item) => item.kind === previousKind)
    .findIndex((item) => item.clientKey === activeTabKey.value);
  enabled.value = config.enabled;
  dimensions.value = config.dimensions || 1024;
  encryptedStore.value = config.encryptedStore;
  models.value = (config.models ?? []).map((item, index) =>
    toDraft(item, index),
  );
  const nextSignatures: Record<string, string> = {};
  for (const model of models.value) {
    nextSignatures[model.clientKey] = draftSignature(model);
  }
  signatures.value = nextSignatures;
  savedRuntime.value = { enabled: enabled.value, dimensions: dimensions.value };
  const list = models.value.filter((item) => item.kind === previousKind);
  const target =
    previousIndex !== -1 && previousIndex < list.length ? previousIndex : 0;
  activeTabKey.value = list[target]?.clientKey ?? '';
}

async function loadConfig() {
  loading.value = true;
  errorMessage.value = '';
  notice.value = '';
  try {
    applyConfig(await getRagModelConfig());
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : '加载模型配置失败';
  } finally {
    loading.value = false;
  }
}

function createModel() {
  draftCounter += 1;
  const kind = selectedKind.value;
  const draft: ModelDraft = {
    id: '',
    kind,
    name: '',
    model: '',
    baseUrl: '',
    apiKey: '',
    clearApiKey: false,
    apiKeySet: false,
    timeoutSeconds: defaultTimeouts[kind],
    active: false,
    clientKey: `draft-${draftCounter}`,
  };
  models.value.push(draft);
  activeTabKey.value = draft.clientKey;
}

function activateModel(model: ModelDraft) {
  for (const other of models.value) {
    if (other.kind === model.kind) {
      other.active = false;
    }
  }
  model.active = true;
}

function removeModel(model: ModelDraft) {
  const index = models.value.findIndex(
    (item) => item.clientKey === model.clientKey,
  );
  if (index === -1) {
    return;
  }
  models.value.splice(index, 1);
  const { [model.clientKey]: _removed, ...rest } = signatures.value;
  signatures.value = rest;
  if (activeTabKey.value === model.clientKey) {
    activeTabKey.value =
      models.value.find((item) => item.kind === model.kind)?.clientKey ?? '';
  }
}

function validateDraft(): string {
  const missing = models.value.find(
    (item) => !item.model.trim() || !item.baseUrl.trim(),
  );
  if (missing) {
    const index = models.value.indexOf(missing);
    return `第 ${index + 1} 个模型（${kindLabels[missing.kind]}）缺少模型名称或 Base URL`;
  }
  if (enabled.value && !models.value.some((item) => item.active)) {
    return '启用 RAG 模型层前需要至少启用一个模型';
  }
  return '';
}

async function saveConfig() {
  const invalid = validateDraft();
  if (invalid) {
    message.warning(invalid);
    return;
  }
  const payload: RagModelConfigInput = {
    enabled: enabled.value,
    dimensions: dimensions.value,
    models: models.value.map(
      (item): RagModelInput => ({
        id: item.id,
        kind: item.kind,
        name: item.name.trim(),
        model: item.model.trim(),
        baseUrl: item.baseUrl.trim(),
        apiKey: item.apiKey,
        clearApiKey: item.clearApiKey,
        timeoutSeconds: item.timeoutSeconds,
        active: item.active,
      }),
    ),
  };
  saving.value = true;
  errorMessage.value = '';
  notice.value = '';
  try {
    applyConfig(await updateRagModelConfig(payload));
    notice.value = '配置已加密保存并立即应用到在线模型运行时。';
    message.success('RAG 模型配置已更新');
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : '保存模型配置失败';
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.form-alert {
  margin-bottom: 16px;
}

.runtime-status {
  display: grid;
  gap: 8px;
  margin-bottom: 12px;
  padding: 10px 12px;
  background: var(--kadmin-soft-bg);
  border: 1px solid var(--kadmin-border);
  border-radius: 8px;
}

.runtime-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.model-list {
  max-height: 520px;
}

.model-empty {
  margin: 32px 0;
}

.model-tabs :deep(.ant-tabs-tab) {
  max-width: 220px;
}

.model-tabs :deep(.ant-tabs-tab-btn) {
  overflow: hidden;
  text-overflow: ellipsis;
}

.clear-key {
  margin-top: 8px;
}

.dimension-hint {
  margin-left: 8px;
  font-size: 12px;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.muted-text {
  color: var(--kadmin-muted, #8c8c8c);
}

@media (max-width: 768px) {
  .runtime-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 4px;
  }
}
</style>
