<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>代码生成</h1>
        <p>选择 PostgreSQL 业务表，一键生成完整的前后端 CRUD 页面</p>
      </div>
      <a-space wrap>
        <a-button :loading="loading" @click="fetchConfigs">
          <ReloadOutlined />
          刷新
        </a-button>
        <a-button v-if="canImport" type="primary" @click="openImport">
          <PlusOutlined />
          导入表
        </a-button>
      </a-space>
    </section>

    <a-alert
      v-if="errorText"
      class="form-alert"
      type="error"
      show-icon
      closable
      :message="errorText"
      @close="errorText = ''"
    />
    <a-alert
      v-if="generateNote"
      class="form-alert"
      type="success"
      show-icon
      closable
      :message="generateNote"
      @close="generateNote = ''"
    />

    <section class="panel">
      <div class="table-toolbar">
        <a-space wrap>
          <a-input
            v-model:value="keyword"
            allow-clear
            class="control-md"
            placeholder="按表名或业务名搜索"
            @press-enter="search"
          >
            <template #prefix><SearchOutlined /></template>
          </a-input>
          <a-button type="primary" @click="search"><SearchOutlined />查询</a-button>
          <a-button @click="resetSearch"><ClearOutlined />重置</a-button>
        </a-space>
      </div>
      <a-table
        row-key="id"
        class="compact-user-table"
        size="small"
        :columns="configColumns"
        :data-source="configs"
        :loading="loading"
        :pagination="false"
        :scroll="{ x: 1100 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'tableName'">
            <strong>{{ record.tableName }}</strong>
            <a-tag v-if="record.generated" color="green">已生成</a-tag>
          </template>
          <template v-else-if="column.key === 'businessName'">
            {{ record.businessName }}
          </template>
          <template v-else-if="column.key === 'module'">
            <a-typography-text class="job-code">
              {{ record.moduleName }} / {{ record.className }} / {{ record.routePrefix }}
            </a-typography-text>
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space :size="2">
              <a-tooltip v-if="canImport" title="配置">
                <a-button type="text" shape="circle" @click="openConfig(record)">
                  <SettingOutlined />
                </a-button>
              </a-tooltip>
              <a-tooltip v-if="canGenerate" title="预览">
                <a-button type="text" shape="circle" :loading="previewLoading === record.id" @click="openPreview(record)">
                  <EyeOutlined />
                </a-button>
              </a-tooltip>
              <a-tooltip v-if="canGenerate" title="下载">
                <a-button type="text" shape="circle" :loading="downloadLoading === record.id" @click="downloadCode(record)">
                  <DownloadOutlined />
                </a-button>
              </a-tooltip>
              <a-popconfirm
                v-if="canGenerate"
                title="确认生成代码到项目目录？"
                @confirm="generate(record, false)"
              >
                <a-tooltip title="生成到项目">
                  <a-button type="text" shape="circle" :loading="generating === record.id">
                    <ThunderboltOutlined />
                  </a-button>
                </a-tooltip>
              </a-popconfirm>
              <a-popconfirm
                v-if="canImport"
                title="仅删除生成配置，不删除已生成的文件。确认删除？"
                @confirm="removeConfig(record)"
              >
                <a-tooltip title="删除配置">
                  <a-button type="text" shape="circle" danger>
                    <DeleteOutlined />
                  </a-button>
                </a-tooltip>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-modal
      :open="importOpen"
      title="导入数据库表"
      :confirm-loading="importing"
      :mask-closable="false"
      @ok="submitImport"
      @cancel="importOpen = false"
    >
      <a-form layout="vertical">
        <a-form-item label="数据表" required>
          <a-select
            v-model:value="importForm.tableName"
            show-search
            :filter-option="false"
            :options="candidates.map((item) => ({ label: item.name, value: item.name }))"
            placeholder="选择 PostgreSQL 业务表"
            @search="searchCandidates"
          />
        </a-form-item>
        <a-form-item label="模块名（默认按表名推导）">
          <a-input v-model:value="importForm.moduleName" placeholder="如 product" />
        </a-form-item>
        <a-form-item label="类名（默认推导）">
          <a-input v-model:value="importForm.className" placeholder="如 Product" />
        </a-form-item>
        <a-form-item label="业务名（默认推导）">
          <a-input v-model:value="importForm.businessName" placeholder="如 产品" />
        </a-form-item>
        <a-form-item label="路由前缀（默认推导）">
          <a-input v-model:value="importForm.routePrefix" placeholder="如 product" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-drawer
      :open="previewOpen"
      title="生成预览"
      width="860"
      @close="previewOpen = false"
    >
      <a-spin :spinning="previewLoading !== null">
        <a-tabs v-if="previewArtifacts.length > 0" v-model:active-key="previewTab">
          <a-tab-pane
            v-for="artifact in previewArtifacts"
            :key="artifact.path"
            :tab="artifact.path.split('/').pop()"
          >
            <pre class="code-preview">{{ artifact.content }}</pre>
          </a-tab-pane>
        </a-tabs>
        <a-empty v-else-if="previewLoading === null" description="暂无预览内容" />
      </a-spin>
    </a-drawer>

    <a-modal
      :open="conflictOpen"
      title="存在冲突文件"
      ok-text="确认覆盖"
      ok-type="danger"
      @ok="confirmOverwrite"
      @cancel="conflictOpen = false"
    >
      <p>以下文件已存在且没有代码生成标记，可能包含手工修改：</p>
      <ul>
        <li v-for="path in conflictPaths" :key="path" class="conflict-path">{{ path }}</li>
      </ul>
      <p>确认后将使用生成内容覆盖这些文件。</p>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

import {
  deleteCodegenTable,
  generateCodegenTable,
  getCodegenCandidates,
  getCodegenTables,
  importCodegenTable,
  previewCodegenTable,
  type CodegenArtifact,
  type CodegenCandidate,
  type CodegenTableConfig,
} from '#/api/kadmin/codegen';
import { requestBlob } from '#/api/kadmin/client';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

const { hasAccessByCodes } = useAccess();
const canImport = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.CODEGEN_IMPORT, '*']),
);
const canGenerate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.CODEGEN_GENERATE, '*']),
);

const router = useRouter();

const loading = ref(false);
const errorText = ref('');
const generateNote = ref('');
const keyword = ref('');
const configs = ref<CodegenTableConfig[]>([]);

const configColumns = [
  { title: '数据表', key: 'tableName', width: 220, fixed: 'left' },
  { title: '业务名', key: 'businessName', width: 160 },
  { title: '模块 / 类 / 前缀', key: 'module', width: 320 },
  { title: '操作', key: 'action', width: 190, fixed: 'right' },
];

async function fetchConfigs() {
  loading.value = true;
  errorText.value = '';
  try {
    configs.value = await getCodegenTables({ keyword: keyword.value });
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : '加载失败';
  } finally {
    loading.value = false;
  }
}

function search() {
  void fetchConfigs();
}

function resetSearch() {
  keyword.value = '';
  void fetchConfigs();
}

const importOpen = ref(false);
const importing = ref(false);
const candidates = ref<CodegenCandidate[]>([]);
const importForm = reactive({
  businessName: '',
  className: '',
  moduleName: '',
  routePrefix: '',
  tableName: '',
});

function openImport() {
  importForm.businessName = '';
  importForm.className = '';
  importForm.moduleName = '';
  importForm.routePrefix = '';
  importForm.tableName = '';
  candidates.value = [];
  importOpen.value = true;
  void searchCandidates('');
}

async function searchCandidates(keywordValue: string) {
  try {
    candidates.value = await getCodegenCandidates({ keyword: keywordValue });
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载表列表失败');
  }
}

async function submitImport() {
  if (!importForm.tableName) {
    message.warning('请选择数据表');
    return;
  }
  importing.value = true;
  try {
    const config = await importCodegenTable({
      businessName: importForm.businessName || undefined,
      className: importForm.className || undefined,
      moduleName: importForm.moduleName || undefined,
      routePrefix: importForm.routePrefix || undefined,
      tableName: importForm.tableName,
    });
    message.success(`已导入表 ${config.tableName}`);
    importOpen.value = false;
    await fetchConfigs();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '导入失败');
  } finally {
    importing.value = false;
  }
}

function openConfig(record: CodegenTableConfig) {
  void router.push({
    path: '/kadmin/codegen-config',
    query: { id: String(record.id) },
  });
}

const previewOpen = ref(false);
const previewLoading = ref<null | number>(null);
const previewArtifacts = ref<CodegenArtifact[]>([]);
const previewTab = ref('');

async function openPreview(record: CodegenTableConfig) {
  previewOpen.value = true;
  previewLoading.value = record.id;
  previewArtifacts.value = [];
  try {
    const result = await previewCodegenTable(record.id);
    previewArtifacts.value = result.artifacts;
    previewTab.value = result.artifacts[0]?.path ?? '';
  } catch (error) {
    message.error(error instanceof Error ? error.message : '预览失败');
    previewOpen.value = false;
  } finally {
    previewLoading.value = null;
  }
}

const downloadLoading = ref<null | number>(null);

async function downloadCode(record: CodegenTableConfig) {
  downloadLoading.value = record.id;
  try {
    const blob = await requestBlob(`/api/codegen/configs/${record.id}/download`);
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = `${record.moduleName}-codegen.zip`;
    anchor.click();
    URL.revokeObjectURL(url);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '下载失败');
  } finally {
    downloadLoading.value = null;
  }
}

const generating = ref<null | number>(null);
const conflictOpen = ref(false);
const conflictPaths = ref<string[]>([]);
let conflictConfigId: null | number = null;

async function generate(record: CodegenTableConfig, confirmOverwrite: boolean) {
  generating.value = record.id;
  generateNote.value = '';
  try {
    const result = await generateCodegenTable(record.id, { confirmOverwrite });
    generateNote.value = `${result.note} 写入 ${result.written.length} 个文件，覆盖 ${result.overwritten.length} 个文件。`;
    message.success('生成成功');
    await fetchConfigs();
  } catch (error) {
    const text = error instanceof Error ? error.message : '生成失败';
    if (text.includes('codegen marker')) {
      conflictConfigId = record.id;
      conflictPaths.value = [];
      conflictOpen.value = true;
      try {
        const preview = await previewCodegenTable(record.id);
        for (const path of preview.artifacts.map((item) => item.path)) {
          conflictPaths.value.push(path);
        }
      } catch {
        // 预览失败时仍允许按提示确认覆盖。
      }
    } else {
      message.error(text);
    }
  } finally {
    generating.value = null;
  }
}

async function confirmOverwrite() {
  conflictOpen.value = false;
  const record = configs.value.find((item) => item.id === conflictConfigId);
  if (record) {
    await generate(record, true);
  }
}

async function removeConfig(record: CodegenTableConfig) {
  try {
    await deleteCodegenTable(record.id);
    message.success('配置已删除');
    await fetchConfigs();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除失败');
  }
}

onMounted(() => {
  void fetchConfigs();
});
</script>

<style scoped>
.code-preview {
  margin: 0;
  padding: 12px;
  overflow: auto;
  max-height: 70vh;
  background: #1f2937;
  border-radius: 6px;
  color: #e5e7eb;
  font-size: 12px;
  line-height: 1.6;
}

.conflict-path {
  margin-bottom: 4px;
  font-family: monospace;
  font-size: 12px;
}
</style>
