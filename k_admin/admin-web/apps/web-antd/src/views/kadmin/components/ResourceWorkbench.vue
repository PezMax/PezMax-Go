<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>组件工作台</h1>
        <p>模块管理</p>
      </div>
      <a-space>
        <a-button @click="refreshRows">
          <ReloadOutlined />
          刷新
        </a-button>
        <a-button type="primary" @click="openDrawer()">
          <PlusOutlined />
          新增
        </a-button>
      </a-space>
    </section>

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="关键词">
          <a-input v-model:value="filters.keyword" allow-clear placeholder="模块 / 负责人" />
        </a-form-item>
        <a-form-item label="状态">
          <a-select
            v-model:value="filters.status"
            allow-clear
            class="control-md"
            :options="statusOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="日期">
          <a-range-picker v-model:value="filters.dateRange" class="control-lg" />
        </a-form-item>
        <a-form-item>
          <a-checkbox v-model:checked="filters.enabledOnly">仅启用</a-checkbox>
        </a-form-item>
        <a-form-item>
          <a-space>
            <a-button type="primary" @click="applySearch">
              <SearchOutlined />
              查询
            </a-button>
            <a-button @click="resetSearch">
              <ClearOutlined />
              重置
            </a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </section>

    <section class="panel">
      <div class="table-toolbar">
        <a-space wrap>
          <a-button :disabled="selectedRowKeys.length !== 1" @click="editSelected">
            <EditOutlined />
            编辑
          </a-button>
          <a-popconfirm title="确认删除选中的模块？" @confirm="removeSelected">
            <a-button danger :disabled="selectedRowKeys.length === 0">
              <DeleteOutlined />
              删除
            </a-button>
          </a-popconfirm>
          <a-button @click="exportRows">
            <DownloadOutlined />
            导出
          </a-button>
        </a-space>
        <a-segmented v-model:value="density" :options="['默认', '紧凑']" />
      </div>

      <a-table
        row-key="key"
        :size="density === '紧凑' ? 'small' : 'middle'"
        :columns="columns"
        :data-source="filteredRows"
        :pagination="pagination"
        :row-selection="{ selectedRowKeys, onChange: onSelectChange }"
        :scroll="{ x: 980 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <a-space>
              <a-avatar :style="{ backgroundColor: avatarColor(record.type) }">
                {{ record.name.slice(0, 1) }}
              </a-avatar>
              <div class="name-cell">
                <strong>{{ record.name }}</strong>
                <span>{{ record.type }}</span>
              </div>
            </a-space>
          </template>

          <template v-else-if="column.key === 'status'">
            <a-tag :color="statusMeta(record.status).color">
              {{ statusMeta(record.status).text }}
            </a-tag>
          </template>

          <template v-else-if="column.key === 'priority'">
            <a-tag :color="priorityMeta(record.priority).color">
              {{ priorityMeta(record.priority).text }}
            </a-tag>
          </template>

          <template v-else-if="column.key === 'progress'">
            <a-progress :percent="record.progress" size="small" />
          </template>

          <template v-else-if="column.key === 'enabled'">
            <a-switch v-model:checked="record.enabled" size="small" />
          </template>

          <template v-else-if="column.key === 'tags'">
            <a-space :size="4" wrap>
              <a-tag v-for="tag in record.tags" :key="tag">{{ tag }}</a-tag>
            </a-space>
          </template>

          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button type="link" size="small" @click="openDetail(record)">查看</a-button>
              <a-button type="link" size="small" @click="openDrawer(record)">编辑</a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:open="drawerOpen"
      :title="editingKey ? '编辑模块' : '新增模块'"
      width="520"
      :destroy-on-close="true"
    >
      <a-form ref="formRef" :model="formState" :rules="rules" layout="vertical">
        <a-form-item label="模块名称" name="name">
          <a-input v-model:value="formState.name" placeholder="请输入模块名称" />
        </a-form-item>
        <a-form-item label="模块类型" name="type">
          <a-select v-model:value="formState.type" :options="typeOptions" />
        </a-form-item>
        <a-form-item label="负责人" name="owner">
          <a-input v-model:value="formState.owner" placeholder="请输入负责人" />
        </a-form-item>
        <a-form-item label="状态" name="status">
          <a-radio-group v-model:value="formState.status" button-style="solid">
            <a-radio-button value="online">运行中</a-radio-button>
            <a-radio-button value="pending">待配置</a-radio-button>
            <a-radio-button value="offline">停用</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="优先级" name="priority">
          <a-select v-model:value="formState.priority" :options="priorityOptions" />
        </a-form-item>
        <a-form-item label="进度" name="progress">
          <a-slider v-model:value="formState.progress" :step="1" />
        </a-form-item>
        <a-form-item label="启用">
          <a-switch v-model:checked="formState.enabled" />
        </a-form-item>
        <a-form-item label="标签">
          <a-checkbox-group v-model:value="formState.tags" :options="tagOptions" />
        </a-form-item>
        <a-form-item label="附件">
          <a-upload :file-list="fileList" :before-upload="beforeUpload" @remove="removeFile">
            <a-button>
              <UploadOutlined />
              选择文件
            </a-button>
          </a-upload>
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="drawerOpen = false">取消</a-button>
          <a-button type="primary" :loading="submitLoading" @click="submitForm">保存</a-button>
        </a-space>
      </template>
    </a-drawer>

    <a-modal v-model:open="detailOpen" title="模块详情" :footer="null">
      <a-descriptions v-if="detailRecord" bordered size="small" :column="1">
        <a-descriptions-item label="模块">{{ detailRecord.name }}</a-descriptions-item>
        <a-descriptions-item label="类型">{{ detailRecord.type }}</a-descriptions-item>
        <a-descriptions-item label="负责人">{{ detailRecord.owner }}</a-descriptions-item>
        <a-descriptions-item label="创建日期">{{ detailRecord.createdAt }}</a-descriptions-item>
        <a-descriptions-item label="进度">
          <a-progress :percent="detailRecord.progress" size="small" />
        </a-descriptions-item>
      </a-descriptions>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  UploadOutlined,
} from '@ant-design/icons-vue';
import { message, type FormInstance, type UploadProps } from 'ant-design-vue';
import { computed, reactive, ref } from 'vue';

import { initialResources } from '../mock';
import type { ResourceItem, ResourcePriority, ResourceStatus } from '../types';

type TablePagination = {
  current?: number;
  pageSize?: number;
};

const filters = reactive<{
  keyword: string;
  status?: ResourceStatus;
  dateRange: unknown[];
  enabledOnly: boolean;
}>({
  keyword: '',
  status: undefined,
  dateRange: [],
  enabledOnly: false,
});
const rows = ref<ResourceItem[]>([...initialResources]);
const selectedRowKeys = ref<string[]>([]);
const density = ref('默认');
const drawerOpen = ref(false);
const detailOpen = ref(false);
const detailRecord = ref<ResourceItem | null>(null);
const editingKey = ref('');
const submitLoading = ref(false);
const formRef = ref<FormInstance>();
const fileList = ref<UploadProps['fileList']>([]);

const formState = reactive<ResourceItem>({
  key: '',
  name: '',
  type: '系统模块',
  owner: '',
  status: 'pending',
  priority: 'medium',
  progress: 40,
  enabled: true,
  createdAt: '',
  tags: [],
});

const rules = {
  name: [{ required: true, message: '请输入模块名称' }],
  type: [{ required: true, message: '请选择模块类型' }],
  owner: [{ required: true, message: '请输入负责人' }],
  status: [{ required: true, message: '请选择状态' }],
};

const statusOptions = [
  { label: '运行中', value: 'online' },
  { label: '待配置', value: 'pending' },
  { label: '停用', value: 'offline' },
];
const typeOptions = ['系统模块', '通用服务', '基础设施', '快速开发'].map((value) => ({
  label: value,
  value,
}));
const priorityOptions = [
  { label: '高', value: 'high' },
  { label: '中', value: 'medium' },
  { label: '低', value: 'low' },
];
const tagOptions = ['RBAC', '菜单', '权限', '上传', 'MinIO', 'CRUD', '审计'].map((value) => ({
  label: value,
  value,
}));

const columns = [
  { title: '模块', dataIndex: 'name', key: 'name', width: 230, fixed: 'left' },
  { title: '负责人', dataIndex: 'owner', key: 'owner', width: 120 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 110 },
  { title: '优先级', dataIndex: 'priority', key: 'priority', width: 100 },
  { title: '进度', dataIndex: 'progress', key: 'progress', width: 170 },
  { title: '启用', dataIndex: 'enabled', key: 'enabled', width: 90 },
  { title: '标签', dataIndex: 'tags', key: 'tags', width: 180 },
  { title: '创建日期', dataIndex: 'createdAt', key: 'createdAt', width: 120 },
  { title: '操作', key: 'action', width: 140, fixed: 'right' },
];

const pagination = reactive({
  current: 1,
  pageSize: 6,
  showSizeChanger: true,
  pageSizeOptions: ['6', '10', '20'],
  showTotal: (total: number) => `共 ${total} 条`,
});

const filteredRows = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase();
  return rows.value.filter((item) => {
    const matchKeyword =
      !keyword ||
      item.name.toLowerCase().includes(keyword) ||
      item.owner.toLowerCase().includes(keyword) ||
      item.type.toLowerCase().includes(keyword);
    const matchStatus = !filters.status || item.status === filters.status;
    const matchEnabled = !filters.enabledOnly || item.enabled;
    return matchKeyword && matchStatus && matchEnabled;
  });
});

function statusMeta(status: ResourceStatus) {
  return {
    online: { color: 'green', text: '运行中' },
    pending: { color: 'orange', text: '待配置' },
    offline: { color: 'red', text: '停用' },
  }[status];
}

function priorityMeta(priority: ResourcePriority) {
  return {
    high: { color: 'red', text: '高' },
    medium: { color: 'blue', text: '中' },
    low: { color: 'default', text: '低' },
  }[priority];
}

function avatarColor(type: string) {
  if (type === '系统模块') return 'hsl(var(--primary))';
  if (type === '通用服务') return 'hsl(var(--success))';
  if (type === '快速开发') return 'hsl(var(--accent-foreground))';
  return 'hsl(var(--warning))';
}

function applySearch() {
  pagination.current = 1;
  message.success('查询完成');
}

function resetSearch() {
  filters.keyword = '';
  filters.status = undefined;
  filters.dateRange = [];
  filters.enabledOnly = false;
  pagination.current = 1;
}

function refreshRows() {
  rows.value = [...initialResources];
  selectedRowKeys.value = [];
  message.success('已刷新');
}

function onSelectChange(keys: string[]) {
  selectedRowKeys.value = keys;
}

function handleTableChange(pager: TablePagination) {
  pagination.current = pager.current || 1;
  pagination.pageSize = pager.pageSize || 6;
}

function openDrawer(record?: ResourceItem) {
  editingKey.value = record?.key || '';
  Object.assign(
    formState,
    record
      ? { ...record, tags: [...record.tags] }
      : {
          key: '',
          name: '',
          type: '系统模块',
          owner: '',
          status: 'pending',
          priority: 'medium',
          progress: 40,
          enabled: true,
          createdAt: '',
          tags: [],
        },
  );
  fileList.value = [];
  drawerOpen.value = true;
}

function editSelected() {
  const record = rows.value.find((item) => item.key === selectedRowKeys.value[0]);
  if (record) {
    openDrawer(record);
  }
}

function removeSelected() {
  rows.value = rows.value.filter((item) => !selectedRowKeys.value.includes(item.key));
  selectedRowKeys.value = [];
  message.success('已删除');
}

function exportRows() {
  message.success(`已导出 ${filteredRows.value.length} 条`);
}

function openDetail(record: ResourceItem) {
  detailRecord.value = record;
  detailOpen.value = true;
}

const beforeUpload: UploadProps['beforeUpload'] = (file) => {
  fileList.value = [file];
  message.success(`${file.name} 已选择`);
  return false;
};

function removeFile() {
  fileList.value = [];
}

async function submitForm() {
  await formRef.value?.validate();
  submitLoading.value = true;
  window.setTimeout(() => {
    const now = new Date().toISOString().slice(0, 10);
    if (editingKey.value) {
      rows.value = rows.value.map((item) =>
        item.key === editingKey.value ? { ...formState, key: editingKey.value } : item,
      );
      message.success('保存成功');
    } else {
      rows.value = [
        { ...formState, key: `${Date.now()}`, createdAt: now },
        ...rows.value,
      ];
      message.success('新增成功');
    }
    submitLoading.value = false;
    drawerOpen.value = false;
  }, 450);
}
</script>
