// Composite-key favorites are maintained manually; generic codegen is single-key.
import { request } from '../client';

export interface EbookFavorite {
  ebookId: number;
  userId: number;
}

export interface EbookFavoritePayload {
  ebookId: number;
  userId: number;
}

export interface EbookFavoriteFilters {
  page?: number;
  pageSize?: number;
}

export interface PageResult<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export function ebookFavoriteKey(key: EbookFavorite) {
  if (!Number.isSafeInteger(key.ebookId) || key.ebookId <= 0 || !Number.isSafeInteger(key.userId) || key.userId <= 0) {
    throw new Error('资源 ID 和用户 ID 必须为正整数');
  }
  return `${key.ebookId},${key.userId}`;
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

export function getEbookFavoriteList(filters: EbookFavoriteFilters = {}) {
  return request<PageResult<EbookFavorite>>(`/api/ebook-favorites${queryString(filters)}`);
}

export function getEbookFavorite(key: EbookFavorite) {
  return request<EbookFavorite>(`/api/ebook-favorites/${ebookFavoriteKey(key)}`);
}

export function createEbookFavorite(payload: EbookFavoritePayload) {
  return request<EbookFavorite>(`/api/ebook-favorites`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateEbookFavorite(key: EbookFavorite, payload: EbookFavoritePayload) {
  return request<EbookFavorite>(`/api/ebook-favorites/${ebookFavoriteKey(key)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteEbookFavorite(key: EbookFavorite) {
  return request<boolean>(`/api/ebook-favorites/${ebookFavoriteKey(key)}`, { method: 'DELETE' });
}
