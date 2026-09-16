import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request } from './client';
import { getSystemMonitor, updateSystemMonitorStatus } from './systemMonitor';

vi.mock('./client', () => ({ request: vi.fn() }));

const requestMock = vi.mocked(request);

describe('system monitor API', () => {
  beforeEach(() => requestMock.mockReset());

  it('loads the current monitor status', async () => {
    requestMock.mockResolvedValue({ enabled: false });

    await getSystemMonitor();

    expect(requestMock).toHaveBeenCalledWith('/api/system-monitor');
  });

  it('updates the monitor switch', async () => {
    requestMock.mockResolvedValue({ enabled: true });

    await updateSystemMonitorStatus(true);

    expect(requestMock).toHaveBeenCalledWith('/api/system-monitor/status', {
      body: JSON.stringify({ enabled: true }),
      method: 'PATCH',
    });
  });
});
