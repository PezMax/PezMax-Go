import { request } from './client';

export const ADMIN_MENU_TYPE = {
  DIRECTORY: 0,
  MENU: 1,
  EXTERNAL: 2,
} as const;

export type AdminMenuType =
  (typeof ADMIN_MENU_TYPE)[keyof typeof ADMIN_MENU_TYPE];

export interface AdminMenu {
  id: number;
  parentId: number;
  type: AdminMenuType;
  order: number;
  title: string;
  icon: string;
  uri: string;
  component?: string;
  header?: string;
  pluginName?: string;
  uuid?: string;
  createdAt?: string;
  updatedAt?: string;
  children?: AdminMenu[];
}

export interface AdminMenuPayload {
  parentId?: number;
  type?: AdminMenuType;
  order?: number;
  title: string;
  icon?: string;
  uri?: string;
  component?: string;
  header?: string;
  pluginName?: string;
  uuid?: string;
}

export interface AdminMenuPosition {
  id: number;
  parentId: number;
  order: number;
}

export function getAdminMenuTree() {
  return request<AdminMenu[]>('/api/admin-menus/tree');
}

export function createAdminMenu(payload: AdminMenuPayload) {
  return request<AdminMenu>('/api/admin-menus', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateAdminMenu(id: number, payload: AdminMenuPayload) {
  return request<AdminMenu>(`/api/admin-menus/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export async function updateAdminMenuLayout(
  items: AdminMenuPosition[],
): Promise<AdminMenu[]> {
  const result = await request<AdminMenu[] | boolean>('/api/admin-menus', {
    method: 'PUT',
    body: JSON.stringify({ items }),
  });

  // Older API versions return true after persisting the layout. Always expose
  // the current tree to callers so a mixed frontend/backend deployment cannot
  // corrupt the reactive menu state with a boolean value.
  return Array.isArray(result) ? result : getAdminMenuTree();
}

export function deleteAdminMenu(id: number) {
  return request<boolean>(`/api/admin-menus/${id}`, {
    method: 'DELETE',
  });
}
