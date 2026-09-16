<template>
  <div class="page-stack monitor-page">
    <section class="page-heading">
      <div>
        <h1>系统监控</h1>
        <p>实时查看服务器与当前服务进程的运行状态</p>
      </div>
      <a-space class="monitor-actions" wrap>
        <a-space class="monitor-switch">
          <span>监控开关</span>
          <a-switch
            :checked="status.enabled"
            :disabled="!canUpdate"
            :loading="switchLoading"
            checked-children="开"
            un-checked-children="关"
            @change="toggleMonitor"
          />
        </a-space>
        <a-tooltip title="刷新监控数据">
          <a-button
            class="monitor-refresh-button"
            shape="circle"
            :loading="refreshLoading"
            aria-label="刷新监控数据"
            @click="refreshStatus"
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
      message="系统监控已关闭"
      description="关闭后服务端不会采集系统指标，页面也不会自动轮询，可减少不必要的性能开销。"
    />
    <a-alert v-else-if="status.lastError" type="warning" show-icon :message="status.lastError" />

    <a-spin :spinning="initialLoading">
      <template v-if="status.enabled && metrics">
        <a-row :gutter="[16, 16]">
          <a-col :xs="24" :sm="12" :xl="6">
            <section class="panel metric-card">
              <div class="metric-heading">
                <span>CPU 占用</span>
                <DashboardOutlined />
              </div>
              <strong>{{ formatPercent(metrics.cpu.usagePercent) }}</strong>
              <a-progress
                :percent="metrics.cpu.usagePercent"
                :show-info="false"
                :status="progressStatus(metrics.cpu.usagePercent)"
              />
              <small
                >物理 {{ metrics.cpu.physicalCores }} 核 / 逻辑
                {{ metrics.cpu.logicalCores }} 核</small
              >
            </section>
          </a-col>
          <a-col :xs="24" :sm="12" :xl="6">
            <section class="panel metric-card">
              <div class="metric-heading">
                <span>内存占用</span>
                <DatabaseOutlined />
              </div>
              <strong>{{ formatPercent(metrics.memory.usagePercent) }}</strong>
              <a-progress
                :percent="metrics.memory.usagePercent"
                :show-info="false"
                :status="progressStatus(metrics.memory.usagePercent)"
              />
              <small
                >{{ formatBytes(metrics.memory.usedBytes) }} /
                {{ formatBytes(metrics.memory.totalBytes) }}</small
              >
            </section>
          </a-col>
          <a-col :xs="24" :sm="12" :xl="6">
            <section class="panel metric-card">
              <div class="metric-heading">
                <span>服务器 IP</span>
                <GlobalOutlined />
              </div>
              <strong class="metric-address">{{ metrics.host.serverIp || '-' }}</strong>
              <small>{{ metrics.host.hostname || '未获取主机名' }}</small>
            </section>
          </a-col>
          <a-col :xs="24" :sm="12" :xl="6">
            <section class="panel metric-card">
              <div class="metric-heading">
                <span>服务器运行时间</span>
                <ClockCircleOutlined />
              </div>
              <strong>{{ formatDuration(metrics.host.uptimeSeconds) }}</strong>
              <small>服务已运行 {{ formatDuration(metrics.application.uptimeSeconds) }}</small>
            </section>
          </a-col>
        </a-row>

        <a-row :gutter="[16, 16]">
          <a-col :xs="24" :xl="12">
            <section class="panel detail-panel">
              <div class="panel-title"><h2>服务器信息</h2></div>
              <a-descriptions bordered size="small" :column="descriptionColumns">
                <a-descriptions-item label="主机名">{{
                  metrics.host.hostname || '-'
                }}</a-descriptions-item>
                <a-descriptions-item label="服务器 IP">{{
                  metrics.host.serverIp || '-'
                }}</a-descriptions-item>
                <a-descriptions-item label="操作系统">{{ operatingSystem }}</a-descriptions-item>
                <a-descriptions-item label="系统架构">{{
                  metrics.host.architecture || '-'
                }}</a-descriptions-item>
                <a-descriptions-item label="内核版本">{{
                  metrics.host.kernelVersion || '-'
                }}</a-descriptions-item>
                <a-descriptions-item label="启动时间">{{
                  formatDateTime(metrics.host.bootTime)
                }}</a-descriptions-item>
                <a-descriptions-item label="全部 IP" :span="descriptionColumns">
                  <a-space wrap>
                    <a-tag v-for="address in metrics.host.ipAddresses" :key="address">{{
                      address
                    }}</a-tag>
                    <span v-if="metrics.host.ipAddresses.length === 0">-</span>
                  </a-space>
                </a-descriptions-item>
              </a-descriptions>
            </section>
          </a-col>
          <a-col :xs="24" :xl="12">
            <section class="panel detail-panel">
              <div class="panel-title"><h2>服务进程</h2></div>
              <a-descriptions bordered size="small" :column="descriptionColumns">
                <a-descriptions-item label="进程 ID">{{
                  metrics.application.pid
                }}</a-descriptions-item>
                <a-descriptions-item label="Go 版本">{{
                  metrics.application.goVersion
                }}</a-descriptions-item>
                <a-descriptions-item label="协程数量">{{
                  metrics.application.goroutines
                }}</a-descriptions-item>
                <a-descriptions-item label="内存占用">{{
                  formatBytes(metrics.application.memoryBytes)
                }}</a-descriptions-item>
                <a-descriptions-item label="堆内存">{{
                  formatBytes(metrics.application.heapAllocBytes)
                }}</a-descriptions-item>
                <a-descriptions-item label="GC 次数">{{
                  metrics.application.gcCount
                }}</a-descriptions-item>
              </a-descriptions>
            </section>
          </a-col>
        </a-row>

        <div class="sample-time">
          指标采样于 {{ formatDateTime(metrics.sampledAt) }}，每
          {{ status.samplingIntervalSeconds }} 秒更新
        </div>
      </template>
      <a-empty v-else-if="status.enabled && !initialLoading" description="正在等待首次指标采样" />
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import {
  ClockCircleOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  ReloadOutlined,
} from '@ant-design/icons-vue';
import { useAccess } from '@vben/access';
import { message } from 'ant-design-vue';
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';

import {
  getSystemMonitor,
  updateSystemMonitorStatus,
  type SystemMonitorStatus,
} from '#/api/kadmin/systemMonitor';
import { KADMIN_PERMISSION } from '#/api/kadmin/permissions';

const { hasAccessByCodes } = useAccess();
const canUpdate = computed(() => hasAccessByCodes([KADMIN_PERMISSION.MONITOR_UPDATE, '*']));
const status = reactive<SystemMonitorStatus>({
  enabled: false,
  lastError: '',
  metrics: null,
  samplingIntervalSeconds: 3,
});
const initialLoading = ref(true);
const refreshLoading = ref(false);
const switchLoading = ref(false);
const apiError = ref('');
const viewportWidth = ref(window.innerWidth);
const metrics = computed(() => status.metrics);
const descriptionColumns = computed(() => (viewportWidth.value < 768 ? 1 : 2));
const operatingSystem = computed(() => {
  if (!metrics.value) return '-';
  return (
    [metrics.value.host.platform, metrics.value.host.platformVersion].filter(Boolean).join(' ') ||
    metrics.value.host.os ||
    '-'
  );
});
let pollTimer: number | undefined;

onMounted(() => {
  window.addEventListener('resize', updateViewportWidth);
  void loadStatus();
});

onUnmounted(() => {
  window.removeEventListener('resize', updateViewportWidth);
  stopPolling();
});

async function loadStatus(showRefreshLoading = false) {
  if (showRefreshLoading) refreshLoading.value = true;
  apiError.value = '';
  try {
    applyStatus(await getSystemMonitor());
  } catch (error) {
    apiError.value = error instanceof Error ? error.message : '加载系统监控失败';
  } finally {
    if (showRefreshLoading) refreshLoading.value = false;
    initialLoading.value = false;
  }
}

function refreshStatus() {
  return loadStatus(true);
}

async function toggleMonitor(checked: boolean | string | number) {
  switchLoading.value = true;
  apiError.value = '';
  try {
    const enabled = Boolean(checked);
    applyStatus(await updateSystemMonitorStatus(enabled));
    message.success(enabled ? '系统监控已开启' : '系统监控已关闭');
    if (enabled) await loadStatus();
  } catch (error) {
    apiError.value = error instanceof Error ? error.message : '更新监控开关失败';
  } finally {
    switchLoading.value = false;
  }
}

function applyStatus(next: SystemMonitorStatus) {
  Object.assign(status, next);
  if (status.enabled) startPolling();
  else stopPolling();
}

function startPolling() {
  if (pollTimer) return;
  const interval = Math.max(1, status.samplingIntervalSeconds) * 1000;
  pollTimer = window.setInterval(() => void loadStatus(), interval);
}

function stopPolling() {
  if (!pollTimer) return;
  window.clearInterval(pollTimer);
  pollTimer = undefined;
}

function updateViewportWidth() {
  viewportWidth.value = window.innerWidth;
}

function formatPercent(value: number) {
  return `${value.toFixed(1)}%`;
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const exponent = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  return `${(value / 1024 ** exponent).toFixed(exponent === 0 ? 0 : 2)} ${units[exponent]}`;
}

function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return '-';
  const days = Math.floor(seconds / 86_400);
  const hours = Math.floor((seconds % 86_400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const parts = [];
  if (days) parts.push(`${days} 天`);
  if (hours || days) parts.push(`${hours} 小时`);
  parts.push(`${minutes} 分钟`);
  return parts.join(' ');
}

function formatDateTime(value: string) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function progressStatus(percent: number) {
  return percent >= 90 ? 'exception' : 'normal';
}
</script>

<style scoped>
.monitor-page :deep(.ant-spin-container) {
  display: grid;
  gap: 16px;
}
.monitor-switch {
  min-height: 32px;
  padding-inline: 12px;
  border: 1px solid var(--kadmin-border);
  border-radius: 8px;
  background: var(--kadmin-bg);
}
.monitor-actions {
  flex-wrap: nowrap;
}
.monitor-refresh-button {
  width: 32px;
  min-width: 32px;
  height: 32px;
  padding: 0;
}
.metric-card {
  min-height: 164px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 10px;
}
.metric-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--kadmin-muted);
  font-size: 14px;
}
.metric-heading :deep(svg) {
  color: var(--kadmin-active-text);
  font-size: 20px;
}
.metric-card strong {
  overflow: hidden;
  color: var(--kadmin-text);
  font-size: 28px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.metric-card small {
  overflow: hidden;
  color: var(--kadmin-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.metric-card .metric-address {
  font-family: Consolas, 'SFMono-Regular', monospace;
  font-size: 23px;
}
.detail-panel {
  height: 100%;
}
.sample-time {
  color: var(--kadmin-muted);
  font-size: 12px;
  text-align: right;
}
@media (max-width: 768px) {
  .monitor-actions {
    flex-wrap: wrap;
  }
  .monitor-switch {
    padding-inline: 0;
    border: 0;
    background: transparent;
  }
  .sample-time {
    text-align: left;
  }
}
</style>
