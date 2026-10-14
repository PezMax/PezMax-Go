// Composite-key favorites are maintained manually; generic codegen is single-key.
import { request } from '../client';

export interface BookmarkFavorite {
  bookmarkId: number;
  userId: number;
}

export interface BookmarkFavoritePayload {
  bookmarkId: number;
  userId: number;
}

export interface BookmarkFavoriteFilters {
  page?: number;
  pageSize?: number;
}

export interface PageResult<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export function bookmarkFavoriteKey(key: BookmarkFavorite) {
  if (!Number.isSafeInteger(key.bookmarkId) || key.bookmarkId <= 0 || !Number.isSafeInteger(key.userId) || key.userId <= 0) {
    throw new Error('资源 ID 和用户 ID 必须为正整数');
  }
  return `${key.bookmarkId},${key.userId}`;
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

export function getBookmarkFavoriteList(filters: BookmarkFavoriteFilters = {}) {
  return request<PageResult<BookmarkFavorite>>(`/api/bookmark-favorites${queryString(filters)}`);
}

export function getBookmarkFavorite(key: BookmarkFavorite) {
  return request<BookmarkFavorite>(`/api/bookmark-favorites/${bookmarkFavoriteKey(key)}`);
}

export function createBookmarkFavorite(payload: BookmarkFavoritePayload) {
  return request<BookmarkFavorite>(`/api/bookmark-favorites`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateBookmarkFavorite(key: BookmarkFavorite, payload: BookmarkFavoritePayload) {
  return request<BookmarkFavorite>(`/api/bookmark-favorites/${bookmarkFavoriteKey(key)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteBookmarkFavorite(key: BookmarkFavorite) {
  return request<boolean>(`/api/bookmark-favorites/${bookmarkFavoriteKey(key)}`, { method: 'DELETE' });
}
