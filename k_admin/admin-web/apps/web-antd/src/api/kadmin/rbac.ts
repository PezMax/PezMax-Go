import { request } from './client';

export interface RbacMenu {
  id: number;
  parentId: number;
  title: string;
  icon?: string;
  uri?: string;
  children?: RbacMenu[];
}

export interface RbacPermission {
  id: number;
  name: string;
  slug: string;
  httpMethod: string;
  httpPath: string;
  pageUri?: string;
  pageTitle?: string;
  scope: 'page' | 'button' | 'api' | string;
  button?: string;
}

export interface RbacUser {
  id: number;
  username: string;
  name: string;
  avatar?: string;
  roleIds: number[];
  roles: string[];
}

export interface RbacRole {
  id: number;
  name: string;
  slug: string;
  isAdmin: boolean;
  menuIds: number[];
  permissionIds: number[];
  userIds: number[];
  menus: RbacMenu[];
  permissions: RbacPermission[];
  users: RbacUser[];
  createdAt?: string;
  updatedAt?: string;
}

export interface RbacDepartment {
  id: number;
  name: string;
  code: string;
  description?: string;
  sort: number;
  status: number;
  roleIds: number[];
  roles: RbacRole[];
  createdAt?: string;
  updatedAt?: string;
}

export interface RbacOverview {
  roles: RbacRole[];
  menus: RbacMenu[];
  users: RbacUser[];
  departments: RbacDepartment[];
  permissions: RbacPermission[];
}

export function getRbacOverview() {
  return request<RbacOverview>('/api/rbac/overview');
}

export function createDepartment(payload: {
  name: string;
  code?: string;
  description?: string;
  sort?: number;
  status?: number;
}) {
  return request<RbacDepartment>('/api/rbac/departments', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateDepartment(
  id: number,
  payload: {
    name: string;
    code?: string;
    description?: string;
    sort?: number;
    status?: number;
  }
) {
  return request<RbacDepartment>(`/api/rbac/departments/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteDepartment(id: number) {
  return request<boolean>(`/api/rbac/departments/${id}`, {
    method: 'DELETE',
  });
}

export function updateDepartmentRoles(id: number, roleIds: number[]) {
  return request<RbacDepartment>(`/api/rbac/departments/${id}/roles`, {
    method: 'PUT',
    body: JSON.stringify({ roleIds }),
  });
}

export function createRole(payload: { name: string; slug?: string }) {
  return request<RbacRole>('/api/rbac/roles', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateRole(id: number, payload: { name: string; slug: string }) {
  return request<RbacRole>(`/api/rbac/roles/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteRole(id: number) {
  return request<boolean>(`/api/rbac/roles/${id}`, {
    method: 'DELETE',
  });
}

export function updateRoleMenus(id: number, menuIds: number[]) {
  return request<RbacRole>(`/api/rbac/roles/${id}/menus`, {
    method: 'PUT',
    body: JSON.stringify({ menuIds }),
  });
}

export function updateRolePermissions(id: number, permissionIds: number[]) {
  return request<RbacRole>(`/api/rbac/roles/${id}/permissions`, {
    method: 'PUT',
    body: JSON.stringify({ permissionIds }),
  });
}

export function updateRoleUsers(id: number, userIds: number[]) {
  return request<RbacRole>(`/api/rbac/roles/${id}/users`, {
    method: 'PUT',
    body: JSON.stringify({ userIds }),
  });
}
