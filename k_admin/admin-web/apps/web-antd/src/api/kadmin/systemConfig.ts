import { request } from './client';

export interface SystemConfigItem {
  key: string;
  value: string;
  label?: string;
  type?: 'text' | 'password' | 'boolean' | 'select';
  options?: string[];
  description?: string;
  builtin: boolean;
  future?: boolean;
}

export interface SystemConfigOverview {
  filePath: string;
  items: SystemConfigItem[];
}

export interface LoginConfig {
  captchaEnabled: boolean;
}

export function getSystemConfig() {
  return request<SystemConfigOverview>('/api/system/config');
}

export function getLoginConfig() {
  return request<LoginConfig>('/api/system/config/login');
}

export function updateSystemConfig(items: SystemConfigItem[]) {
  return request<SystemConfigOverview>('/api/system/config', {
    method: 'PUT',
    body: JSON.stringify({
      items: items.map((item) => ({
        key: item.key,
        value: item.value,
      })),
    }),
  });
}
