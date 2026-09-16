import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useAccessStore } from '@vben/stores';

import { request, requestBlob } from './client';

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    status,
  });
}

describe('kadmin request client', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('refreshes access token and retries once after 401', async () => {
    const accessStore = useAccessStore();
    accessStore.setAccessToken('old-access-token');
    accessStore.setRefreshToken('old-refresh-token');

    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({ code: 1, message: 'invalid token' }, 401),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: {
            accessToken: 'new-access-token',
            refreshToken: 'new-refresh-token',
          },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ code: 0, data: { ok: true } }));
    vi.stubGlobal('fetch', fetchMock);

    await expect(request<{ ok: boolean }>('/api/logs')).resolves.toEqual({
      ok: true,
    });

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/auth/refresh');
    expect(accessStore.accessToken).toBe('new-access-token');
    expect(accessStore.refreshToken).toBe('new-refresh-token');
    expect(
      (fetchMock.mock.calls[2]?.[1]?.headers as Headers).get('Authorization'),
    ).toBe('Bearer new-access-token');
  });

  it('downloads authenticated blobs after refreshing an expired token', async () => {
    const accessStore = useAccessStore();
    accessStore.setAccessToken('old-access-token');
    accessStore.setRefreshToken('old-refresh-token');
    const expected = new Blob(['avatar'], { type: 'image/png' });

    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({ code: 1, message: 'invalid token' }, 401),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: { accessToken: 'new-access-token' },
        }),
      )
      .mockResolvedValueOnce(new Response(expected, { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const actual = await requestBlob('/api/files/7/content');

    await expect(actual.text()).resolves.toBe('avatar');
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(
      (fetchMock.mock.calls[2]?.[1]?.headers as Headers).get('Authorization'),
    ).toBe('Bearer new-access-token');
  });

  it('adds an idempotency key to mutation requests', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ code: 0, data: { id: 7 } }));
    vi.stubGlobal('fetch', fetchMock);

    await request('/api/users', {
      body: JSON.stringify({ username: 'operator' }),
      method: 'POST',
    });

    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get('Idempotency-Key')).toMatch(/^[\w.:-]{8,128}$/);
  });

  it('preserves a caller-provided idempotency key', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ code: 0, data: { id: 8 } }));
    vi.stubGlobal('fetch', fetchMock);

    await request('/api/users', {
      body: JSON.stringify({ username: 'operator' }),
      headers: { 'Idempotency-Key': 'caller-request-42' },
      method: 'POST',
    });

    const headers = fetchMock.mock.calls[0]?.[1]?.headers as Headers;
    expect(headers.get('Idempotency-Key')).toBe('caller-request-42');
  });
});
