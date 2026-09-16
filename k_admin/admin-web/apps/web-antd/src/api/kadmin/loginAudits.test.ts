import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request } from './client';
import {
  cleanupExpiredLoginAudits,
  deleteLoginAudits,
  getLoginAuditRetention,
  getLoginAudits,
  updateLoginAuditRetention,
} from './loginAudits';

vi.mock('./client', () => ({ request: vi.fn() }));

const requestMock = vi.mocked(request);

describe('login audit API', () => {
  beforeEach(() => requestMock.mockReset());

  it('serializes filters', async () => {
    requestMock.mockResolvedValueOnce({ items: [], page: 2, pageSize: 50, total: 0 });
    await getLoginAudits({ account: 'admin', ip: '127.0.0.1', page: 2, pageSize: 50, status: 'failed' });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/login-audits?account=admin&ip=127.0.0.1&page=2&pageSize=50&status=failed',
    );
  });

  it('uses cleanup and retention endpoints', async () => {
    requestMock.mockResolvedValue({ deletedCount: 1 });
    await deleteLoginAudits([1, 2]);
    await cleanupExpiredLoginAudits();
    await getLoginAuditRetention();
    await updateLoginAuditRetention(180);
    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/login-audits', {
      body: JSON.stringify({ ids: [1, 2] }),
      method: 'DELETE',
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/login-audits/cleanup', { method: 'POST' });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/api/login-audits/retention');
    expect(requestMock).toHaveBeenNthCalledWith(4, '/api/login-audits/retention', {
      body: JSON.stringify({ days: 180 }),
      method: 'PATCH',
    });
  });
});
