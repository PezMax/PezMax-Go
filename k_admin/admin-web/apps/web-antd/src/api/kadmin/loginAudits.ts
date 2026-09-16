import { request } from './client';

export type LoginAuditStatus = 'failed' | 'success';
export type LoginAuditResult =
  | 'account_disabled'
  | 'account_locked'
  | 'account_unlocked'
  | 'captcha_invalid'
  | 'account_not_found'
  | 'invalid_password'
  | 'success'
  | 'system_error';

export interface LoginAudit {
  account: string;
  browser: string;
  createdAt: string;
  durationMs: number;
  failureReason: string;
  id: number;
  ip: string;
  occurredAt: string;
  os: string;
  result: LoginAuditResult;
  status: LoginAuditStatus;
  userAgent: string;
  userId: null | number;
}

export interface LoginAuditFilters {
  account?: string;
  endedAt?: string;
  ip?: string;
  page?: number;
  pageSize?: number;
  result?: LoginAuditResult;
  startedAt?: string;
  status?: LoginAuditStatus;
}

export interface LoginAuditListResult {
  items: LoginAudit[];
  page: number;
  pageSize: number;
  total: number;
}

export interface LoginAuditRetention {
  days: number;
  updatedAt: string;
  updatedBy: number;
}

export interface LoginAuditCleanupResult {
  deletedCount: number;
}

export function getLoginAudits(filters: LoginAuditFilters = {}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') params.set(key, String(value));
  });
  const query = params.toString();
  return request<LoginAuditListResult>(
    `/api/login-audits${query ? `?${query}` : ''}`,
  );
}

export function deleteLoginAudits(ids: number[]) {
  return request<LoginAuditCleanupResult>('/api/login-audits', {
    body: JSON.stringify({ ids }),
    method: 'DELETE',
  });
}

export function cleanupExpiredLoginAudits() {
  return request<LoginAuditCleanupResult>('/api/login-audits/cleanup', {
    method: 'POST',
  });
}

export function getLoginAuditRetention() {
  return request<LoginAuditRetention>('/api/login-audits/retention');
}

export function updateLoginAuditRetention(days: number) {
  return request<LoginAuditRetention>('/api/login-audits/retention', {
    body: JSON.stringify({ days }),
    method: 'PATCH',
  });
}
