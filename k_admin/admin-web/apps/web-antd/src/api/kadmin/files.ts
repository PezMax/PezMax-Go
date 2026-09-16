import { request, requestBlob } from './client';

export type FilePurpose =
  | 'attachment'
  | 'avatar'
  | 'editor-image'
  | 'import-temp';

export interface UploadedFile {
  id: number;
  name: string;
  url: string;
  size: number;
  contentType: string;
  extension: string;
  storage: 'local' | 'minio';
  purpose: FilePurpose;
  visibility: 'private' | 'public';
  status: 'ready';
  createdBy: number;
  createdAt: string;
  expiresAt?: string;
}

export function uploadFile(file: File, purpose: FilePurpose) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('purpose', purpose);
  return request<UploadedFile>('/api/files', {
    body: formData,
    method: 'POST',
  });
}

export function getFile(id: number) {
  return request<UploadedFile>(`/api/files/${id}`);
}

export function getFileContent(id: number) {
  return requestBlob(`/api/files/${id}/content`);
}

export function deleteFile(id: number) {
  return request<boolean>(`/api/files/${id}`, {
    method: 'DELETE',
  });
}

export function managedFileId(url: string) {
  const match = url.match(/\/api\/files\/(\d+)\/content(?:[?#]|$)/);
  if (!match?.[1]) {
    return undefined;
  }
  const id = Number(match[1]);
  return Number.isSafeInteger(id) && id > 0 ? id : undefined;
}

export function isManagedFileUrl(url?: string) {
  return !!url && managedFileId(url) !== undefined;
}
