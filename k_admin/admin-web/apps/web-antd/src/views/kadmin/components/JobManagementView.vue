<template>
  <div class="page-stack job-page">
    <section class="page-heading">
      <div>
        <h1>定时任务</h1>
        <p>任务调度与执行记录</p>
      </div>
      <a-space wrap>
        <a-button :loading="activeLoading" @click="refreshActiveTab">
          <ReloadOutlined />
          刷新
        </a-button>
        <a-button v-if="canCreate" type="primary" @click="openCreate">
          <PlusOutlined />
          新建任务
        </a-button>
      </a-space>
    </section>

    <a-tabs
      v-model:active-key="activeTab"
      class="job-tabs"
      @change="handleTabChange"
    >
      <a-tab-pane key="tasks" tab="任务管理">
        <section class="panel">
          <a-form :model="taskFilters" layout="inline" class="search-form">
            <a-form-item label="关键词">
              <a-input
                v-model:value="taskFilters.keyword"
                allow-clear
                class="control-lg"
                placeholder="任务名称"
                @press-enter="searchTasks"
              >
                <template #prefix><SearchOutlined /></template>
              </a-input>
            </a-form-item>
            <a-form-item label="类型">
              <a-select
                v-model:value="taskFilters.handler"
                allow-clear
                class="control-md"
                :options="handlerOptions"
                placeholder="全部"
              />
            </a-form-item>
            <a-form-item label="状态">
              <a-select
                v-model:value="taskFilters.status"
                allow-clear
                class="control-md"
                :options="statusOptions"
                placeholder="全部"
              />
            </a-form-item>
            <a-form-item>
              <a-space wrap>
                <a-button type="primary" @click="searchTasks"
                  ><SearchOutlined />查询</a-button
                >
                <a-button @click="resetTaskSearch"
                  ><ClearOutlined />重置</a-button
                >
              </a-space>
            </a-form-item>
          </a-form>
        </section>

        <a-alert
          v-if="taskError"
          class="form-alert"
          type="error"
          show-icon
          closable
          :message="taskError"
          @close="taskError = ''"
        />

        <section class="panel">
          <div class="table-toolbar">
            <a-space wrap>
              <a-tag color="green">本页启用 {{ taskSummary.enabled }}</a-tag>
              <a-tag>本页暂停 {{ taskSummary.paused }}</a-tag>
              <a-tag color="blue">本页内置 {{ taskSummary.builtIn }}</a-tag>
            </a-space>
          </div>
          <a-table
            row-key="id"
            class="compact-user-table"
            size="small"
            :columns="taskColumns"
            :data-source="tasks"
            :loading="taskLoading"
            :pagination="taskPagination"
            :scroll="{ x: 1180 }"
            @change="handleTaskTableChange"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <div class="job-name-cell">
                  <a-space :size="6">
                    <strong>{{ record.name }}</strong>
                    <a-tag v-if="record.builtIn" color="blue">内置</a-tag>
                  </a-space>
                  <span>{{ handlerLabel(record.handler) }}</span>
                </div>
              </template>
              <template v-else-if="column.key === 'cron'">
                <a-typography-text class="job-code" copyable>
                  {{ record.cronExpression }}
                </a-typography-text>
              </template>
              <template v-else-if="column.key === 'status'">
                <div class="status-switch-cell">
                  <a-switch
                    :checked="record.status === 'enabled'"
                    :disabled="!canUpdate || statusUpdatingIds.has(record.id)"
                    :loading="statusUpdatingIds.has(record.id)"
                    checked-children="启用"
                    un-checked-children="暂停"
                    @change="toggleStatus(record, $event)"
                  />
                </div>
              </template>
              <template v-else-if="column.key === 'lastRunAt'">
                {{ record.lastRunAt || '-' }}
              </template>
              <template v-else-if="column.key === 'nextRunAt'">
                {{
                  record.status === 'enabled' ? record.nextRunAt || '-' : '-'
                }}
              </template>
              <template v-else-if="column.key === 'description'">
                <a-typography-text
                  :content="record.description || '-'"
                  ellipsis
                />
              </template>
              <template v-else-if="column.key === 'action'">
                <a-space :size="2">
                  <a-popconfirm
                    v-if="canRun"
                    title="确认立即执行该任务？"
                    @confirm="executeNow(record)"
                  >
                    <a-tooltip title="立即执行">
                      <a-button
                        type="text"
                        shape="circle"
                        :loading="runningIds.has(record.id)"
                      >
                        <PlayCircleOutlined />
                      </a-button>
                    </a-tooltip>
                  </a-popconfirm>
                  <a-tooltip v-if="canUpdate" title="编辑">
                    <a-button
                      type="text"
                      shape="circle"
                      @click="openEdit(record)"
                    >
                      <EditOutlined />
                    </a-button>
                  </a-tooltip>
                  <a-popconfirm
                    v-if="canDelete && !record.builtIn"
                    title="确认删除该任务？执行日志会继续保留。"
                    @confirm="removeTask(record)"
                  >
                    <a-tooltip title="删除">
                      <a-button type="text" shape="circle" danger
                        ><DeleteOutlined
                      /></a-button>
                    </a-tooltip>
                  </a-popconfirm>
                </a-space>
              </template>
            </template>
          </a-table>
        </section>
      </a-tab-pane>

      <a-tab-pane v-if="canViewLogs" key="logs" tab="任务日志">
        <section class="panel">
          <a-form :model="logFilters" layout="inline" class="search-form">
            <a-form-item label="关键词">
              <a-input
                v-model:value="logFilters.keyword"
                allow-clear
                class="control-lg"
                placeholder="任务名称 / 输出 / 错误"
                @press-enter="searchLogs"
              >
                <template #prefix><SearchOutlined /></template>
              </a-input>
            </a-form-item>
            <a-form-item label="结果">
              <a-select
                v-model:value="logFilters.status"
                allow-clear
                class="control-md"
                :options="executionStatusOptions"
                placeholder="全部"
              />
            </a-form-item>
            <a-form-item label="触发方式">
              <a-select
                v-model:value="logFilters.trigger"
                allow-clear
                class="control-md"
                :options="triggerOptions"
                placeholder="全部"
              />
            </a-form-item>
            <a-form-item>
              <a-space wrap>
                <a-button type="primary" @click="searchLogs"
                  ><SearchOutlined />查询</a-button
                >
                <a-button @click="resetLogSearch"
                  ><ClearOutlined />重置</a-button
                >
              </a-space>
            </a-form-item>
          </a-form>
        </section>

        <a-alert
          v-if="logError"
          class="form-alert"
          type="error"
          show-icon
          closable
          :message="logError"
          @close="logError = ''"
        />

        <section class="panel">
          <a-table
            row-key="id"
            class="compact-user-table"
            size="small"
            :columns="logColumns"
            :data-source="executions"
            :loading="logLoading"
            :pagination="logPagination"
            :scroll="{ x: 1040 }"
            @change="handleLogTableChange"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'jobName'">
                <div class="job-name-cell">
                  <strong>{{ record.jobName }}</strong>
                  <span>{{ handlerLabel(record.handler) }}</span>
                </div>
              </template>
              <template v-else-if="column.key === 'status'">
                <a-tag :color="executionStatusMeta(record.status).color">
                  {{ executionStatusMeta(record.status).label }}
                </a-tag>
              </template>
              <template v-else-if="column.key === 'trigger'">
                {{ record.trigger === 'manual' ? '手动' : '定时' }}
              </template>
              <template v-else-if="column.key === 'duration'">
                {{ formatDuration(record.durationMs) }}
              </template>
              <template v-else-if="column.key === 'summary'">
                <a-typography-text
                  :content="record.error || record.output || '-'"
                  :type="record.status === 'failed' ? 'danger' : undefined"
                  ellipsis
                />
              </template>
              <template v-else-if="column.key === 'action'">
                <a-tooltip title="查看详情">
                  <a-button
                    type="text"
                    shape="circle"
                    @click="openExecution(record)"
                  >
                    <EyeOutlined />
                  </a-button>
                </a-tooltip>
              </template>
            </template>
          </a-table>
        </section>
      </a-tab-pane>
    </a-tabs>

    <a-drawer
      :open="editorOpen"
      :title="editingTask ? '编辑任务' : '新建任务'"
      width="min(560px, 100vw)"
      :destroy-on-close="true"
      :mask-closable="false"
      @close="attemptCloseEditor"
    >
      <a-form
        ref="editorFormRef"
        :model="editor"
        :rules="editorRules"
        layout="vertical"
      >
        <a-form-item label="任务名称" name="name">
          <a-input
            v-model:value="editor.name"
            :disabled="Boolean(editingTask?.builtIn)"
          />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :xs="24" :sm="14">
            <a-form-item label="任务类型" name="handler">
              <a-select
                v-model:value="editor.handler"
                :disabled="Boolean(editingTask?.builtIn)"
                :options="handlerOptions"
              />
            </a-form-item>
          </a-col>
          <a-col :xs="24" :sm="10">
            <a-form-item label="状态" name="status">
              <a-segmented
                v-model:value="editor.status"
                block
                :options="editorStatusOptions"
              />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="Cron 表达式" name="cronExpression">
          <a-input
            v-model:value="editor.cronExpression"
            class="job-code-input"
          />
        </a-form-item>
        <template v-if="editor.handler === 'log_cleanup'">
          <a-row :gutter="16">
            <a-col :xs="24" :sm="12">
              <a-form-item label="请求日志保留天数" name="retentionDays">
                <a-input-number
                  v-model:value="editor.retentionDays"
                  :min="1"
                  :max="3650"
                  class="full-width"
                />
              </a-form-item>
            </a-col>
            <a-col :xs="24" :sm="12">
              <a-form-item label="任务日志保留天数" name="taskLogRetentionDays">
                <a-input-number
                  v-model:value="editor.taskLogRetentionDays"
                  :min="1"
                  :max="3650"
                  class="full-width"
                />
              </a-form-item>
            </a-col>
          </a-row>
        </template>
        <a-form-item label="备注" name="description">
          <a-textarea
            v-model:value="editor.description"
            :maxlength="500"
            :rows="3"
            show-count
          />
        </a-form-item>
      </a-form>
      <template #extra>
        <a-space>
          <a-button @click="attemptCloseEditor">取消</a-button>
          <a-button type="primary" :loading="editorSaving" @click="saveTask">
            保存
          </a-button>
        </a-space>
      </template>
    </a-drawer>

    <a-drawer
      :open="detailOpen"
      title="执行详情"
      width="min(680px, 94vw)"
      @close="detailOpen = false"
    >
      <a-spin :spinning="detailLoading">
        <a-descriptions
          v-if="detailExecution"
          bordered
          size="small"
          :column="1"
        >
          <a-descriptions-item label="任务">{{
            detailExecution.jobName
          }}</a-descriptions-item>
          <a-descriptions-item label="任务类型">{{
            handlerLabel(detailExecution.handler)
          }}</a-descriptions-item>
          <a-descriptions-item label="执行结果">
            <a-tag :color="executionStatusMeta(detailExecution.status).color">
              {{ executionStatusMeta(detailExecution.status).label }}
            </a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="触发方式">
            {{ detailExecution.trigger === 'manual' ? '手动' : '定时' }}
          </a-descriptions-item>
          <a-descriptions-item label="开始时间">{{
            detailExecution.startedAt || '-'
          }}</a-descriptions-item>
          <a-descriptions-item label="结束时间">{{
            detailExecution.finishedAt || '-'
          }}</a-descriptions-item>
          <a-descriptions-item label="耗时">{{
            formatDuration(detailExecution.durationMs)
          }}</a-descriptions-item>
          <a-descriptions-item label="输出">
            <pre class="log-detail-pre">{{
              detailExecution.output || '-'
            }}</pre>
          </a-descriptions-item>
          <a-descriptions-item v-if="detailExecution.error" label="错误">
            <pre class="log-detail-pre job-error-pre">{{
              detailExecution.error
            }}</pre>
          </a-descriptions-item>
        </a-descriptions>
      </a-spin>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import {
  ClearOutlined,
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message, Modal } from 'ant-design-vue';
import type { FormInstance } from 'ant-design-vue';
import { computed, onMounted, reactive, ref } from 'vue';

import {
  createJob,
  deleteJob,
  getJobExecution,
  getJobExecutions,
  getJobs,
  runJob,
  updateJob,
  updateJobStatus,
} from '#/api/kadmin/jobs';
import type {
  JobExecution,
  JobExecutionStatus,
  JobHandler,
  JobPayload,
  JobStatus,
  JobTrigger,
  ScheduledJob,
} from '#/api/kadmin/jobs';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

type TablePagination = { current?: number; pageSize?: number };

const { hasAccessByCodes } = useAccess();
const canCreate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.JOB_CREATE, '*']),
);
const canUpdate = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.JOB_UPDATE, '*']),
);
const canDelete = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.JOB_DELETE, '*']),
);
const canRun = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.JOB_RUN, '*']),
);
const canViewLogs = computed(() =>
  hasAccessByCodes([KADMIN_PERMISSION.JOB_LOG_LIST, '*']),
);

const activeTab = ref('tasks');
const taskLoading = ref(false);
const logLoading = ref(false);
const taskError = ref('');
const logError = ref('');
const tasks = ref<ScheduledJob[]>([]);
const executions = ref<JobExecution[]>([]);
const statusUpdatingIds = ref(new Set<number>());
const runningIds = ref(new Set<number>());

const taskFilters = reactive<{
  handler?: JobHandler;
  keyword: string;
  status?: JobStatus;
}>({ keyword: '' });
const logFilters = reactive<{
  keyword: string;
  status?: JobExecutionStatus;
  trigger?: JobTrigger;
}>({ keyword: '' });
const taskPagination = reactive({
  current: 1,
  pageSize: 20,
  pageSizeOptions: ['10', '20', '50', '100'],
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 项`,
  total: 0,
});
const logPagination = reactive({
  current: 1,
  pageSize: 20,
  pageSizeOptions: ['10', '20', '50', '100'],
  showSizeChanger: true,
  showTotal: (total: number) => `共 ${total} 条`,
  total: 0,
});

const handlerOptions = [
  { label: '日志清理', value: 'log_cleanup' },
  { label: '系统缓存刷新', value: 'cache_refresh' },
];
const statusOptions = [
  { label: '运行中', value: 'enabled' },
  { label: '已暂停', value: 'paused' },
];
const editorStatusOptions = [
  { label: '启用', value: 'enabled' },
  { label: '暂停', value: 'paused' },
];
const executionStatusOptions = [
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
];
const triggerOptions = [
  { label: '手动', value: 'manual' },
  { label: '定时', value: 'scheduled' },
];

const taskColumns = [
  { title: '任务', key: 'name', width: 210, fixed: 'left' },
  { title: 'Cron 表达式', key: 'cron', width: 160 },
  { title: '状态', key: 'status', width: 130 },
  { title: '上次执行', key: 'lastRunAt', width: 170 },
  { title: '下次执行', key: 'nextRunAt', width: 170 },
  { title: '备注', key: 'description', width: 240 },
  { title: '操作', key: 'action', width: 130, fixed: 'right' },
];
const logColumns = [
  {
    title: '开始时间',
    dataIndex: 'startedAt',
    key: 'startedAt',
    width: 170,
    fixed: 'left',
  },
  { title: '任务', key: 'jobName', width: 200 },
  { title: '结果', key: 'status', width: 90 },
  { title: '触发', key: 'trigger', width: 80 },
  { title: '耗时', key: 'duration', width: 100 },
  { title: '输出 / 错误', key: 'summary', width: 320 },
  { title: '操作', key: 'action', width: 70, fixed: 'right' },
];

const taskSummary = computed(() => ({
  builtIn: tasks.value.filter((task) => task.builtIn).length,
  enabled: tasks.value.filter((task) => task.status === 'enabled').length,
  paused: tasks.value.filter((task) => task.status === 'paused').length,
}));
const activeLoading = computed(() =>
  activeTab.value === 'tasks' ? taskLoading.value : logLoading.value,
);

const editorOpen = ref(false);
const editorSaving = ref(false);
const editingTask = ref<ScheduledJob | null>(null);
const editorFormRef = ref<FormInstance>();
const editorSnapshot = ref('');
const editor = reactive({
  cronExpression: '0 2 * * *',
  description: '',
  handler: 'log_cleanup' as JobHandler,
  name: '',
  retentionDays: 30,
  status: 'paused' as JobStatus,
  taskLogRetentionDays: 90,
});
const editorRules = {
  cronExpression: [{ required: true, message: '请输入 Cron 表达式' }],
  handler: [{ required: true, message: '请选择任务类型' }],
  name: [
    { required: true, message: '请输入任务名称' },
    { max: 100, message: '最多 100 个字符' },
  ],
  retentionDays: [{ required: true, message: '请输入保留天数' }],
  taskLogRetentionDays: [{ required: true, message: '请输入保留天数' }],
};

const detailOpen = ref(false);
const detailLoading = ref(false);
const detailExecution = ref<JobExecution | null>(null);

onMounted(() => void loadTasks());

async function loadTasks() {
  taskLoading.value = true;
  taskError.value = '';
  try {
    const data = await getJobs({
      handler: taskFilters.handler,
      keyword: taskFilters.keyword.trim(),
      page: taskPagination.current,
      pageSize: taskPagination.pageSize,
      status: taskFilters.status,
    });
    tasks.value = data.items || [];
    taskPagination.total = data.total || 0;
  } catch (error) {
    taskError.value = error instanceof Error ? error.message : '加载任务失败';
  } finally {
    taskLoading.value = false;
  }
}

async function loadLogs() {
  if (!canViewLogs.value) return;
  logLoading.value = true;
  logError.value = '';
  try {
    const data = await getJobExecutions({
      keyword: logFilters.keyword.trim(),
      page: logPagination.current,
      pageSize: logPagination.pageSize,
      status: logFilters.status,
      trigger: logFilters.trigger,
    });
    executions.value = data.items || [];
    logPagination.total = data.total || 0;
  } catch (error) {
    logError.value =
      error instanceof Error ? error.message : '加载任务日志失败';
  } finally {
    logLoading.value = false;
  }
}

function refreshActiveTab() {
  return activeTab.value === 'tasks' ? loadTasks() : loadLogs();
}

function handleTabChange(key: string | number) {
  if (key === 'logs' && executions.value.length === 0) void loadLogs();
}

function searchTasks() {
  taskPagination.current = 1;
  void loadTasks();
}

function resetTaskSearch() {
  taskFilters.keyword = '';
  taskFilters.handler = undefined;
  taskFilters.status = undefined;
  searchTasks();
}

function searchLogs() {
  logPagination.current = 1;
  void loadLogs();
}

function resetLogSearch() {
  logFilters.keyword = '';
  logFilters.status = undefined;
  logFilters.trigger = undefined;
  searchLogs();
}

function handleTaskTableChange(pagination: TablePagination) {
  taskPagination.current = pagination.current || 1;
  taskPagination.pageSize = pagination.pageSize || 20;
  void loadTasks();
}

function handleLogTableChange(pagination: TablePagination) {
  logPagination.current = pagination.current || 1;
  logPagination.pageSize = pagination.pageSize || 20;
  void loadLogs();
}

async function toggleStatus(
  task: ScheduledJob,
  checked: boolean | string | number,
) {
  statusUpdatingIds.value = new Set(statusUpdatingIds.value).add(task.id);
  try {
    await updateJobStatus(task.id, checked ? 'enabled' : 'paused');
    message.success(checked ? '任务已恢复' : '任务已暂停');
    await loadTasks();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '更新任务状态失败');
  } finally {
    const next = new Set(statusUpdatingIds.value);
    next.delete(task.id);
    statusUpdatingIds.value = next;
  }
}

async function executeNow(task: ScheduledJob) {
  runningIds.value = new Set(runningIds.value).add(task.id);
  try {
    const execution = await runJob(task.id);
    if (execution.status === 'failed')
      message.error(execution.error || '任务执行失败');
    else message.success(execution.output || '任务执行成功');
    await loadTasks();
    if (canViewLogs.value) await loadLogs();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '任务执行失败');
  } finally {
    const next = new Set(runningIds.value);
    next.delete(task.id);
    runningIds.value = next;
  }
}

async function removeTask(task: ScheduledJob) {
  try {
    await deleteJob(task.id);
    message.success('任务已删除');
    if (tasks.value.length === 1 && taskPagination.current > 1)
      taskPagination.current -= 1;
    await loadTasks();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除任务失败');
  }
}

function openCreate() {
  editingTask.value = null;
  Object.assign(editor, {
    cronExpression: '0 2 * * *',
    description: '',
    handler: 'log_cleanup',
    name: '',
    retentionDays: 30,
    status: 'paused',
    taskLogRetentionDays: 90,
  });
  openEditor();
}

function openEdit(task: ScheduledJob) {
  editingTask.value = task;
  Object.assign(editor, {
    cronExpression: task.cronExpression,
    description: task.description,
    handler: task.handler,
    name: task.name,
    retentionDays: numberParameter(task, 'retentionDays', 30),
    status: task.status,
    taskLogRetentionDays: numberParameter(task, 'taskLogRetentionDays', 90),
  });
  openEditor();
}

function openEditor() {
  editorOpen.value = true;
  editorSnapshot.value = editorState();
  requestAnimationFrame(() => editorFormRef.value?.clearValidate());
}

function attemptCloseEditor() {
  if (editorState() === editorSnapshot.value) {
    editorOpen.value = false;
    return;
  }
  Modal.confirm({
    content: '当前修改尚未保存。',
    okText: '放弃修改',
    okType: 'danger',
    title: '关闭编辑窗口？',
    onOk: () => {
      editorOpen.value = false;
    },
  });
}

async function saveTask() {
  await editorFormRef.value?.validate();
  editorSaving.value = true;
  const payload: JobPayload = {
    cronExpression: editor.cronExpression.trim(),
    description: editor.description.trim(),
    handler: editor.handler,
    name: editor.name.trim(),
    parameters:
      editor.handler === 'log_cleanup'
        ? {
            retentionDays: editor.retentionDays,
            taskLogRetentionDays: editor.taskLogRetentionDays,
          }
        : {},
    status: editor.status,
  };
  try {
    await (editingTask.value
      ? updateJob(editingTask.value.id, payload)
      : createJob(payload));
    message.success(editingTask.value ? '任务已更新' : '任务已创建');
    editorOpen.value = false;
    await loadTasks();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存任务失败');
  } finally {
    editorSaving.value = false;
  }
}

async function openExecution(execution: JobExecution) {
  detailOpen.value = true;
  detailLoading.value = true;
  detailExecution.value = execution;
  try {
    detailExecution.value = await getJobExecution(execution.id);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载执行详情失败');
  } finally {
    detailLoading.value = false;
  }
}

function editorState() {
  return JSON.stringify(editor);
}

function numberParameter(task: ScheduledJob, key: string, fallback: number) {
  const value = Number(task.parameters?.[key]);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function handlerLabel(handler: JobHandler) {
  return (
    handlerOptions.find((option) => option.value === handler)?.label || handler
  );
}

function executionStatusMeta(status: JobExecutionStatus) {
  if (status === 'success') return { color: 'green', label: '成功' };
  if (status === 'failed') return { color: 'red', label: '失败' };
  return { color: 'blue', label: '执行中' };
}

function formatDuration(durationMs: number) {
  if (durationMs < 1000) return `${durationMs} ms`;
  return `${(durationMs / 1000).toFixed(durationMs < 10_000 ? 2 : 1)} s`;
}
</script>

<style scoped>
.job-tabs {
  min-width: 0;
}
.job-tabs :deep(.ant-tabs-content-holder),
.job-tabs :deep(.ant-tabs-tabpane) {
  min-width: 0;
}
.job-tabs .panel + .panel {
  margin-top: 16px;
}
.job-name-cell {
  min-width: 0;
  display: grid;
  gap: 3px;
}
.job-name-cell strong,
.job-name-cell span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.job-name-cell span {
  color: var(--kadmin-muted);
  font-size: 12px;
}
.job-code,
.job-code-input {
  font-family: Consolas, 'SFMono-Regular', monospace;
}
.job-code {
  font-size: 12px;
}
.job-error-pre {
  color: #dc2626;
}
@media (max-width: 768px) {
  .job-tabs :deep(.ant-tabs-nav) {
    padding-inline: 2px;
  }
}
</style>
