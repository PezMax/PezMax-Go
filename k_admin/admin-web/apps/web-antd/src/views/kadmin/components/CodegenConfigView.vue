<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>生成配置</h1>
        <p>
          <template v-if="config">
            数据表 {{ config.tableName }}
            <a-tag v-if="config.generated" color="green">已生成</a-tag>
          </template>
          <template v-else-if="errorText">配置不可用</template>
          <template v-else>加载中…</template>
        </p>
      </div>
      <a-space wrap>
        <a-button @click="goBack">
          <ArrowLeftOutlined />
          返回列表
        </a-button>
        <a-button type="primary" :loading="saving" :disabled="!canImport" @click="saveConfig">
          <SaveOutlined />
          保存配置
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

    <a-spin :spinning="loading">
      <template v-if="config">
        <section class="panel">
          <h2 class="panel-title">基本信息</h2>
          <a-form layout="vertical" class="config-form">
            <a-row :gutter="16">
              <a-col :xs="24" :md="12">
                <a-form-item label="数据表">
                  <a-input :value="config.tableName" disabled />
                </a-form-item>
              </a-col>
              <a-col :xs="24" :md="12">
                <a-form-item label="业务名">
                  <a-input v-model:value="draft.businessName" @change="markDirty" />
                </a-form-item>
              </a-col>
              <a-col :xs="24" :md="12">
                <a-form-item label="模块名（Go 包名）">
                  <a-input v-model:value="draft.moduleName" @change="markDirty" />
                </a-form-item>
              </a-col>
              <a-col :xs="24" :md="12">
                <a-form-item label="类名">
                  <a-input v-model:value="draft.className" @change="markDirty" />
                </a-form-item>
              </a-col>
              <a-col :xs="24" :md="12">
                <a-form-item label="路由前缀">
                  <a-input v-model:value="draft.routePrefix" @change="markDirty" />
                </a-form-item>
              </a-col>
            </a-row>
          </a-form>
        </section>

        <section class="panel">
          <h2 class="panel-title">字段配置</h2>
          <div class="table-toolbar">
            <a-space wrap>
              <a-tag color="blue">列表列控制表格展示</a-tag>
              <a-tag>查询列进入搜索表单</a-tag>
              <a-tag color="purple">新增/修改列进入表单</a-tag>
            </a-space>
          </div>
          <a-table
            row-key="name"
            size="small"
            :columns="columnEditColumns"
            :data-source="draft.columns"
            :pagination="false"
            :scroll="{ x: 860 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <strong>{{ record.name }}</strong>
                <a-tag v-if="record.isPK" color="blue">主键</a-tag>
              </template>
              <template v-else-if="column.key === 'type'">
                <a-typography-text class="job-code">{{ record.goType }}</a-typography-text>
              </template>
              <template v-else-if="column.key === 'label'">
                <a-input v-model:value="record.label" size="small" @change="markDirty" />
              </template>
              <template v-else-if="column.key === 'control'">
                <a-select
                  v-model:value="record.control"
                  size="small"
                  :options="controlOptions"
                  @change="markDirty"
                />
              </template>
              <template v-else-if="column.key === 'listed'">
                <a-switch v-model:checked="record.listed" size="small" @change="markDirty" />
              </template>
              <template v-else-if="column.key === 'queryable'">
                <a-switch v-model:checked="record.queryable" size="small" @change="markDirty" />
              </template>
              <template v-else-if="column.key === 'creatable'">
                <a-switch
                  v-model:checked="record.creatable"
                  size="small"
                  :disabled="record.isPK"
                  @change="markDirty"
                />
              </template>
              <template v-else-if="column.key === 'editable'">
                <a-switch
                  v-model:checked="record.editable"
                  size="small"
                  :disabled="record.isPK"
                  @change="markDirty"
                />
              </template>
              <template v-else-if="column.key === 'required'">
                <a-switch
                  v-model:checked="record.required"
                  size="small"
                  :disabled="record.isPK"
                  @change="markDirty"
                />
              </template>
            </template>
          </a-table>
        </section>
      </template>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeftOutlined, SaveOutlined } from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message, Modal } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router';

import {
  getCodegenTable,
  updateCodegenTable,
  type CodegenColumn,
  type CodegenTableConfig,
} from '#/api/kadmin/codegen';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

const { hasAccessByCodes } = useAccess();
const canImport = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.CODEGEN_IMPORT, '*']),
);

const route = useRoute();
const router = useRouter();

const controlOptions = [
  { label: '输入框', value: 'input' },
  { label: '多行文本', value: 'textarea' },
  { label: '数字', value: 'number' },
  { label: '开关', value: 'switch' },
  { label: '日期', value: 'date' },
  { label: '日期时间', value: 'datetime' },
  { label: '时间', value: 'time' },
];

const columnEditColumns = [
  { title: '字段', key: 'name', width: 150, fixed: 'left' },
  { title: '类型', key: 'type', width: 90 },
  { title: '标签', key: 'label', width: 150 },
  { title: '控件', key: 'control', width: 120 },
  { title: '列表', key: 'listed', width: 60 },
  { title: '查询', key: 'queryable', width: 60 },
  { title: '新增', key: 'creatable', width: 60 },
  { title: '修改', key: 'editable', width: 60 },
  { title: '必填', key: 'required', width: 60 },
];

const loading = ref(false);
const saving = ref(false);
const errorText = ref('');
const dirty = ref(false);
const config = ref<CodegenTableConfig | null>(null);
const draft = reactive({
  businessName: '',
  className: '',
  columns: [] as CodegenColumn[],
  moduleName: '',
  routePrefix: '',
});

function markDirty() {
  if (!loading.value) {
    dirty.value = true;
  }
}

async function loadConfig() {
  const id = Number(route.query.id);
  if (!Number.isInteger(id) || id <= 0) {
    errorText.value = '缺少有效的配置 ID';
    return;
  }
  loading.value = true;
  errorText.value = '';
  try {
    const detail = await getCodegenTable(id);
    config.value = detail;
    draft.moduleName = detail.moduleName;
    draft.className = detail.className;
    draft.businessName = detail.businessName;
    draft.routePrefix = detail.routePrefix;
    draft.columns = detail.columns.map((column) => ({ ...column }));
    dirty.value = false;
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : '加载配置失败';
  } finally {
    loading.value = false;
  }
}

async function saveConfig() {
  if (!config.value) {
    return;
  }
  saving.value = true;
  try {
    await updateCodegenTable(config.value.id, {
      businessName: draft.businessName,
      className: draft.className,
      columns: draft.columns,
      moduleName: draft.moduleName,
      routePrefix: draft.routePrefix,
    });
    dirty.value = false;
    message.success('配置已保存');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存失败');
  } finally {
    saving.value = false;
  }
}

function goBack() {
  if (!dirty.value) {
    void router.push('/kadmin/codegen');
    return;
  }
  Modal.confirm({
    title: '放弃修改？',
    content: '当前配置有未保存的修改。',
    okText: '放弃',
    okType: 'danger',
    cancelText: '继续编辑',
    onOk: () => {
      dirty.value = false;
      void router.push('/kadmin/codegen');
    },
  });
}

onBeforeRouteLeave(() => {
  if (!dirty.value) {
    return true;
  }
  return new Promise<boolean>((resolve) => {
    Modal.confirm({
      title: '放弃修改？',
      content: '当前配置有未保存的修改。',
      okText: '放弃',
      okType: 'danger',
      cancelText: '继续编辑',
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
});

onMounted(() => {
  void loadConfig();
});
</script>

<style scoped>
.config-form {
  margin-bottom: 4px;
}

.panel-title {
  margin: 0 0 12px;
  font-size: 15px;
}
</style>
