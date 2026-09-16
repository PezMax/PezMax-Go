import { request } from './client';

export type JobHandler = 'cache_refresh' | 'log_cleanup';
export type JobStatus = 'enabled' | 'paused';
export type JobExecutionStatus = 'failed' | 'running' | 'success';
export type JobTrigger = 'manual' | 'scheduled';

export interface ScheduledJob {
  builtIn: boolean;
  createdAt: string;
  createdBy: number;
  cronExpression: string;
  description: string;
  handler: JobHandler;
  id: number;
  lastRunAt: string;
  name: string;
  nextRunAt: string;
  parameters: Record<string, unknown>;
  status: JobStatus;
  updatedAt: string;
}

export interface JobPayload {
  cronExpression: string;
  description: string;
  handler: JobHandler;
  name: string;
  parameters: Record<string, unknown>;
  status: JobStatus;
}

export interface JobExecution {
  createdAt: string;
  durationMs: number;
  error: string;
  finishedAt: string;
  handler: JobHandler;
  id: number;
  jobId: null | number;
  jobName: string;
  output: string;
  startedAt: string;
  status: JobExecutionStatus;
  trigger: JobTrigger;
  triggeredBy: null | number;
}

export interface PageResult<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface JobFilters {
  handler?: JobHandler;
  keyword?: string;
  page?: number;
  pageSize?: number;
  status?: JobStatus;
}

export interface ExecutionFilters {
  jobId?: number;
  keyword?: string;
  page?: number;
  pageSize?: number;
  status?: JobExecutionStatus;
  trigger?: JobTrigger;
}

function queryString<T extends object>(filters: T) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      params.set(key, String(value));
    }
  });
  const query = params.toString();
  return query ? `?${query}` : '';
}

export function getJobs(filters: JobFilters = {}) {
  return request<PageResult<ScheduledJob>>(`/api/jobs${queryString(filters)}`);
}

export function getJob(id: number) {
  return request<ScheduledJob>(`/api/jobs/${id}`);
}

export function createJob(payload: JobPayload) {
  return request<ScheduledJob>('/api/jobs', {
    body: JSON.stringify(payload),
    method: 'POST',
  });
}

export function updateJob(id: number, payload: JobPayload) {
  return request<ScheduledJob>(`/api/jobs/${id}`, {
    body: JSON.stringify(payload),
    method: 'PUT',
  });
}

export function updateJobStatus(id: number, status: JobStatus) {
  return request<ScheduledJob>(`/api/jobs/${id}/status`, {
    body: JSON.stringify({ status }),
    method: 'PATCH',
  });
}

export function deleteJob(id: number) {
  return request<boolean>(`/api/jobs/${id}`, { method: 'DELETE' });
}

export function runJob(id: number) {
  return request<JobExecution>(`/api/jobs/${id}/run`, { method: 'POST' });
}

export function getJobExecutions(filters: ExecutionFilters = {}) {
  return request<PageResult<JobExecution>>(
    `/api/job-logs${queryString(filters)}`,
  );
}

export function getJobExecution(id: number) {
  return request<JobExecution>(`/api/job-logs/${id}`);
}
