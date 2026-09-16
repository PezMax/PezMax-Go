<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>接口负载排行</h1>
        <p>按接口、方法或状态码聚合 HTTP 指标，定位高频、慢与高错误率接口</p>
      </div>
      <a-space class="monitor-actions" wrap>
        <a-space class="monitor-switch">
          <span>接口采样</span>
          <a-switch
            :checked="status.enabled"
            :disabled="!canUpdate"
            :loading="switchLoading"
            checked-children="开"
            un-checked-children="关"
            @change="toggleSampling"
          />
        </a-space>
        <a-tooltip title="刷新排行">
          <a-button
            class="monitor-refresh-button"
            shape="circle"
            :loading="loading"
            aria-label="刷新排行"
            @click="loadRankings"
          >
            <ReloadOutlined />
          </a-button>
        </a-tooltip>
      </a-space>
    </section>

    <a-alert
      v-if="apiError"
      type="error"
      show-icon
      closable
      :message="apiError"
      @close="apiError = ''"
    />
    <a-alert
      v-else-if="!status.enabled"
      type="info"
      show-icon
      message="接口采样已关闭"
      description="关闭后服务端不再写入负载指标，排行只显示已保留的历史数据。"
    />
    <a-alert v-else-if="status.lastError" type="warning" show-icon :message="status.lastError" />

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="时间范围">
          <a-range-picker
            v-model:value="filters.dateRange"
            class="log-date-range"
            format="YYYY-MM-DD HH:mm"
            :show-time="{ format: 'HH:mm' }"
            :presets="rangePresets"
            :disabled-date="disableFutureDate"
          />
        </a-form-item>
        <a-form-item label="接口">
          <a-input
            v-model:value="filters.route"
            allow-clear
            class="control-lg"
            placeholder="路径关键词，如 /api/users"
            @press-enter="searchRankings"
          >
            <template #prefix><SearchOutlined /></template>
          </a-input>
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
        <a-form-item label="状态码">
          <a-input-number
            v-model:value="filters.statusCode"
            class="control-md"
            :min="100"
            :max="599"
            placeholder="100 - 599"
          />
        </a-form-item>
        <a-form-item label="聚合维度">
          <a-radio-group v-model:value="filters.groupBy" button-style="solid">
            <a-radio-button value="route">接口</a-radio-button>
            <a-radio-button value="method">方法</a-radio-button>
            <a-radio-button value="status">状态码</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item>
          <a-space wrap>
            <a-button type="primary" @click="searchRankings">
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
        <span class="muted-text">
          {{ summaryText }}
        </span>
      </div>
      <a-table
        :columns="columns"
        :data-source="rankings"
        :loading="loading"
        :pagination="pagination"
        :row-key="rowKey"
        size="middle"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'route'">
            <code class="route-cell">{{ record.route || '-' }}</code>
          </template>
          <template v-else-if="column.key === 'method'">
            <a-tag v-if="record.method">{{ record.method }}</a-tag>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key === 'statusCode'">
            <span v-if="record.statusCode !== null">{{ record.statusCode }}</span>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key === 'requestCount'">
            {{ record.requestCount.toLocaleString() }}
          </template>
          <template v-else-if="column.key === 'qps'">
            {{ formatQps(record.qps) }}
          </template>
          <template v-else-if="column.key === 'errorRate'">
            <span :class="errorRateClass(record.errorRate)">
              {{ formatPercent(record.errorRate) }}
            </span>
          </template>
          <template v-else-if="column.key === 'avgDurationMs'">
            {{ formatDuration(record.avgDurationMs) }}
          </template>
          <template v-else-if="column.key === 'maxDurationMs'">
            {{ formatDuration(record.maxDurationMs) }}
          </template>
        </template>
        <template #emptyText>
          <a-empty :description="emptyDescription" />
        </template>
      </a-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ClearOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import dayjs, { type Dayjs } from 'dayjs';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  getLoadRankingStatus,
  getLoadRankings,
  type LoadRankingDimension,
  type LoadRankingItem,
  type LoadRankingResult,
  type LoadRankingStatus,
  updateLoadRankingStatus,
} from '#/api/kadmin/loadRanking';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

const { hasAccessByCodes } = useAccess();
const canUpdate = computed(() => hasAccessByCodes([KADMIN_PERMISSION.LOAD_RANK_UPDATE, '*']));

const status = reactive<LoadRankingStatus>({
  bucketSeconds: 60,
  enabled: false,
  flushIntervalSeconds: 10,
  lastError: '',
  lastFlushAt: '',
  retentionDays: 30,
});
const rankings = ref<LoadRankingItem[]>([]);
const total = ref(0);
const windowSeconds = ref(0);
const loading = ref(false);
const switchLoading = ref(false);
const apiError = ref('');

const filters = reactive<{
  dateRange: null | [Dayjs, Dayjs];
  groupBy: 'method' | 'route' | 'status';
  method: string;
  route: string;
  statusCode: null | number;
}>({
  dateRange: null,
  groupBy: 'route',
  method: '',
  route: '',
  statusCode: null,
});

const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'HEAD'].map(
  (method) => ({ label: method, value: method })
);

const rangePresets = [
  { label: '最近 1 小时', value: () => [dayjs().subtract(1, 'hour'), dayjs()] },
  { label: '最近 6 小时', value: () => [dayjs().subtract(6, 'hour'), dayjs()] },
  { label: '最近 24 小时', value: () => [dayjs().subtract(24, 'hour'), dayjs()] },
  { label: '最近 7 天', value: () => [dayjs().subtract(7, 'day'), dayjs()] },
];

const columns = computed(() => {
  const sortKey: LoadRankingDimension = dimension.value;
  return [
    {
      dataIndex: 'route',
      key: 'route',
      title:
        filters.groupBy === 'method' ? '方法' : filters.groupBy === 'status' ? '状态码' : '接口',
      ellipsis: true,
    },
    ...(filters.groupBy === 'route'
      ? [
          { dataIndex: 'method', key: 'method', title: '方法', width: 96 },
          { dataIndex: 'statusCode', key: 'statusCode', title: '状态码', width: 96 },
        ]
      : []),
    {
      dataIndex: 'requestCount',
      key: 'requestCount',
      title: '请求量',
      width: 120,
      sorter: true,
      sortOrder: sortKey === 'requestCount' ? sortDirection.value : null,
    },
    {
      dataIndex: 'qps',
      key: 'qps',
      title: 'QPS',
      width: 110,
      sorter: true,
      sortOrder: sortKey === 'qps' ? sortDirection.value : null,
    },
    {
      dataIndex: 'errorRate',
      key: 'errorRate',
      title: '错误率',
      width: 110,
      sorter: true,
      sortOrder: sortKey === 'errorRate' ? sortDirection.value : null,
    },
    {
      dataIndex: 'avgDurationMs',
      key: 'avgDurationMs',
      title: '平均耗时',
      width: 120,
      sorter: true,
      sortOrder: sortKey === 'avgDurationMs' ? sortDirection.value : null,
    },
    {
      dataIndex: 'maxDurationMs',
      key: 'maxDurationMs',
      title: '最大耗时',
      width: 120,
    },
  ];
});

const dimension = ref<LoadRankingDimension>('requestCount');
const order = ref<'asc' | 'desc'>('desc');
const sortDirection = computed(() => (order.value === 'desc' ? 'descend' : 'ascend'));

const pagination = reactive({
  current: 1,
  pageSize: 20,
  showSizeChanger: true,
  pageSizeOptions: ['10', '20', '50', '100'],
  showTotal: (count: number) => `共 ${count} 条`,
});

const summaryText = computed(() => {
  if (total.value === 0) return '当前筛选范围内暂无指标';
  return `共 ${total.value} 组聚合结果，覆盖 ${formatWindow(windowSeconds.value)} 数据`;
});

const emptyDescription = computed(() =>
  status.enabled ? '当前时间范围内没有采样数据' : '采样关闭期间没有新指标，开启采样后开始聚合'
);

function disableFutureDate(date: Dayjs) {
  return date.isAfter(dayjs(), 'minute');
}

onMounted(() => {
  void loadStatus();
  void loadRankings();
});

async function loadStatus() {
  apiError.value = '';
  try {
    Object.assign(status, await getLoadRankingStatus());
  } catch (error) {
    apiError.value = error instanceof Error ? error.message : '加载接口采样状态失败';
  }
}

async function toggleSampling(checked: boolean | string | number) {
  switchLoading.value = true;
  apiError.value = '';
  try {
    const enabled = Boolean(checked);
    Object.assign(status, await updateLoadRankingStatus(enabled));
    message.success(enabled ? '接口采样已开启' : '接口采样已关闭');
    await loadRankings();
  } catch (error) {
    apiError.value = error instanceof Error ? error.message : '更新采样开关失败';
  } finally {
    switchLoading.value = false;
  }
}

async function loadRankings() {
  loading.value = true;
  apiError.value = '';
  try {
    const [startedAt, endedAt] = resolveRange();
    const result: LoadRankingResult = await getLoadRankings({
      startedAt,
      endedAt,
      route: filters.route.trim() || undefined,
      method: filters.method || undefined,
      statusCode: filters.statusCode ?? undefined,
      groupBy: filters.groupBy,
      dimension: dimension.value,
      order: order.value,
      page: pagination.current,
      pageSize: pagination.pageSize,
    });
    rankings.value = result.items;
    total.value = result.total;
    windowSeconds.value = result.windowSeconds;
  } catch (error) {
    apiError.value = error instanceof Error ? error.message : '加载接口负载排行失败';
  } finally {
    loading.value = false;
  }
}

function resolveRange(): [string, string] {
  const start = filters.dateRange?.[0] ?? dayjs().subtract(1, 'hour');
  const end = filters.dateRange?.[1] ?? dayjs();
  return [start.toISOString(), end.toISOString()];
}

function searchRankings() {
  pagination.current = 1;
  void loadRankings();
}

function resetSearch() {
  filters.dateRange = null;
  filters.route = '';
  filters.method = '';
  filters.statusCode = null;
  filters.groupBy = 'route';
  dimension.value = 'requestCount';
  order.value = 'desc';
  pagination.current = 1;
  void loadRankings();
}

function handleTableChange(
  next: {
    current?: number;
    pageSize?: number;
  },
  _filters: unknown,
  sorter: unknown
) {
  if (next.current) pagination.current = next.current;
  if (next.pageSize) pagination.pageSize = next.pageSize;
  const sort = sorter as { columnKey?: string; order?: string };
  if (sort?.columnKey && sort?.order) {
    dimension.value = sort.columnKey as LoadRankingDimension;
    order.value = sort.order === 'ascend' ? 'asc' : 'desc';
  }
  void loadRankings();
}

function rowKey(record: LoadRankingItem, index: number) {
  const dimensionKey = record.route || record.method || (record.statusCode ?? '') || '';
  return `${dimensionKey}:${index}`;
}

function formatQps(value: number) {
  return value.toFixed(3);
}

function formatPercent(value: number) {
  return `${(value * 100).toFixed(2)}%`;
}

function errorRateClass(value: number) {
  return value >= 0.5 ? 'error-rate-high' : value > 0.05 ? 'error-rate-mid' : '';
}

function formatDuration(milliseconds: number) {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return '-';
  if (milliseconds < 1000) return `${Math.round(milliseconds)} ms`;
  return `${(milliseconds / 1000).toFixed(2)} s`;
}

function formatWindow(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '-';
  const hours = seconds / 3600;
  if (hours >= 24) return `${(hours / 24).toFixed(1)} 天`;
  if (hours >= 1) return `${hours.toFixed(1)} 小时`;
  return `${Math.round(seconds)} 秒`;
}
</script>

<style scoped>
.route-cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  word-break: break-all;
}

.error-rate-high {
  color: var(--color-error, #f5222d);
  font-weight: 600;
}

.error-rate-mid {
  color: var(--color-warning, #fa8c16);
}
</style>
