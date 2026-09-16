import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request } from './client';
import { deleteLog, deleteLogs, getLog, getLogs } from './logs';

vi.mock('./client', () => ({
  request: vi.fn(),
}));

const requestMock = vi.mocked(request);

describe('log API', () => {
  beforeEach(() => {
    requestMock.mockReset();
  });

  it('serializes server-side pagination and filters', async () => {
    requestMock.mockResolvedValueOnce({ items: [], page: 2, pageSize: 50, total: 0 });

    await getLogs({
      keyword: 'login',
      page: 2,
      pageSize: 50,
      success: false,
    });

    expect(requestMock).toHaveBeenCalledWith(
      '/api/logs?keyword=login&page=2&pageSize=50&success=false',
    );
  });

  it('uses detail and delete endpoints', async () => {
    requestMock.mockResolvedValue(true);

    await getLog(8);
    await deleteLog(8);
    await deleteLogs([8, 9]);

    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/logs/8');
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/logs/8', {
      method: 'DELETE',
    });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/api/logs', {
      body: JSON.stringify({ ids: [8, 9] }),
      method: 'DELETE',
    });
  });
});
