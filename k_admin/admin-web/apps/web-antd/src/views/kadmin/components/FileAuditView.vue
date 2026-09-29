<!--
  文件审核工作台（已实装：后端接口见 internal/kadmin/exam_file_audit.go）。

  功能：
  - 待审核 / 被举报复审双队列，独立筛选与分页；
  - 文件分页预览（pdfjs 按需渲染：翻到哪页加载哪页 / 图片 / 不支持格式降级下载），
    预览底部提供快捷审核操作；
  - 多选一键通过、单个用户的全部待审核文件一键通过；
  - 被举报文件复审，审核结论必须附带原因并返回给上传用户。
-->
<script setup lang="ts">
import type { File as ExamFile } from '#/api/kadmin/generated/file';

import {
  CheckOutlined,
  CheckSquareOutlined,
  ClearOutlined,
  CloseOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
  UsergroupAddOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message, Modal } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  approveFiles,
  approveUserFiles,
  auditReportedFile,
  countPendingByUser,
  fetchPendingFiles,
  fetchReportedFiles,
  previewKindOf,
  rejectPendingFile,
  resolvePreview,
  type ReportedExamFile,
} from '#/api/kadmin/fileAudit';

import FilePreviewPager from './FilePreviewPager.vue';

type TablePagination = { current?: number; pageSize?: number };
type AuditDecision = 'approve' | 'reject';

const { hasAccessByCodes } = useAccess();
const canAudit = computed(() => hasAccessByCodes(['system:file:audit', '*']));

const FILE_TYPE_OPTIONS = [
  { label: '期末', value: 1 },
  { label: '期中', value: 2 },
  { label: '资料', value: 3 },
  { label: '补考', value: 4 },
  { label: '其他学校', value: 5 },
];

function fileTypeText(value?: number) {
  return FILE_TYPE_OPTIONS.find((option) => option.value === value)?.label ?? '-';
}

function fileTypeColor(value?: number) {
  const colors = ['', 'blue', 'cyan', 'geekblue', 'purple', 'orange'];
  return colors[value ?? 0] || 'default';
}

const STATUS_META: Record<number, { color: string; text: string }> = {
  0: { color: 'gold', text: '待审核' },
  1: { color: 'success', text: '已通过' },
  2: { color: 'error', text: '未通过' },
  3: { color: 'volcano', text: '被举报' },
};

function statusMeta(status?: number) {
  return STATUS_META[status ?? 0] ?? STATUS_META[0]!;
}

function formatSize(size?: number) {
  if (!size) return '-';
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(0)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

// ---------------------------------------------------------------------------
// 数据加载
// ---------------------------------------------------------------------------

const errorText = ref('');
const activeTab = ref<'pending' | 'reported'>('pending');
// 审核内容队列：exam-试卷资料（ptmj_file），ebook-电子书（ptmj_ebook）
const activeKind = ref<'exam' | 'ebook'>('exam');

const EBOOK_TYPE_OPTIONS = [
  { label: '教材', value: 1 },
  { label: '教辅/参考书', value: 2 },
  { label: '课外读物', value: 3 },
  { label: '其他', value: 4 },
];

function ebookTypeText(value?: number) {
  return EBOOK_TYPE_OPTIONS.find((option) => option.value === value)?.label ?? '-';
}

interface QueueState {
  items: ExamFile[];
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
}

const pending = reactive<QueueState>({
  items: [],
  total: 0,
  page: 1,
  pageSize: 10,
  loading: false,
});

const reported = reactive<QueueState>({
  items: [],
  total: 0,
  page: 1,
  pageSize: 10,
  loading: false,
});

const pendingFilters = reactive<{
  fileName: string;
  fileType: number | undefined;
  user: string;
}>({
  fileName: '',
  fileType: undefined,
  user: '',
});

const reportedFilters = reactive<{ keyword: string }>({ keyword: '' });

/** 本次会话的审核操作计数（页面内统计展示）。 */
const counters = reactive({ approved: 0, rejected: 0 });

async function loadPending() {
  pending.loading = true;
  errorText.value = '';
  try {
    const result = await fetchPendingFiles({
      page: pending.page,
      pageSize: pending.pageSize,
      fileName: pendingFilters.fileName || undefined,
      user: pendingFilters.user || undefined,
      fileType: pendingFilters.fileType,
      kind: activeKind.value,
    });
    pending.items = result.items;
    pending.total = result.total;
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : '加载待审核文件失败';
  } finally {
    pending.loading = false;
  }
}

async function loadReported() {
  reported.loading = true;
  errorText.value = '';
  try {
    const result = await fetchReportedFiles({
      page: reported.page,
      pageSize: reported.pageSize,
      fileName: reportedFilters.keyword || undefined,
      kind: activeKind.value,
    });
    reported.items = result.items;
    reported.total = result.total;
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : '加载被举报文件失败';
  } finally {
    reported.loading = false;
  }
}

function switchKind(kind: 'exam' | 'ebook') {
  if (activeKind.value === kind) return;
  activeKind.value = kind;
  selectedRowKeys.value = [];
  void refreshAll();
}

async function refreshAll() {
  await Promise.all([loadPending(), loadReported()]);
}

function searchPending() {
  pending.page = 1;
  void loadPending();
}

function resetPendingFilters() {
  pendingFilters.fileName = '';
  pendingFilters.user = '';
  pendingFilters.fileType = undefined;
  searchPending();
}

function searchReported() {
  reported.page = 1;
  void loadReported();
}

function resetReportedFilters() {
  reportedFilters.keyword = '';
  searchReported();
}

function handlePendingChange(pagination: TablePagination) {
  pending.page = pagination.current ?? 1;
  pending.pageSize = pagination.pageSize ?? 10;
  selectedRowKeys.value = [];
  void loadPending();
}

function handleReportedChange(pagination: TablePagination) {
  reported.page = pagination.current ?? 1;
  reported.pageSize = pagination.pageSize ?? 10;
  void loadReported();
}

const pendingPagination = computed(() => ({
  current: pending.page,
  pageSize: pending.pageSize,
  pageSizeOptions: ['10', '20', '50'],
  showSizeChanger: true,
  showTotal: (value: number) => `共 ${value} 条`,
  total: pending.total,
}));

const reportedPagination = computed(() => ({
  current: reported.page,
  pageSize: reported.pageSize,
  pageSizeOptions: ['10', '20', '50'],
  showSizeChanger: true,
  showTotal: (value: number) => `共 ${value} 条`,
  total: reported.total,
}));

// ---------------------------------------------------------------------------
// 多选批量通过 / 按用户一键通过
// ---------------------------------------------------------------------------

const selectedRowKeys = ref<number[]>([]);

function onPendingSelect(keys: Array<number | string>) {
  selectedRowKeys.value = keys.map((key) => Number(key));
}

function batchApprove() {
  const count = selectedRowKeys.value.length;
  if (!count) return;
  Modal.confirm({
    title: '批量通过确认',
    content: `将选中的 ${count} 个待审核文件全部通过，通过后用户端立即可见。确认操作？`,
    okText: '确认通过',
    cancelText: '取消',
    onOk: async () => {
      try {
        const { approved } = await approveFiles(selectedRowKeys.value, activeKind.value);
        counters.approved += approved;
        message.success(`已通过 ${approved} 个文件`);
        selectedRowKeys.value = [];
        await loadPending();
      } catch (error) {
        message.error(error instanceof Error ? error.message : '批量通过失败');
      }
    },
  });
}

async function approveOne(record: ExamFile) {
  try {
    const { approved } = await approveFiles([record.fileId], activeKind.value);
    counters.approved += approved;
    message.success(`已通过「${record.fileName}」`);
    await loadPending();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '操作失败');
  }
}

function approveUserAll(record: ExamFile) {
  void (async () => {
    try {
      const count = await countPendingByUser(record.userId, activeKind.value);
      if (!count) {
        message.info('该用户当前没有待审核文件');
        return;
      }
      Modal.confirm({
        title: '通过该用户全部待审核文件',
        content: `用户 ${record.createBy}（ID ${record.userId}）当前共有 ${count} 个待审核文件，将全部通过并上架。确认操作？`,
        okText: '全部通过',
        cancelText: '取消',
        onOk: async () => {
          try {
            const { approved } = await approveUserFiles(record.userId, activeKind.value);
            counters.approved += approved;
            message.success(`已通过用户 ${record.createBy} 的 ${approved} 个文件`);
            selectedRowKeys.value = [];
            await loadPending();
          } catch (error) {
            message.error(error instanceof Error ? error.message : '操作失败');
          }
        },
      });
    } catch (error) {
      message.error(error instanceof Error ? error.message : '查询待审核数量失败');
    }
  })();
}

// ---------------------------------------------------------------------------
// 文件预览抽屉
// ---------------------------------------------------------------------------

const preview = reactive({
  open: false,
  loading: false,
  file: null as null | ExamFile,
  url: '',
  pageCount: undefined as number | undefined,
  page: 1,
});

const previewHeight = computed(() =>
  preview.file && previewKindOf(preview.file.fileFormat) === 'unsupported' ? 220 : 540,
);

async function openPreview(record: ExamFile) {
  preview.file = record;
  preview.url = '';
  preview.pageCount = undefined;
  preview.page = 1;
  preview.open = true;
  preview.loading = true;
  try {
    const result = await resolvePreview(record);
    preview.url = result.url;
    preview.pageCount = result.pageCount;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载文件预览失败');
    preview.open = false;
  } finally {
    preview.loading = false;
  }
}

/** 预览抽屉快捷操作：直接通过待审核文件。 */
function approveFromPreview() {
  if (!preview.file) return;
  const record = preview.file;
  preview.open = false;
  void approveOne(record);
}

/** 预览抽屉快捷操作：拒绝待审核文件（跳转原因填写）。 */
function rejectFromPreview() {
  if (!preview.file) return;
  const record = preview.file;
  preview.open = false;
  openRejectPending(record);
}

/** 预览抽屉快捷操作：复审被举报文件（跳转复审弹窗填写原因）。 */
function reReviewFromPreview(decision: AuditDecision) {
  if (!preview.file) return;
  const record = preview.file as ReportedExamFile;
  preview.open = false;
  openReReview(record, decision);
}

// ---------------------------------------------------------------------------
// 审核（拒绝 / 被举报复审）
// ---------------------------------------------------------------------------

const audit = reactive({
  open: false,
  mode: 'reported' as 'pending' | 'reported',
  file: null as null | ExamFile,
  decision: 'approve' as AuditDecision,
  reason: '',
  submitting: false,
  previewLoading: false,
  previewUrl: '',
  pageCount: undefined as number | undefined,
  page: 1,
});

const auditTitle = computed(() => {
  if (!audit.file) return '文件审核';
  return audit.mode === 'reported'
    ? `被举报文件复审 · ${audit.file.fileName}`
    : `拒绝文件 · ${audit.file.fileName}`;
});

const auditReportInfo = computed(() =>
  audit.mode === 'reported' && audit.file ? (audit.file as ReportedExamFile) : null,
);

const decisionHint = computed(() =>
  audit.decision === 'approve'
    ? '确认文件无违规内容：举报不属实，文件将恢复上架并对用户可见。'
    : '确认举报属实：文件将下架，并向用户下发整改通知。',
);

const reasonPlaceholder = computed(() => {
  if (audit.mode === 'pending') {
    return '例如：文件清晰度不足，无法识别试卷内容，请重新上传后再提交。';
  }
  return audit.decision === 'approve'
    ? '例如：经复审文件内容无违规，举报不属实，予以恢复上架。'
    : '例如：经复审举报属实，文件包含广告二维码，不予上架。';
});

function openRejectPending(record: ExamFile) {
  audit.mode = 'pending';
  audit.file = record;
  audit.decision = 'reject';
  audit.reason = '';
  audit.page = 1;
  audit.previewUrl = '';
  audit.pageCount = undefined;
  audit.open = true;
  void loadAuditPreview(record);
}

function openReReview(record: ReportedExamFile, decision: AuditDecision = 'approve') {
  audit.mode = 'reported';
  audit.file = record;
  audit.decision = decision;
  audit.reason = '';
  audit.page = 1;
  audit.previewUrl = '';
  audit.pageCount = undefined;
  audit.open = true;
  void loadAuditPreview(record);
}

async function loadAuditPreview(record: ExamFile) {
  audit.previewLoading = true;
  try {
    const result = await resolvePreview(record);
    audit.previewUrl = result.url;
    audit.pageCount = result.pageCount;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载文件预览失败');
  } finally {
    audit.previewLoading = false;
  }
}

async function submitAudit() {
  if (!audit.file) return;
  const reason = audit.reason.trim();
  if (reason.length < 2) {
    message.warning('请填写审核原因（至少 2 个字符），将随结果返回给上传用户');
    return;
  }
  audit.submitting = true;
  try {
    if (audit.mode === 'reported') {
      const decision = audit.decision;
      await auditReportedFile({
        fileId: audit.file.fileId,
        decision,
        reason,
        kind: activeKind.value,
      });
      if (decision === 'approve') {
        counters.approved += 1;
        message.success('复审完成：举报不属实，文件已恢复上架');
      } else {
        counters.rejected += 1;
        message.success('复审完成：举报属实，文件已下架，原因已返回给用户');
      }
      await loadReported();
    } else {
      await rejectPendingFile({ fileId: audit.file.fileId, reason, kind: activeKind.value });
      counters.rejected += 1;
      message.success('已拒绝该文件，原因已返回给用户');
      await loadPending();
    }
    audit.open = false;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '提交审核结果失败');
  } finally {
    audit.submitting = false;
  }
}

// ---------------------------------------------------------------------------
// 表格列定义
// ---------------------------------------------------------------------------

const pendingColumns = [
  { title: 'ID', dataIndex: 'fileId', key: 'fileId', width: 70 },
  { title: '文件', key: 'file', width: 250 },
  { title: '上传用户', key: 'user', width: 120 },
  { title: '类型', key: 'fileType', width: 80 },
  { title: '学校 / 科目', key: 'origin', width: 160 },
  { title: '年份', dataIndex: 'fileYear', key: 'fileYear', width: 70 },
  { title: '大小', key: 'fileSize', width: 80 },
  { title: '上传时间', dataIndex: 'createTime', key: 'createTime', width: 150 },
  { title: '操作', key: 'action', width: 310, fixed: 'right' },
];

const reportedColumns = [
  { title: 'ID', dataIndex: 'fileId', key: 'fileId', width: 70 },
  { title: '文件', key: 'file', width: 230 },
  { title: '上传用户', key: 'user', width: 120 },
  { title: '举报原因', dataIndex: 'reportReason', key: 'reportReason', width: 230, ellipsis: true },
  { title: '举报人', dataIndex: 'reporterName', key: 'reporterName', width: 100 },
  { title: '举报时间', dataIndex: 'reportTime', key: 'reportTime', width: 150 },
  { title: '操作', key: 'action', width: 120, fixed: 'right' },
];

onMounted(() => {
  void refreshAll();
});
</script>

<template>
  <div class="page-stack">
    <section class="page-heading">
      <div>
        <h1>文件审核</h1>
        <p>待审核与被举报内容的集中审核工作台 · 试卷 ptmj_file / 电子书 ptmj_ebook</p>
      </div>
      <a-space wrap>
        <a-radio-group :value="activeKind" @update:value="switchKind">
          <a-radio-button value="exam">试卷资料</a-radio-button>
          <a-radio-button value="ebook">电子书</a-radio-button>
        </a-radio-group>
        <a-button :loading="pending.loading || reported.loading" @click="refreshAll">
          <ReloadOutlined />
          刷新
        </a-button>
      </a-space>
    </section>

    <a-row :gutter="16">
      <a-col :md="6" :xs="12">
        <div class="stat-box">
          <span>待审核文件</span>
          <strong style="color: #d46b08">{{ pending.total }}</strong>
        </div>
      </a-col>
      <a-col :md="6" :xs="12">
        <div class="stat-box">
          <span>被举报待复审</span>
          <strong style="color: #cf1322">{{ reported.total }}</strong>
        </div>
      </a-col>
      <a-col :md="6" :xs="12">
        <div class="stat-box">
          <span>本次已通过</span>
          <strong style="color: #389e0d">{{ counters.approved }}</strong>
        </div>
      </a-col>
      <a-col :md="6" :xs="12">
        <div class="stat-box">
          <span>本次已拒绝</span>
          <strong>{{ counters.rejected }}</strong>
        </div>
      </a-col>
    </a-row>

    <a-alert
      v-if="errorText"
      class="form-alert"
      type="error"
      show-icon
      closable
      :message="errorText"
      @close="errorText = ''"
    />

    <a-tabs v-model:activeKey="activeTab" size="large">
      <a-tab-pane key="pending">
        <template #tab>
          <span>待审核<span class="tab-count">{{ pending.total }}</span></span>
        </template>
        <div class="audit-tab-stack">
          <section class="panel">
            <a-form :model="pendingFilters" layout="inline" class="search-form">
              <a-form-item label="文件名">
                <a-input
                  v-model:value="pendingFilters.fileName"
                  allow-clear
                  class="control-md"
                  placeholder="文件名关键词"
                  @press-enter="searchPending"
                />
              </a-form-item>
              <a-form-item label="上传用户">
                <a-input
                  v-model:value="pendingFilters.user"
                  allow-clear
                  class="control-md"
                  placeholder="用户 ID 或昵称"
                  @press-enter="searchPending"
                />
              </a-form-item>
              <a-form-item label="文件类型">
                <a-select
                  v-model:value="pendingFilters.fileType"
                  allow-clear
                  class="control-md"
                  placeholder="文件类型"
                  :options="FILE_TYPE_OPTIONS"
                />
              </a-form-item>
              <a-form-item>
                <a-space wrap>
                  <a-button type="primary" @click="searchPending">
                    <SearchOutlined />
                    查询
                  </a-button>
                  <a-button @click="resetPendingFilters">
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
                <a-button
                  v-if="canAudit"
                  type="primary"
                  :disabled="selectedRowKeys.length === 0"
                  @click="batchApprove"
                >
                  <CheckSquareOutlined />
                  一键通过{{ selectedRowKeys.length ? `（已选 ${selectedRowKeys.length} 项）` : '' }}
                </a-button>
                <a-button
                  v-if="selectedRowKeys.length"
                  @click="selectedRowKeys = []"
                >
                  清空选择
                </a-button>
              </a-space>
              <a-tag color="blue">共 {{ pending.total }} 条</a-tag>
            </div>
            <a-table
              row-key="fileId"
              class="compact-user-table"
              size="small"
              :columns="pendingColumns"
              :data-source="pending.items"
              :loading="pending.loading"
              :pagination="pendingPagination"
              :row-selection="{ selectedRowKeys, onChange: onPendingSelect }"
              :scroll="{ x: 1240 }"
              @change="handlePendingChange"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'file'">
                  <div class="name-cell">
                    <strong :title="record.fileName">{{ record.fileName }}</strong>
                    <span>
                      {{ record.fileFormat.toUpperCase() }} · {{ formatSize(record.fileSize) }}
                    </span>
                  </div>
                </template>
                <template v-else-if="column.key === 'user'">
                  <div class="name-cell">
                    <strong>{{ record.createBy }}</strong>
                    <span>ID {{ record.userId }}</span>
                  </div>
                </template>
                <template v-else-if="column.key === 'fileType'">
                  <a-tag v-if="activeKind === 'ebook'" color="green">
                    {{ ebookTypeText(record.fileType) }}
                  </a-tag>
                  <a-tag v-else :color="fileTypeColor(record.fileType)">
                    {{ fileTypeText(record.fileType) }}
                  </a-tag>
                </template>
                <template v-else-if="column.key === 'origin'">
                  <div class="name-cell">
                    <strong>{{ activeKind === 'ebook' ? record.fileSubject : record.fileSchool }}</strong>
                    <span>{{ record.fileSubject }}</span>
                  </div>
                </template>
                <template v-else-if="column.key === 'fileSize'">
                  {{ formatSize(record.fileSize) }}
                </template>
                <template v-else-if="column.key === 'action'">
                  <a-space :size="2">
                    <a-button type="link" size="small" @click="openPreview(record)">
                      <EyeOutlined />
                      预览
                    </a-button>
                    <a-button
                      v-if="canAudit"
                      type="link"
                      size="small"
                      @click="approveOne(record)"
                    >
                      <CheckOutlined />
                      通过
                    </a-button>
                    <a-button
                      v-if="canAudit"
                      type="link"
                      size="small"
                      danger
                      @click="openRejectPending(record)"
                    >
                      <CloseOutlined />
                      拒绝
                    </a-button>
                    <a-tooltip v-if="canAudit" title="通过该用户当前全部待审核文件">
                      <a-button type="link" size="small" @click="approveUserAll(record)">
                        <UsergroupAddOutlined />
                        通过TA全部
                      </a-button>
                    </a-tooltip>
                  </a-space>
                </template>
              </template>
            </a-table>
          </section>
        </div>
      </a-tab-pane>

      <a-tab-pane key="reported">
        <template #tab>
          <span>
            被举报复审<span class="tab-count tab-count-danger">{{ reported.total }}</span>
          </span>
        </template>
        <div class="audit-tab-stack">
          <section class="panel">
            <a-form :model="reportedFilters" layout="inline" class="search-form">
              <a-form-item label="关键词">
                <a-input
                  v-model:value="reportedFilters.keyword"
                  allow-clear
                  class="control-lg"
                  placeholder="文件名或举报原因"
                  @press-enter="searchReported"
                />
              </a-form-item>
              <a-form-item>
                <a-space wrap>
                  <a-button type="primary" @click="searchReported">
                    <SearchOutlined />
                    查询
                  </a-button>
                  <a-button @click="resetReportedFilters">
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
                复审需给出审核结论与原因，结果将返回给上传用户
              </span>
              <a-tag color="volcano">共 {{ reported.total }} 条</a-tag>
            </div>
            <a-table
              row-key="fileId"
              class="compact-user-table"
              size="small"
              :columns="reportedColumns"
              :data-source="reported.items"
              :loading="reported.loading"
              :pagination="reportedPagination"
              :scroll="{ x: 1020 }"
              @change="handleReportedChange"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'file'">
                  <div class="name-cell">
                    <strong :title="record.fileName">{{ record.fileName }}</strong>
                    <span>
                      {{ record.fileFormat.toUpperCase() }} · {{ formatSize(record.fileSize) }}
                    </span>
                  </div>
                </template>
                <template v-else-if="column.key === 'user'">
                  <div class="name-cell">
                    <strong>{{ record.createBy }}</strong>
                    <span>ID {{ record.userId }}</span>
                  </div>
                </template>
                <template v-else-if="column.key === 'reportReason'">
                  <a-tooltip :title="record.reportReason">
                    <span class="report-reason">{{ record.reportReason }}</span>
                  </a-tooltip>
                </template>
                <template v-else-if="column.key === 'action'">
                  <a-space :size="2">
                    <a-button type="link" size="small" @click="openPreview(record)">
                      <EyeOutlined />
                      预览
                    </a-button>
                    <a-button
                      v-if="canAudit"
                      type="primary"
                      size="small"
                      danger
                      @click="openReReview(record)"
                    >
                      复审
                    </a-button>
                  </a-space>
                </template>
              </template>
            </a-table>
          </section>
        </div>
      </a-tab-pane>
    </a-tabs>

    <a-drawer
      v-model:open="preview.open"
      title="文件预览"
      width="820"
      :destroy-on-close="true"
    >
      <a-spin v-if="preview.file" :spinning="preview.loading">
        <a-descriptions size="small" bordered :column="2" class="audit-desc">
          <a-descriptions-item label="文件名" :span="2">
            {{ preview.file.fileName }}
          </a-descriptions-item>
          <a-descriptions-item label="上传用户">
            {{ preview.file.createBy }}（ID {{ preview.file.userId }}）
          </a-descriptions-item>
          <a-descriptions-item label="状态">
            <a-tag :color="statusMeta(preview.file.fileStatus).color">
              {{ statusMeta(preview.file.fileStatus).text }}
            </a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="学校 / 科目">
            {{ preview.file.fileSchool }} · {{ preview.file.fileSubject }}
          </a-descriptions-item>
          <a-descriptions-item label="类型 / 年份">
            {{ fileTypeText(preview.file.fileType) }} · {{ preview.file.fileYear }}
          </a-descriptions-item>
          <a-descriptions-item label="格式 / 大小">
            {{ preview.file.fileFormat.toUpperCase() }} · {{ formatSize(preview.file.fileSize) }}
          </a-descriptions-item>
          <a-descriptions-item label="上传时间">
            {{ preview.file.createTime }}
          </a-descriptions-item>
        </a-descriptions>
        <FilePreviewPager
          v-model:page="preview.page"
          :file-name="preview.file.fileName"
          :file-format="preview.file.fileFormat"
          :file-url="preview.url"
          :page-count="preview.pageCount"
          :height="previewHeight"
        />
        <a-space
          v-if="canAudit && preview.file.fileStatus === 0"
          wrap
          class="preview-actions"
        >
          <a-button type="primary" @click="approveFromPreview">
            <CheckOutlined />
            通过
          </a-button>
          <a-button danger @click="rejectFromPreview">
            <CloseOutlined />
            拒绝
          </a-button>
          <span class="muted-text">通过后文件立即上架；拒绝需填写原因并返回给用户</span>
        </a-space>
        <a-space
          v-else-if="canAudit && preview.file.fileStatus === 3"
          wrap
          class="preview-actions"
        >
          <a-button type="primary" @click="reReviewFromPreview('approve')">
            <CheckOutlined />
            复审通过（举报不属实）
          </a-button>
          <a-button danger @click="reReviewFromPreview('reject')">
            <CloseOutlined />
            复审拒绝（举报属实）
          </a-button>
          <span class="muted-text">下一步需填写审核原因，结论与原因将返回给上传用户</span>
        </a-space>
      </a-spin>
    </a-drawer>

    <a-modal
      :open="audit.open"
      :title="auditTitle"
      width="1020px"
      :confirm-loading="audit.submitting"
      :mask-closable="false"
      :destroy-on-close="true"
      :ok-text="audit.mode === 'reported' ? '提交复审结果' : '确认拒绝'"
      :ok-button-props="{ danger: audit.decision === 'reject' }"
      cancel-text="取消"
      @ok="submitAudit"
      @cancel="audit.open = false"
    >
      <a-alert
        v-if="auditReportInfo"
        class="form-alert"
        type="warning"
        show-icon
      >
        <template #message>
          举报原因：{{ auditReportInfo.reportReason }}
        </template>
        <template #description>
          举报人：{{ auditReportInfo.reporterName }} · 举报时间：{{
            auditReportInfo.reportTime
          }}
        </template>
      </a-alert>

      <a-descriptions
        v-if="audit.file"
        size="small"
        bordered
        :column="3"
        class="audit-desc"
      >
        <a-descriptions-item label="文件 ID">
          {{ audit.file.fileId }}
        </a-descriptions-item>
        <a-descriptions-item label="上传用户">
          {{ audit.file.createBy }}（ID {{ audit.file.userId }}）
        </a-descriptions-item>
        <a-descriptions-item label="类型 / 年份">
          {{ fileTypeText(audit.file.fileType) }} · {{ audit.file.fileYear }}
        </a-descriptions-item>
        <a-descriptions-item label="学校 / 科目" :span="2">
          {{ audit.file.fileSchool }} · {{ audit.file.fileSubject }}
        </a-descriptions-item>
        <a-descriptions-item label="格式 / 大小">
          {{ audit.file.fileFormat.toUpperCase() }} · {{ formatSize(audit.file.fileSize) }}
        </a-descriptions-item>
      </a-descriptions>

      <a-row :gutter="16" class="audit-modal-body">
        <a-col :lg="14" :xs="24">
          <a-spin :spinning="audit.previewLoading">
            <FilePreviewPager
              v-if="audit.file"
              v-model:page="audit.page"
              :file-name="audit.file.fileName"
              :file-format="audit.file.fileFormat"
              :file-url="audit.previewUrl"
              :page-count="audit.pageCount"
              :height="400"
            />
          </a-spin>
        </a-col>
        <a-col :lg="10" :xs="24">
          <a-form layout="vertical">
            <a-form-item v-if="audit.mode === 'reported'" label="审核结论" required>
              <a-radio-group
                v-model:value="audit.decision"
                button-style="solid"
                class="audit-decision"
              >
                <a-radio-button value="approve">通过（举报不属实）</a-radio-button>
                <a-radio-button value="reject">拒绝（举报属实）</a-radio-button>
              </a-radio-group>
              <p class="muted-text">{{ decisionHint }}</p>
            </a-form-item>
            <a-form-item label="审核原因" required>
              <a-textarea
                v-model:value="audit.reason"
                :rows="6"
                :maxlength="200"
                show-count
                :placeholder="reasonPlaceholder"
              />
              <p class="muted-text">审核结果与原因将返回给上传用户。</p>
            </a-form-item>
          </a-form>
        </a-col>
      </a-row>
    </a-modal>
  </div>
</template>

<style scoped>
.audit-tab-stack {
  display: grid;
  gap: 16px;
}

.tab-count {
  display: inline-block;
  min-width: 18px;
  margin-left: 6px;
  padding: 0 5px;
  border-radius: 9px;
  background: hsl(var(--primary) / 15%);
  color: hsl(var(--primary));
  font-size: 12px;
  line-height: 18px;
  text-align: center;
}

.tab-count-danger {
  background: #cf132226;
  color: #f5222d;
}

.report-reason {
  color: #cf1322;
}

.audit-desc {
  margin-bottom: 16px;
}

.preview-actions {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--kadmin-border, hsl(var(--border)));
}

.audit-modal-body {
  margin-top: 4px;
}

.audit-decision {
  display: flex;
  flex-wrap: wrap;
}
</style>
