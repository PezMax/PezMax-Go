import { beforeEach, describe, expect, it, vi } from 'vitest';

import { request, requestBlob } from './client';
import {
  deleteFile,
  getFile,
  getFileContent,
  isManagedFileUrl,
  managedFileId,
  uploadFile,
} from './files';

vi.mock('./client', () => ({
  request: vi.fn(),
  requestBlob: vi.fn(),
}));

const requestMock = vi.mocked(request);
const requestBlobMock = vi.mocked(requestBlob);

describe('file API', () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestBlobMock.mockReset();
  });

  it('uploads a file with its server-side purpose', async () => {
    const file = new File(['avatar'], 'avatar.png', { type: 'image/png' });
    requestMock.mockResolvedValueOnce({ id: 7 });

    await uploadFile(file, 'avatar');

    expect(requestMock).toHaveBeenCalledTimes(1);
    const [path, init] = requestMock.mock.calls[0] ?? [];
    expect(path).toBe('/api/files');
    expect(init?.method).toBe('POST');
    expect(init?.body).toBeInstanceOf(FormData);
    const formData = init?.body as FormData;
    expect(formData.get('file')).toBe(file);
    expect(formData.get('purpose')).toBe('avatar');
  });

  it('uses metadata, content and delete endpoints', async () => {
    requestMock.mockResolvedValue({});
    requestBlobMock.mockResolvedValue(new Blob());

    await getFile(7);
    await getFileContent(7);
    await deleteFile(7);

    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/files/7');
    expect(requestBlobMock).toHaveBeenCalledWith('/api/files/7/content');
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/files/7', {
      method: 'DELETE',
    });
  });

  it('recognizes only stable managed file content URLs', () => {
    expect(managedFileId('/api/files/42/content')).toBe(42);
    expect(
      managedFileId('https://example.test/api/files/42/content?download=1'),
    ).toBe(42);
    expect(isManagedFileUrl('/api/uploads/avatars/legacy.png')).toBe(false);
    expect(managedFileId('/api/files/0/content')).toBeUndefined();
  });
});
