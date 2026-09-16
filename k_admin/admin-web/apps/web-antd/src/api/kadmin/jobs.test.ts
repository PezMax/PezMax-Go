import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request } from './client';
import {
  createJob,
  deleteJob,
  getJobExecutions,
  getJobs,
  runJob,
  updateJobStatus,
} from './jobs';

vi.mock('./client', () => ({ request: vi.fn() }));

const requestMock = vi.mocked(request);

describe('scheduled job API', () => {
  beforeEach(() => requestMock.mockReset());

  it('serializes task and execution filters', async () => {
    requestMock.mockResolvedValue({
      items: [],
      page: 2,
      pageSize: 20,
      total: 0,
    });

    await getJobs({ keyword: 'cleanup', page: 2, status: 'paused' });
    await getJobExecutions({ jobId: 7, status: 'failed', trigger: 'manual' });

    expect(requestMock).toHaveBeenNthCalledWith(
      1,
      '/api/jobs?keyword=cleanup&page=2&status=paused',
    );
    expect(requestMock).toHaveBeenNthCalledWith(
      2,
      '/api/job-logs?jobId=7&status=failed&trigger=manual',
    );
  });

  it('uses mutation endpoints and payloads', async () => {
    requestMock.mockResolvedValue({});
    const payload = {
      cronExpression: '0 2 * * *',
      description: '',
      handler: 'log_cleanup' as const,
      name: 'cleanup',
      parameters: { retentionDays: 30, taskLogRetentionDays: 90 },
      status: 'enabled' as const,
    };

    await createJob(payload);
    await updateJobStatus(8, 'paused');
    await runJob(8);
    await deleteJob(8);

    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/jobs', {
      body: JSON.stringify(payload),
      method: 'POST',
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/jobs/8/status', {
      body: JSON.stringify({ status: 'paused' }),
      method: 'PATCH',
    });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/api/jobs/8/run', {
      method: 'POST',
    });
    expect(requestMock).toHaveBeenNthCalledWith(4, '/api/jobs/8', {
      method: 'DELETE',
    });
  });
});
