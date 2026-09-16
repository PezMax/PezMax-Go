import { request } from './client';

export interface DictionaryType {
  id: number;
  name: string;
  code: string;
  description?: string;
  sort: number;
  status: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface DictionaryData {
  id: number;
  dictType: string;
  label: string;
  value: string;
  color?: string;
  cssClass?: string;
  isDefault: boolean;
  sort: number;
  status: number;
  remark?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface DictionaryListResponse<T> {
  items: T[];
  total: number;
}

export interface DictionaryOverview {
  types: DictionaryType[];
  data: DictionaryData[];
}

export interface DictionaryTypePayload {
  name: string;
  code: string;
  description?: string;
  sort?: number;
  status?: number;
}

export interface DictionaryDataPayload {
  dictType: string;
  label: string;
  value: string;
  color?: string;
  cssClass?: string;
  isDefault?: boolean;
  sort?: number;
  status?: number;
  remark?: string;
}

export interface DictionaryFilters {
  keyword?: string;
  status?: number | string;
}

export interface DictionaryDataFilters extends DictionaryFilters {
  dictType?: string;
}

export function getDictionaryOverview() {
  return request<DictionaryOverview>('/api/dictionaries/overview');
}

export function getDictionaryTypes(filters: DictionaryFilters = {}) {
  const params = new URLSearchParams();
  if (filters.keyword) params.set('keyword', filters.keyword);
  if (filters.status !== undefined && filters.status !== '') params.set('status', String(filters.status));
  const query = params.toString();
  return request<DictionaryListResponse<DictionaryType>>(
    `/api/dictionaries/types${query ? `?${query}` : ''}`,
  );
}

export function createDictionaryType(payload: DictionaryTypePayload) {
  return request<DictionaryType>('/api/dictionaries/types', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateDictionaryType(id: number, payload: DictionaryTypePayload) {
  return request<DictionaryType>(`/api/dictionaries/types/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteDictionaryType(id: number) {
  return request<boolean>(`/api/dictionaries/types/${id}`, {
    method: 'DELETE',
  });
}

export function getDictionaryData(filters: DictionaryDataFilters = {}) {
  const params = new URLSearchParams();
  if (filters.dictType) params.set('dictType', filters.dictType);
  if (filters.keyword) params.set('keyword', filters.keyword);
  if (filters.status !== undefined && filters.status !== '') params.set('status', String(filters.status));
  const query = params.toString();
  return request<DictionaryListResponse<DictionaryData>>(
    `/api/dictionaries/data${query ? `?${query}` : ''}`,
  );
}

export function createDictionaryData(payload: DictionaryDataPayload) {
  return request<DictionaryData>('/api/dictionaries/data', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateDictionaryData(id: number, payload: DictionaryDataPayload) {
  return request<DictionaryData>(`/api/dictionaries/data/${id}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteDictionaryData(id: number) {
  return request<boolean>(`/api/dictionaries/data/${id}`, {
    method: 'DELETE',
  });
}
