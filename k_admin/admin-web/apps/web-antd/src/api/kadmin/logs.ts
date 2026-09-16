import { request } from './client';

export interface ManagedLog {
  action: string;
  actorName: string;
  createdAt: string;
  durationMs: null | number;
  errorCode: string;
  errorMessage: string;
  eventId: string;
  eventType: string;
  expiresAt: string;
  id: number;
  input: string;
  ip: string;
  level: string;
  message: string;
  metadata: Record<string, unknown>;
  method: string;
  module: string;
  occurredAt: string;
  path: string;
  requestId: string;
  source: string;
  statusCode: null | number;
  success: boolean | null;
  traceId: string;
  userAgent: string;
  userId: null | number;
}

export interface LogListFilters {
  endedAt?: string;
  eventType?: string;
  keyword?: string;
  level?: string;
  method?: string;
  page?: number;
  pageSize?: number;
  source?: string;
  startedAt?: string;
  statusCode?: number;
  success?: boolean;
}

export interface LogListResult {
  items: ManagedLog[];
  page: number;
  pageSize: number;
  total: number;
}

export function getLogs(filters: LogListFilters = {}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      params.set(key, String(value));
    }
  });
  const query = params.toString();
  return request<LogListResult>(`/api/logs${query ? `?${query}` : ''}`);
}

export function getLog(id: number) {
  return request<ManagedLog>(`/api/logs/${id}`);
}

export function deleteLog(id: number) {
  return request<boolean>(`/api/logs/${id}`, { method: 'DELETE' });
}

export function deleteLogs(ids: number[]) {
  return request<boolean>('/api/logs', {
    body: JSON.stringify({ ids }),
    method: 'DELETE',
  });
}
