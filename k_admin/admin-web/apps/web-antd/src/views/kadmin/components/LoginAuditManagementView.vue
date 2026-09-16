<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>登录审计</h1>
        <p>查询账号登录结果与客户端信息</p>
      </div>
    </section>

    <section class="panel">
      <a-form :model="filters" layout="inline" class="search-form">
        <a-form-item label="账号">
          <a-input
            v-model:value="filters.account"
            allow-clear
            class="control-md"
            placeholder="登录账号"
            @press-enter="searchAudits"
          />
        </a-form-item>
        <a-form-item label="IP">
          <a-input
            v-model:value="filters.ip"
            allow-clear
            class="control-md"
            placeholder="客户端 IP"
            @press-enter="searchAudits"
          />
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
        <a-form-item label="结果">
          <a-select
            v-model:value="filters.result"
            allow-clear
            class="control-lg"
            :options="resultOptions"
            placeholder="全部"
          />
        </a-form-item>
        <a-form-item label="时间">
          <a-range-picker
            v-model:value="filters.dateRange"
            class="audit-date-range"
            format="YYYY-MM-DD HH:mm"
            :show-time="{ format: 'HH:mm' }"
          />
        </a-form-item>
        <a-form-item>
          <a-space wrap>
            <a-button type="primary" @click="searchAudits">
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
        <span class="muted-text">共 {{ pagination.total }} 条记录</span>
        <a-space wrap>
          <span class="muted-text">保留 {{ retentionDays }} 天</span>
          <a-tooltip v-if="canRetention" title="设置保留周期">
            <a-button shape="circle" @click="openRetention"><SettingOutlined /></a-button>
          </a-tooltip>
          <a-tooltip title="刷新">
            <a-button shape="circle" :loading="loading" @click="loadAudits">
              <ReloadOutlined />
            </a-button>
          </a-tooltip>
          <a-popconfirm
            v-if="canDelete"
            :title="`确认清理超过 ${retentionDays} 天的记录？`"
            @confirm="cleanupExpired"
          >
            <a-button :loading="cleaning"><ClearOutlined />清理过期</a-button>
          </a-popconfirm>
          <a-popconfirm
            v-if="canDelete"
            title="确认删除选中的登录审计记录？此操作无法撤销。"
            @confirm="removeSelected"
          >
            <a-button danger :disabled="selectedRowKeys.length === 0" :loading="deleting">
              <DeleteOutlined />批量删除
            </a-button>
          </a-popconfirm>
        </a-space>
      </div>

      <a-table
        row-key="id"
        class="compact-user-table audit-table"
        size="small"
        :columns="columns"
        :data-source="audits"
        :loading="loading"
        :locale="{ emptyText: '暂无登录审计记录' }"
        :pagination="pagination"
        :row-selection="rowSelection"
        :scroll="{ x: 1390 }"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'occurredAt'">
            {{ formatLoginTime(record.occurredAt) }}
          </template>
          <template v-else-if="column.key === 'account'">
            <div class="audit-two-line">
              <strong>{{ record.account || '-' }}</strong>
              <span>用户 ID：{{ record.userId ?? '-' }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'client'">
            <div class="audit-two-line">
              <strong>{{ record.browser || '未知浏览器' }}</strong>
              <span>{{ record.os || '未知系统' }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'result'">
            <a-tag :color="record.status === 'success' ? 'green' : resultColor(record.result)">
              {{ resultLabel(record.result) }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'failureReason'">
            <a-typography-text
              :content="record.failureReason || '-'"
              :ellipsis="{ tooltip: record.failureReason }"
            />
          </template>
          <template v-else-if="column.key === 'durationMs'">
            {{ formatDuration(record.durationMs) }}
          </template>
          <template v-else-if="column.key === 'action'">
            <a-tooltip title="查看详情">
              <a-button type="text" shape="circle" @click="openDetail(record)">
                <EyeOutlined />
              </a-button>
            </a-tooltip>
          </template>
        </template>
      </a-table>
    </section>

    <a-drawer v-model:open="detailOpen" title="登录审计详情" width="min(640px, 94vw)">
      <a-descriptions v-if="detailAudit" bordered size="small" :column="1">
        <a-descriptions-item label="登录时间">{{ detailAudit.occurredAt }}</a-descriptions-item>
        <a-descriptions-item label="账号 / 用户 ID">
          {{ detailAudit.account || '-' }} / {{ detailAudit.userId ?? '-' }}
        </a-descriptions-item>
        <a-descriptions-item label="登录结果">
          <a-tag
            :color="detailAudit.status === 'success' ? 'green' : resultColor(detailAudit.result)"
          >
            {{ resultLabel(detailAudit.result) }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="失败原因">{{
          detailAudit.failureReason || '-'
        }}</a-descriptions-item>
        <a-descriptions-item label="耗时">{{
          formatDuration(detailAudit.durationMs)
        }}</a-descriptions-item>
        <a-descriptions-item label="IP">{{ detailAudit.ip || '-' }}</a-descriptions-item>
        <a-descriptions-item label="浏览器">{{ detailAudit.browser || '-' }}</a-descriptions-item>
        <a-descriptions-item label="操作系统">{{ detailAudit.os || '-' }}</a-descriptions-item>
        <a-descriptions-item label="User-Agent">
          <span class="audit-break-text">{{ detailAudit.userAgent || '-' }}</span>
        </a-descriptions-item>
      </a-descriptions>
    </a-drawer>

    <a-modal
      v-model:open="retentionOpen"
      title="登录审计保留周期"
      :confirm-loading="savingRetention"
      @ok="saveRetention"
    >
      <a-form layout="vertical">
        <a-form-item label="保留天数" required>
          <a-input-number
            v-model:value="retentionDraft"
            :min="1"
            :max="3650"
            class="retention-input"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import dayjs, { type Dayjs } from 'dayjs';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  cleanupExpiredLoginAudits,
  deleteLoginAudits,
  getLoginAuditRetention,
  getLoginAudits,
  type LoginAudit,
  type LoginAuditResult,
  type LoginAuditStatus,
  updateLoginAuditRetention,
} from '#/api/kadmin/loginAudits';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

type TablePagination = { current?: number; pageSize?: number };

const { hasAccessByCodes } = useAccess();
const canDelete = computed(() => hasAccessByCodes([KADMIN_PERMISSION.LOGIN_LOG_DELETE, '*']));
const canRetention = computed(() => hasAccessByCodes([KADMIN_PERMISSION.LOGIN_LOG_RETENTION, '*']));
const audits = ref<LoginAudit[]>([]);
const loading = ref(false);
const deleting = ref(false);
const cleaning = ref(false);
const selectedRowKeys = ref<number[]>([]);
const detailOpen = ref(false);
const detailAudit = ref<LoginAudit | null>(null);
const retentionDays = ref(90);
const retentionDraft = ref(90);
const retentionOpen = ref(false);
const savingRetention = ref(false);

const filters = reactive({
  account: '',
  dateRange: null as [Dayjs, Dayjs] | null,
  ip: '',
  result: undefined as LoginAuditResult | undefined,
  status: undefined as LoginAuditStatus | undefined,
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
  {
    title: '登录时间',
    dataIndex: 'occurredAt',
    key: 'occurredAt',
    width: 180,
    fixed: 'left',
  },
  { title: '账号', key: 'account', width: 180 },
  { title: 'IP', dataIndex: 'ip', key: 'ip', width: 150 },
  { title: '浏览器 / OS', key: 'client', width: 240 },
  { title: '结果', key: 'result', width: 130 },
  { title: '失败原因', key: 'failureReason', width: 180 },
  { title: '耗时', key: 'durationMs', width: 100 },
  { title: '操作', key: 'action', width: 80, fixed: 'right' },
];
const statusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
];
const resultOptions = [
  { label: '登录成功', value: 'success' },
  { label: '账号不存在', value: 'account_not_found' },
  { label: '密码错误', value: 'invalid_password' },
  { label: '账号禁用', value: 'account_disabled' },
  { label: '锁定拒绝', value: 'account_locked' },
  { label: '管理员解锁', value: 'account_unlocked' },
  { label: '验证码错误', value: 'captcha_invalid' },
  { label: '系统异常', value: 'system_error' },
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

onMounted(() => {
  void Promise.all([loadAudits(), loadRetention()]);
});

async function loadAudits() {
  loading.value = true;
  try {
    const data = await getLoginAudits({
      account: filters.account.trim(),
      endedAt: filters.dateRange?.[1].toISOString(),
      ip: filters.ip.trim(),
      page: pagination.current,
      pageSize: pagination.pageSize,
      result: filters.result,
      startedAt: filters.dateRange?.[0].toISOString(),
      status: filters.status,
    });
    audits.value = data.items || [];
    pagination.total = data.total || 0;
    selectedRowKeys.value = selectedRowKeys.value.filter((id) =>
      audits.value.some((item) => item.id === id)
    );
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载登录审计失败');
  } finally {
    loading.value = false;
  }
}

async function loadRetention() {
  try {
    const setting = await getLoginAuditRetention();
    retentionDays.value = setting.days;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载保留周期失败');
  }
}

function searchAudits() {
  pagination.current = 1;
  void loadAudits();
}

function resetSearch() {
  filters.account = '';
  filters.dateRange = null;
  filters.ip = '';
  filters.result = undefined;
  filters.status = undefined;
  pagination.current = 1;
  void loadAudits();
}

function handleTableChange(pager: TablePagination) {
  pagination.current = pager.current || 1;
  pagination.pageSize = pager.pageSize || 20;
  void loadAudits();
}

function openDetail(record: LoginAudit) {
  detailAudit.value = record;
  detailOpen.value = true;
}

async function removeSelected() {
  if (selectedRowKeys.value.length === 0) return;
  deleting.value = true;
  try {
    const result = await deleteLoginAudits(selectedRowKeys.value);
    selectedRowKeys.value = [];
    await loadAudits();
    message.success(`已删除 ${result.deletedCount} 条记录`);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除登录审计失败');
  } finally {
    deleting.value = false;
  }
}

async function cleanupExpired() {
  cleaning.value = true;
  try {
    const result = await cleanupExpiredLoginAudits();
    await loadAudits();
    message.success(`已清理 ${result.deletedCount} 条过期记录`);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '清理过期记录失败');
  } finally {
    cleaning.value = false;
  }
}

function openRetention() {
  retentionDraft.value = retentionDays.value;
  retentionOpen.value = true;
}

async function saveRetention() {
  if (!retentionDraft.value || retentionDraft.value < 1 || retentionDraft.value > 3650) {
    message.warning('保留天数必须在 1 至 3650 之间');
    return;
  }
  savingRetention.value = true;
  try {
    const setting = await updateLoginAuditRetention(retentionDraft.value);
    retentionDays.value = setting.days;
    retentionOpen.value = false;
    await loadAudits();
    message.success('保留周期已更新');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '更新保留周期失败');
  } finally {
    savingRetention.value = false;
  }
}

function resultLabel(result: LoginAuditResult) {
  return resultOptions.find((item) => item.value === result)?.label || result;
}

function resultColor(result: LoginAuditResult) {
  if (result === 'account_unlocked') return 'green';
  if (result === 'account_disabled' || result === 'account_locked' || result === 'captcha_invalid')
    return 'red';
  if (result === 'system_error') return 'magenta';
  return 'orange';
}

function formatLoginTime(value: string) {
  if (!value) return '-';
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value;
}

function formatDuration(duration: number) {
  return duration < 1000 ? `${duration} ms` : `${(duration / 1000).toFixed(2)} s`;
}
</script>

<style scoped>
.audit-date-range {
  width: 340px;
}

.audit-two-line {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.audit-two-line span {
  overflow: hidden;
  color: var(--ant-color-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.audit-break-text {
  overflow-wrap: anywhere;
}

.retention-input {
  width: 100%;
}

@media (max-width: 768px) {
  .audit-date-range {
    width: min(100%, 340px);
  }
}
</style>
