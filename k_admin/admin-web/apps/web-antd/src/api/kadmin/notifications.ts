import { request } from './client';

export type NotificationType = 'info' | 'success' | 'warning';

export interface KadminNotification {
  content: string;
  createdAt: string;
  id: number;
  isRead: boolean;
  link: string;
  title: string;
  type: NotificationType;
}

export interface NotificationPayload {
  content?: string;
  link?: string;
  title: string;
  type?: NotificationType;
}

export interface NotificationPage {
  items: KadminNotification[];
  page: number;
  pageSize: number;
  total: number;
  unread: number;
}

function queryString(filters: {
  page?: number;
  pageSize?: number;
  unreadOnly?: boolean;
}) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  });
  const query = params.toString();
  return query ? `?${query}` : '';
}

export function getNotifications(
  filters: { page?: number; pageSize?: number; unreadOnly?: boolean } = {},
) {
  return request<NotificationPage>(`/api/notifications${queryString(filters)}`);
}

export function createNotification(payload: NotificationPayload) {
  return request<KadminNotification>('/api/notifications', {
    body: JSON.stringify(payload),
    method: 'POST',
  });
}

export function markNotificationRead(id: number) {
  return request<KadminNotification>(`/api/notifications/${id}/read`, {
    method: 'PATCH',
  });
}

export function markAllNotificationsRead() {
  return request<boolean>('/api/notification-batch/read-all', {
    method: 'PATCH',
  });
}

export function deleteNotification(id: number) {
  return request<boolean>(`/api/notifications/${id}`, { method: 'DELETE' });
}

export function clearReadNotifications() {
  return request<boolean>('/api/notification-batch/read', {
    method: 'DELETE',
  });
}
