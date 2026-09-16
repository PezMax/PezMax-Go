import { useAccessStore } from '@vben/stores';

const API_BASE = import.meta.env.VITE_GLOB_API_URL || '/api';
let refreshAccessTokenTask: null | Promise<string> = null;

export interface ApiEnvelope<T> {
  code: number;
  message?: string;
  msg?: string;
  data: T;
}

interface LoginResult {
  accessToken?: string;
  refreshToken?: string;
}

export async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  return requestWithAuth<T>(path, init, false);
}

export async function requestBlob(
  path: string,
  init: RequestInit = {},
): Promise<Blob> {
  return requestBlobWithAuth(path, init, false);
}

async function requestWithAuth<T>(
  path: string,
  init: RequestInit,
  hasRetried: boolean,
): Promise<T> {
  const { payload, response } = await sendRequest(path, init);

  if (response.status === 401 && !hasRetried && useAccessStore().refreshToken) {
    await refreshAccessToken();
    return requestWithAuth<T>(path, init, true);
  }

  if (!response.ok) {
    if (response.status === 401) {
      markLoginExpired();
      throw new Error('登录已过期，请重新登录');
    }
    throw new Error(
      getPayloadMessage(payload) || response.statusText || '请求失败',
    );
  }

  if (payload && typeof payload === 'object' && 'code' in payload) {
    const envelope = payload as ApiEnvelope<T>;
    if (envelope.code !== 0) {
      throw new Error(envelope.message || envelope.msg || '请求失败');
    }
    return envelope.data;
  }

  return payload as T;
}

async function requestBlobWithAuth(
  path: string,
  init: RequestInit,
  hasRetried: boolean,
): Promise<Blob> {
  const response = await sendResponse(path, init);

  if (response.status === 401 && !hasRetried && useAccessStore().refreshToken) {
    await refreshAccessToken();
    return requestBlobWithAuth(path, init, true);
  }

  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    if (response.status === 401) {
      markLoginExpired();
      throw new Error('登录已过期，请重新登录');
    }
    throw new Error(
      getPayloadMessage(payload) || response.statusText || '请求失败',
    );
  }

  return response.blob();
}

async function sendRequest(path: string, init: RequestInit) {
  const response = await sendResponse(path, init);
  const payload = await response.json().catch(() => null);

  return { payload, response };
}

async function sendResponse(path: string, init: RequestInit) {
  const headers = new Headers(init.headers);
  const hasBody = init.body !== undefined && init.body !== null;

  if (
    hasBody &&
    !(init.body instanceof FormData) &&
    !headers.has('Content-Type')
  ) {
    headers.set('Content-Type', 'application/json');
  }

  const token = useAccessStore().accessToken;
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  const method = String(init.method || 'GET').toUpperCase();
  if (
    ['DELETE', 'PATCH', 'POST', 'PUT'].includes(method) &&
    !headers.has('Idempotency-Key')
  ) {
    headers.set(
      'Idempotency-Key',
      globalThis.crypto?.randomUUID?.() ??
        `${Date.now()}-${Math.random().toString(16).slice(2)}`,
    );
  }

  const response = await fetch(buildApiUrl(path), {
    ...init,
    headers,
  });
  return response;
}

function refreshAccessToken(): Promise<string> {
  refreshAccessTokenTask ??= refreshAccessTokenOnce().finally(() => {
    refreshAccessTokenTask = null;
  });
  return refreshAccessTokenTask;
}

async function refreshAccessTokenOnce(): Promise<string> {
  const accessStore = useAccessStore();
  const refreshToken = accessStore.refreshToken;
  if (!refreshToken) {
    markLoginExpired();
    throw new Error('登录已过期，请重新登录');
  }

  const response = await fetch(buildApiUrl('/auth/refresh'), {
    body: JSON.stringify({ refreshToken }),
    headers: {
      'Content-Type': 'application/json',
    },
    method: 'POST',
  });
  const payload = await response.json().catch(() => null);

  if (!response.ok) {
    markLoginExpired();
    throw new Error('登录已过期，请重新登录');
  }

  if (payload && typeof payload === 'object' && 'code' in payload) {
    const envelope = payload as ApiEnvelope<LoginResult>;
    if (envelope.code !== 0) {
      markLoginExpired();
      throw new Error(
        envelope.message || envelope.msg || '登录已过期，请重新登录',
      );
    }
    if (!envelope.data?.accessToken) {
      markLoginExpired();
      throw new Error('刷新登录状态失败');
    }
    accessStore.setAccessToken(envelope.data.accessToken);
    accessStore.setRefreshToken(envelope.data.refreshToken || refreshToken);
    return envelope.data.accessToken;
  }

  markLoginExpired();
  throw new Error('刷新登录状态失败');
}

function markLoginExpired() {
  const accessStore = useAccessStore();
  accessStore.setAccessToken(null);
  accessStore.setRefreshToken(null);
  accessStore.setLoginExpired(true);
}

function buildApiUrl(path: string) {
  const base = API_BASE.replace(/\/$/, '');
  const apiPath = path.startsWith('/api/') ? path.slice(4) : path;
  return `${base}${apiPath.startsWith('/') ? apiPath : `/${apiPath}`}`;
}

function getPayloadMessage(payload: unknown) {
  if (!payload || typeof payload !== 'object') {
    return '';
  }
  const data = payload as { message?: string; msg?: string };
  return data.message || data.msg || '';
}
