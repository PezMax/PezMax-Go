<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>日志管理</h1>
        <p>查询服务器请求与审计事件</p>
      </div>
    </section>

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="关键词">
          <a-input
            v-model:value="filters.keyword"
            allow-clear
            class="control-lg"
            placeholder="路径 / 用户 / 请求 ID"
            @press-enter="searchLogs"
          >
            <template #prefix><SearchOutlined /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="事件">
          <a-select
            v-model:value="filters.eventType"
            allow-clear
            class="control-md"
            :options="eventTypeOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="级别">
          <a-select
            v-model:value="filters.level"
            allow-clear
            class="control-md"
            :options="levelOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="来源">
          <a-select
            v-model:value="filters.source"
            allow-clear
            class="control-md"
            :options="sourceOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="方法">
          <a-select
            v-model:value="filters.method"
            allow-clear
            class="control-md"
            :options="methodOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="结果">
          <a-select
            v-model:value="filters.success"
            allow-clear
            class="control-md"
            :options="successOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="状态码">
          <a-input-number
            v-model:value="filters.statusCode"
            class="control-md"
            :min="100"
            :max="599"
            placeholder="100 - 599"
          />
        </a-form-item>
        <a-form-item label="时间">
          <a-range-picker
            v-model:value="filters.dateRange"
            class="log-date-range"
            format="YYYY-MM-DD HH:mm"
            :show-time="{ format: 'HH:mm' }"
          />
        </a-form-item>
        <a-form-item>
          <a-space wrap>
            <a-button type="primary" @click="searchLogs">
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
        <span class="muted-text">共 {{ pagination.total }} 条日志</span>
        <a-space wrap>
          <a-space :size="6">
            <span class="muted-text">自动刷新</span>
            <a-switch v-model:checked="autoRefresh" size="small" />
          </a-space>
          <a-tooltip title="刷新">
            <a-button shape="circle" :loading="loading" @click="loadLogs">
              <ReloadOutlined />
            </a-button>
          </a-tooltip>
          <a-popconfirm
            v-if="canDelete"
            title="确认删除选中的日志？此操作无法撤销。"
            @confirm="removeSelectedLogs"
          >
            <a-button danger :disabled="selectedRowKeys.length === 0" :loading="deleting">
              <DeleteOutlined />
              批量删除
            </a-button>
          </a-popconfirm>
        </a-space>
      </div>

      <a-table
        row-key="id"
        class="compact-user-table log-table"
        size="small"
        :columns="columns"
        :data-source="logs"
        :loading="loading"
        :locale="{ emptyText: '暂无日志' }"
        :pagination="pagination"
        :row-selection="rowSelection"
        :scroll="{ x: 1580 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'level'">
            <a-tag :color="levelMeta(record.level).color">
              {{ levelMeta(record.level).text }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'eventType'">
            <a-tag>{{ eventTypeLabel(record.eventType) }}</a-tag>
          </template>
          <template v-else-if="column.key === 'request'">
            <div class="log-request-cell">
              <div>
                <a-tag v-if="record.method" :color="methodColor(record.method)">
                  {{ record.method }}
                </a-tag>
                <span class="log-path" :title="record.path || record.message">
                  {{ record.path || record.message || '-' }}
                </span>
              </div>
              <span v-if="record.message && record.path">{{ record.message }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'result'">
            <a-space :size="4">
              <a-tag :color="statusColor(record.statusCode)">
                {{ record.statusCode ?? '-' }}
              </a-tag>
              <CheckCircleOutlined v-if="record.success === true" class="log-success-icon" />
              <CloseCircleOutlined v-else-if="record.success === false" class="log-error-icon" />
            </a-space>
          </template>
          <template v-else-if="column.key === 'duration'">
            {{ formatDuration(record.durationMs) }}
          </template>
          <template v-else-if="column.key === 'origin'">
            <div class="log-two-line-cell">
              <strong>{{ record.source || '-' }}</strong>
              <span>{{ record.module || '-' }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'actor'">
            <div class="log-two-line-cell">
              <strong>{{ record.actorName || '匿名' }}</strong>
              <span>{{ record.ip || '-' }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'requestId'">
            <a-typography-text
              class="log-code"
              :content="record.requestId || '-'"
              :copyable="record.requestId ? { text: record.requestId } : false"
              ellipsis
            />
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space :size="4">
              <a-tooltip title="查看详情">
                <a-button type="text" shape="circle" @click="openDetail(record)">
                  <EyeOutlined />
                </a-button>
              </a-tooltip>
              <a-popconfirm
                v-if="canDelete"
                title="确认删除该日志？此操作无法撤销。"
                @confirm="removeLog(record)"
              >
                <a-tooltip title="删除">
                  <a-button type="text" shape="circle" danger><DeleteOutlined /></a-button>
                </a-tooltip>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </section>

    <a-drawer
      v-model:open="detailOpen"
      title="日志详情"
      width="min(680px, 94vw)"
      :destroy-on-close="true"
    >
      <a-spin :spinning="detailLoading">
        <a-descriptions v-if="detailLog" bordered size="small" :column="1">
          <a-descriptions-item label="发生时间">{{ detailLog.occurredAt }}</a-descriptions-item>
          <a-descriptions-item label="事件">
            <a-space>
              <a-tag :color="levelMeta(detailLog.level).color">
                {{ levelMeta(detailLog.level).text }}
              </a-tag>
              <a-tag>{{ eventTypeLabel(detailLog.eventType) }}</a-tag>
            </a-space>
          </a-descriptions-item>
          <a-descriptions-item label="请求">
            <a-tag v-if="detailLog.method" :color="methodColor(detailLog.method)">
              {{ detailLog.method }}
            </a-tag>
            {{ detailLog.path || '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="响应">
            {{ detailLog.statusCode ?? '-' }} / {{ formatDuration(detailLog.durationMs) }}
          </a-descriptions-item>
          <a-descriptions-item label="来源">
            {{ detailLog.source || '-' }} / {{ detailLog.module || '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="操作用户">
            {{ detailLog.actorName || '匿名' }}（{{ detailLog.userId ?? '-' }}）
          </a-descriptions-item>
          <a-descriptions-item label="客户端 IP">{{ detailLog.ip || '-' }}</a-descriptions-item>
          <a-descriptions-item label="请求 ID">
            <a-typography-text :copyable="{ text: detailLog.requestId }">
              {{ detailLog.requestId || '-' }}
            </a-typography-text>
          </a-descriptions-item>
          <a-descriptions-item label="链路 ID">
            <a-typography-text :copyable="{ text: detailLog.traceId }">
              {{ detailLog.traceId || '-' }}
            </a-typography-text>
          </a-descriptions-item>
          <a-descriptions-item label="消息">{{ detailLog.message || '-' }}</a-descriptions-item>
          <a-descriptions-item v-if="detailLog.errorMessage" label="错误">
            {{ detailLog.errorCode || 'error' }}：{{ detailLog.errorMessage }}
          </a-descriptions-item>
          <a-descriptions-item label="请求参数">
            <pre class="log-detail-pre">{{ formatJson(detailLog.input) }}</pre>
          </a-descriptions-item>
          <a-descriptions-item label="元数据">
            <pre class="log-detail-pre">{{ formatJson(detailLog.metadata) }}</pre>
          </a-descriptions-item>
          <a-descriptions-item label="User Agent">
            <span class="log-break-text">{{ detailLog.userAgent || '-' }}</span>
          </a-descriptions-item>
        </a-descriptions>
      </a-spin>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import {
  CheckCircleOutlined,
  ClearOutlined,
  CloseCircleOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import type { Dayjs } from 'dayjs';
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';

import { deleteLog, deleteLogs, getLog, getLogs, type ManagedLog } from '#/api/kadmin/logs';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

type TablePagination = { current?: number; pageSize?: number };

const { hasAccessByCodes } = useAccess();
const canDelete = computed(() => hasAccessByCodes([KADMIN_PERMISSION.LOG_DELETE, '*']));
const loading = ref(false);
const deleting = ref(false);
const detailLoading = ref(false);
const logs = ref<ManagedLog[]>([]);
const selectedRowKeys = ref<number[]>([]);
const detailOpen = ref(false);
const detailLog = ref<ManagedLog | null>(null);
const autoRefresh = ref(false);
let refreshTimer: ReturnType<typeof setInterval> | undefined;
let logRequestInFlight = false;

const filters = reactive({
  dateRange: null as [Dayjs, Dayjs] | null,
  eventType: undefined as string | undefined,
  keyword: '',
  level: undefined as string | undefined,
  method: undefined as string | undefined,
  source: undefined as string | undefined,
  statusCode: undefined as number | undefined,
  success: undefined as boolean | undefined,
});

const pagination = reactive({
  current: 1,
  pageSize: 20,
  pageSizeOptions: ['10', '20', '50', '100'],
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
  total: 0,
});

const columns = [
  { title: '时间', dataIndex: 'occurredAt', key: 'occurredAt', width: 170, fixed: 'left' },
  { title: '级别', key: 'level', width: 82 },
  { title: '事件', key: 'eventType', width: 90 },
  { title: '请求', key: 'request', width: 360 },
  { title: '结果', key: 'result', width: 108 },
  { title: '耗时', key: 'duration', width: 90 },
  { title: '来源 / 模块', key: 'origin', width: 150 },
  { title: '用户 / IP', key: 'actor', width: 180 },
  { title: '请求 ID', key: 'requestId', width: 190 },
  { title: '操作', key: 'action', width: 90, fixed: 'right' },
];

const eventTypeOptions = [
  { label: '请求', value: 'request' },
  { label: '认证', value: 'auth' },
  { label: '操作', value: 'operation' },
  { label: '审计', value: 'audit' },
  { label: '系统', value: 'system' },
];
const levelOptions = ['debug', 'info', 'warn', 'error', 'fatal'].map((value) => ({
  label: levelMeta(value).text,
  value,
}));
const sourceOptions = ['vbenapi', 'goadmin', 'server'].map((value) => ({
  label: value,
  value,
}));
const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((value) => ({
  label: value,
  value,
}));
const successOptions = [
  { label: '成功', value: true },
  { label: '失败', value: false },
];
const rowSelection = computed(() =>
  canDelete.value
    ? {
        onChange: (keys: (number | string)[]) => {
          selectedRowKeys.value = keys.map(Number);
        },
        selectedRowKeys: selectedRowKeys.value,
      }
    : undefined
);

onMounted(() => void loadLogs());
onBeforeUnmount(stopAutoRefresh);
watch(autoRefresh, (enabled) => {
  stopAutoRefresh();
  if (enabled) {
    refreshTimer = setInterval(() => void loadLogs(true), 5000);
  }
});

async function loadLogs(silent = false) {
  if (logRequestInFlight) return;
  logRequestInFlight = true;
  if (!silent) loading.value = true;
  try {
    const data = await getLogs({
      endedAt: filters.dateRange?.[1].toISOString(),
      eventType: filters.eventType,
      keyword: filters.keyword.trim(),
      level: filters.level,
      method: filters.method,
      page: pagination.current,
      pageSize: pagination.pageSize,
      source: filters.source,
      startedAt: filters.dateRange?.[0].toISOString(),
      statusCode: filters.statusCode,
      success: filters.success,
    });
    logs.value = data.items || [];
    pagination.total = data.total || 0;
    selectedRowKeys.value = selectedRowKeys.value.filter((id) =>
      logs.value.some((log) => log.id === id)
    );
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载日志失败');
  } finally {
    if (!silent) loading.value = false;
    logRequestInFlight = false;
  }
}

function searchLogs() {
  pagination.current = 1;
  void loadLogs();
}

function resetSearch() {
  filters.dateRange = null;
  filters.eventType = undefined;
  filters.keyword = '';
  filters.level = undefined;
  filters.method = undefined;
  filters.source = undefined;
  filters.statusCode = undefined;
  filters.success = undefined;
  pagination.current = 1;
  void loadLogs();
}

function handleTableChange(pager: TablePagination) {
  pagination.current = pager.current || 1;
  pagination.pageSize = pager.pageSize || 20;
  void loadLogs();
}

async function openDetail(record: ManagedLog) {
  detailLog.value = record;
  detailOpen.value = true;
  detailLoading.value = true;
  try {
    detailLog.value = await getLog(record.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载日志详情失败');
  } finally {
    detailLoading.value = false;
  }
}

async function removeLog(record: ManagedLog) {
  deleting.value = true;
  try {
    await deleteLog(record.id);
    if (logs.value.length === 1 && pagination.current > 1) pagination.current -= 1;
    await loadLogs();
    message.success('日志已删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除日志失败');
  } finally {
    deleting.value = false;
  }
}

async function removeSelectedLogs() {
  if (selectedRowKeys.value.length === 0) return;
  deleting.value = true;
  try {
    await deleteLogs(selectedRowKeys.value);
    selectedRowKeys.value = [];
    await loadLogs();
    message.success('日志已批量删除');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '批量删除日志失败');
  } finally {
    deleting.value = false;
  }
}

function stopAutoRefresh() {
  if (refreshTimer) clearInterval(refreshTimer);
  refreshTimer = undefined;
}

function levelMeta(level: string) {
  const values: Record<string, { color: string; text: string }> = {
    debug: { color: 'default', text: '调试' },
    error: { color: 'red', text: '错误' },
    fatal: { color: 'magenta', text: '严重' },
    info: { color: 'blue', text: '信息' },
    warn: { color: 'orange', text: '警告' },
  };
  return values[level] || { color: 'default', text: level || '-' };
}

function eventTypeLabel(eventType: string) {
  return eventTypeOptions.find((item) => item.value === eventType)?.label || eventType || '-';
}

function methodColor(method: string) {
  const colors: Record<string, string> = {
    DELETE: 'red',
    GET: 'green',
    PATCH: 'orange',
    POST: 'blue',
    PUT: 'geekblue',
  };
  return colors[method] || 'default';
}

function statusColor(statusCode: null | number) {
  if (!statusCode) return 'default';
  if (statusCode >= 500) return 'red';
  if (statusCode >= 400) return 'orange';
  if (statusCode >= 300) return 'blue';
  return 'green';
}

function formatDuration(duration: null | number) {
  if (duration === null) return '-';
  if (duration < 1000) return `${duration} ms`;
  return `${(duration / 1000).toFixed(2)} s`;
}

function formatJson(value: unknown) {
  if (value === '' || value === null || value === undefined) return '-';
  try {
    const parsed = typeof value === 'string' ? JSON.parse(value) : value;
    return JSON.stringify(parsed, null, 2);
  } catch {
    return String(value);
  }
}
</script>
