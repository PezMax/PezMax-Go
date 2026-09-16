import { request } from './client';

export interface SystemMonitorStatus {
  enabled: boolean;
  lastError: string;
  metrics: null | SystemMetrics;
  samplingIntervalSeconds: number;
}

export interface SystemMetrics {
  application: ApplicationMetrics;
  cpu: CpuMetrics;
  host: HostMetrics;
  memory: MemoryMetrics;
  sampledAt: string;
}

export interface CpuMetrics {
  logicalCores: number;
  physicalCores: number;
  usagePercent: number;
}

export interface MemoryMetrics {
  availableBytes: number;
  totalBytes: number;
  usagePercent: number;
  usedBytes: number;
}

export interface HostMetrics {
  architecture: string;
  bootTime: string;
  hostname: string;
  ipAddresses: string[];
  kernelVersion: string;
  os: string;
  platform: string;
  platformVersion: string;
  serverIp: string;
  uptimeSeconds: number;
}

export interface ApplicationMetrics {
  gcCount: number;
  goVersion: string;
  goroutines: number;
  heapAllocBytes: number;
  heapSystemBytes: number;
  memoryBytes: number;
  pid: number;
  uptimeSeconds: number;
}

export function getSystemMonitor() {
  return request<SystemMonitorStatus>('/api/system-monitor');
}

export function updateSystemMonitorStatus(enabled: boolean) {
  return request<SystemMonitorStatus>('/api/system-monitor/status', {
    body: JSON.stringify({ enabled }),
    method: 'PATCH',
  });
}
