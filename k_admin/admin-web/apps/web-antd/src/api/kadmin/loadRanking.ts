import { request } from './client';

export interface LoadRankingStatus {
  bucketSeconds: number;
  enabled: boolean;
  flushIntervalSeconds: number;
  lastError: string;
  lastFlushAt: string;
  retentionDays: number;
}

export interface LoadRankingItem {
  avgDurationMs: number;
  errorCount: number;
  errorRate: number;
  maxDurationMs: number;
  method: string;
  qps: number;
  requestCount: number;
  route: string;
  statusCode: null | number;
}

export interface LoadRankingResult {
  endedAt: string;
  items: LoadRankingItem[];
  page: number;
  pageSize: number;
  startedAt: string;
  total: number;
  windowSeconds: number;
}

export type LoadRankingGroupBy = 'method' | 'route' | 'status';

export type LoadRankingDimension =
  | 'avgDurationMs'
  | 'errorRate'
  | 'qps'
  | 'requestCount';

export interface LoadRankingQuery {
  dimension?: LoadRankingDimension;
  endedAt?: string;
  groupBy?: LoadRankingGroupBy;
  method?: string;
  order?: 'asc' | 'desc';
  page?: number;
  pageSize?: number;
  route?: string;
  startedAt?: string;
  statusCode?: number;
}

export function getLoadRankingStatus() {
  return request<LoadRankingStatus>('/api/load-ranking/status');
}

export function updateLoadRankingStatus(enabled: boolean) {
  return request<LoadRankingStatus>('/api/load-ranking/status', {
    body: JSON.stringify({ enabled }),
    method: 'PATCH',
  });
}

export function getLoadRankings(query: LoadRankingQuery = {}) {
  const params = new URLSearchParams();
  if (query.startedAt) params.set('startedAt', query.startedAt);
  if (query.endedAt) params.set('endedAt', query.endedAt);
  if (query.route) params.set('route', query.route);
  if (query.method) params.set('method', query.method);
  if (query.statusCode !== undefined && query.statusCode !== null) {
    params.set('statusCode', String(query.statusCode));
  }
  if (query.groupBy) params.set('groupBy', query.groupBy);
  if (query.dimension) params.set('dimension', query.dimension);
  if (query.order) params.set('order', query.order);
  if (query.page !== undefined) params.set('page', String(query.page));
  if (query.pageSize !== undefined) {
    params.set('pageSize', String(query.pageSize));
  }
  const queryString = params.toString();
  return request<LoadRankingResult>(
    `/api/load-ranking/rankings${queryString ? `?${queryString}` : ''}`,
  );
}
