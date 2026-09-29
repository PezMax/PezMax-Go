/**
 * 文件审核 API（已实装，后端实现见 internal/kadmin/exam_file_audit.go）。
 *
 * 接口清单（kadmin 管理端 JWT + RequirePermission）：
 * - GET  /api/exam-file-audit/pending    待审核队列（file_status=0），支持 fileName/user/fileType 过滤
 * - GET  /api/exam-file-audit/reported   被举报复审队列（file_status=3 联查最新待处理举报单）
 * - POST /api/exam-file-audit/approve       { fileIds }  多选批量通过（0→1）
 * - POST /api/exam-file-audit/approve-user  { userId }   单用户全部待审核一键通过
 * - POST /api/exam-file-audit/reject        { fileId, reason } 待审核拒绝（0→2，原因随下架通知返回用户）
 * - POST /api/exam-file-audit/audit         { fileId, decision, reason } 被举报复审
 *        （approve=举报不属实恢复上架、举报单置不属实；reject=举报属实下架、举报单置属实并下发下架通知）
 *
 * 权限：页面 system:file:audit:view（GET），审核操作 system:file:audit（POST），
 * 已在 bootstrap/permissions.go 注册；后台菜单“文件审核”随启动自动补种。
 *
 * 预览：fileUrl 由后端转为可读 HTTP 直链（datumReadableFileURL），前端 pdfjs
 * 按需逐页渲染（翻到哪页加载哪页）；pageCount 由 pdfjs 解析，后端无需返回。
 */
import type { File as ExamFile } from './generated/file';

import { request } from './client';

export type { ExamFile };

export interface ReportedExamFile extends ExamFile {
  reportId: number;
  reportReason: string;
  reportResult: string;
  reporterName: string;
  reportTime: string;
}

export interface PendingFileFilters {
  page: number;
  pageSize: number;
  /** 文件名关键词 */
  fileName?: string;
  /** 上传用户：支持用户 ID 或用户名 */
  user?: string;
  fileType?: number;
}

export interface ReportedFileFilters {
  page: number;
  pageSize: number;
  /** 文件名或举报原因关键词 */
  fileName?: string;
}

export interface FileAuditPageResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

function queryString(filters: object) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      params.set(key, String(value));
    }
  });
  const query = params.toString();
  return query ? `?${query}` : '';
}

/** 待审核队列（fileStatus = 0）。kind=ebook 时走电子书待审队列。 */
export function fetchPendingFiles(
  filters: PendingFileFilters & { kind?: 'exam' | 'ebook' },
): Promise<FileAuditPageResult<ExamFile>> {
  return request<FileAuditPageResult<ExamFile>>(
    `/api/exam-file-audit/pending${queryString(filters)}`,
  );
}

/** 被举报复审队列（fileStatus = 3，联查最新待处理举报单）。kind=ebook 走电子书队列。 */
export function fetchReportedFiles(
  filters: ReportedFileFilters & { kind?: 'exam' | 'ebook' },
): Promise<FileAuditPageResult<ReportedExamFile>> {
  return request<FileAuditPageResult<ReportedExamFile>>(
    `/api/exam-file-audit/reported${queryString(filters)}`,
  );
}

/** 多选一键通过。kind=ebook 时通过电子书待审。 */
export function approveFiles(
  fileIds: number[],
  kind: 'exam' | 'ebook' = 'exam',
): Promise<{ approved: number }> {
  return request<{ approved: number }>(`/api/exam-file-audit/approve`, {
    body: JSON.stringify({ fileIds, kind }),
    method: 'POST',
  });
}

/** 单个用户的全部待审核文件一键通过。kind=ebook 时通过电子书待审。 */
export function approveUserFiles(
  userId: number,
  kind: 'exam' | 'ebook' = 'exam',
): Promise<{ approved: number }> {
  return request<{ approved: number }>(`/api/exam-file-audit/approve-user`, {
    body: JSON.stringify({ userId, kind }),
    method: 'POST',
  });
}

/** 查询某用户当前待审核文件数量（用于二次确认文案）。 */
export async function countPendingByUser(
  userId: number,
  kind: 'exam' | 'ebook' = 'exam',
): Promise<number> {
  const result = await request<FileAuditPageResult<ExamFile>>(
    `/api/exam-file-audit/pending${queryString({ page: 1, pageSize: 1, userId, kind })}`,
  );
  return result.total;
}

/** 待审核文件单条拒绝（fileStatus 0→2），原因必填并随通知返回给用户。 */
export function rejectPendingFile(input: {
  fileId: number;
  reason: string;
  kind?: 'exam' | 'ebook';
}): Promise<void> {
  return request<unknown>(`/api/exam-file-audit/reject`, {
    body: JSON.stringify(input),
    method: 'POST',
  }).then(() => undefined);
}

/** 被举报文件复审：通过（举报不属实，恢复上架）/ 拒绝（举报属实，下架）。 */
export function auditReportedFile(input: {
  fileId: number;
  decision: 'approve' | 'reject';
  reason: string;
  kind?: 'exam' | 'ebook';
}): Promise<void> {
  return request<unknown>(`/api/exam-file-audit/audit`, {
    body: JSON.stringify(input),
    method: 'POST',
  }).then(() => undefined);
}

export type PreviewKind = 'image' | 'pdf' | 'unsupported';

/** 文件预览方式：PDF 分页预览 / 图片预览 / 暂不支持（提示下载）。 */
export function previewKindOf(format: string): PreviewKind {
  const ext = format.toLowerCase();
  if (['jpg', 'jpeg', 'png', 'webp'].includes(ext)) {
    return 'image';
  }
  if (ext === 'pdf') {
    return 'pdf';
  }
  return 'unsupported';
}

/**
 * 解析文件预览地址：真实数据直接使用后端返回的可读 fileUrl
 * （datumReadableFileURL 已把 minio:// 转为 HTTP 直链），由 pdfjs 按需渲染。
 */
export function resolvePreview(
  file: ExamFile,
): Promise<{ url: string; pageCount?: number }> {
  return Promise.resolve({ url: file.fileUrl });
}
