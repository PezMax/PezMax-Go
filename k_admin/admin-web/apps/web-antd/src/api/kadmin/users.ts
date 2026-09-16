import { useAccessStore } from '@vben/stores';

import { request } from './client';

const API_BASE = import.meta.env.VITE_GLOB_API_URL || '/api';

export interface ManagedUser {
  id: number;
  username: string;
  name: string;
  avatar?: string;
  status: string;
  roleIds: number[];
  roles: string[];
  departmentIds: number[];
  departments: string[];
  createdAt?: string;
  updatedAt?: string;
}

export interface ManagedUserListResponse {
  items: ManagedUser[];
  total: number;
}
export interface ManagedUserRoleOption {
  id: number;
  name: string;
  slug: string;
}

export interface ManagedUserDepartmentOption {
  id: number;
  name: string;
  roleIds: number[];
}

export interface ManagedUserOptions {
  roles: ManagedUserRoleOption[];
  departments: ManagedUserDepartmentOption[];
}

export interface UserPayload {
  username: string;
  password?: string;
  name?: string;
  avatar?: string;
  status?: string;
  roleIds?: number[];
}

export interface UserListFilters {
  keyword?: string;
  department?: string;
  role?: string;
  status?: string;
}

export function getUsers(filters: UserListFilters = {}) {
  const params = new URLSearchParams();
  if (filters.keyword) params.set('keyword', filters.keyword);
  if (filters.department) params.set('department', filters.department);
  if (filters.role) params.set('role', filters.role);
  if (filters.status) params.set('status', filters.status);
  const query = params.toString();
  return request<ManagedUserListResponse>(
    `/api/users${query ? `?${query}` : ''}`,
  );
}
export function getUserOptions() {
  return request<ManagedUserOptions>('/api/users/options');
}

export function createUser(payload: UserPayload & { password: string }) {
  return request<ManagedUser>('/api/users', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateUser(id: number, payload: UserPayload) {
  return request<ManagedUser>(`/api/users/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function updateUserStatus(id: number, status: string) {
  return request<ManagedUser>(`/api/users/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify({ status }),
  });
}

export function deleteUser(id: number) {
  return request<boolean>(`/api/users/${id}`, {
    method: 'DELETE',
  });
}

export function resetUserPassword(id: number, password: string) {
  return request<boolean>(`/api/users/${id}/password`, {
    method: 'PUT',
    body: JSON.stringify({ password }),
  });
}

export function unlockUser(id: number, ip = '') {
  return request<boolean>(`/api/users/${id}/unlock`, {
    method: 'PUT',
    body: JSON.stringify({ ip }),
  });
}

export type UserImportExportFormat = 'csv' | 'sql' | 'xlsx';

export function importUsers(format: UserImportExportFormat, content: string) {
  return request<{ imported: number }>('/api/users/import', {
    method: 'POST',
    body: JSON.stringify({ format, content }),
  });
}

export async function exportUsers(format: UserImportExportFormat) {
  const headers = new Headers();
  const token = useAccessStore().accessToken;
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE}/users/export?format=${format}`, {
    headers,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(
      payload?.message || payload?.msg || response.statusText || '导出失败',
    );
  }

  const blob = await response.blob();
  const fallbackName =
    format === 'sql'
      ? 'users.sql'
      : format === 'xlsx'
        ? 'users.xlsx'
        : 'users.csv';
  const disposition = response.headers.get('Content-Disposition') || '';
  const match = disposition.match(/filename="?([^";]+)"?/i);
  return {
    blob,
    filename: match?.[1] || fallbackName,
  };
}
