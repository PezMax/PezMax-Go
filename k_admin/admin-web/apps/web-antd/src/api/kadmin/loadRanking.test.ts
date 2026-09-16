import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request } from './client';
import {
  getLoadRankingStatus,
  getLoadRankings,
  updateLoadRankingStatus,
} from './loadRanking';

vi.mock('./client', () => ({ request: vi.fn() }));

const requestMock = vi.mocked(request);

describe('load ranking API', () => {
  beforeEach(() => requestMock.mockReset());

  it('loads the sampling status', async () => {
    requestMock.mockResolvedValue({ enabled: false });

    await getLoadRankingStatus();

    expect(requestMock).toHaveBeenCalledWith('/api/load-ranking/status');
  });

  it('updates the sampling switch', async () => {
    requestMock.mockResolvedValue({ enabled: true });

    await updateLoadRankingStatus(true);

    expect(requestMock).toHaveBeenCalledWith('/api/load-ranking/status', {
      body: JSON.stringify({ enabled: true }),
      method: 'PATCH',
    });
  });

  it('requests rankings with defaults when no query is given', async () => {
    requestMock.mockResolvedValue({ items: [] });

    await getLoadRankings();

    expect(requestMock).toHaveBeenCalledWith('/api/load-ranking/rankings');
  });

  it('encodes filters, range and sort into the query string', async () => {
    requestMock.mockResolvedValue({ items: [] });

    await getLoadRankings({
      startedAt: '2026-07-31T00:00:00Z',
      endedAt: '2026-07-31T01:00:00Z',
      route: '/api/users',
      method: 'GET',
      statusCode: 500,
      groupBy: 'route',
      dimension: 'qps',
      order: 'desc',
      page: 2,
      pageSize: 50,
    });

    expect(requestMock).toHaveBeenCalledWith(
      '/api/load-ranking/rankings?startedAt=2026-07-31T00%3A00%3A00Z&endedAt=2026-07-31T01%3A00%3A00Z&route=%2Fapi%2Fusers&method=GET&statusCode=500&groupBy=route&dimension=qps&order=desc&page=2&pageSize=50',
    );
  });

  it('omits empty filters', async () => {
    requestMock.mockResolvedValue({ items: [] });

    await getLoadRankings({ route: '' });

    expect(requestMock).toHaveBeenCalledWith('/api/load-ranking/rankings');
  });
});
