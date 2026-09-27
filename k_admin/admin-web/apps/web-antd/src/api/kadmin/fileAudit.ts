/**
 * 文件审核 API。
 *
 * ★ 预览阶段：USE_MOCK = true，数据来自 fileAudit.mock.ts，不依赖后端。
 *
 * 同步 kadmin 后台时的改造清单：
 * 1. 将下方 USE_MOCK 置为 false，并删除 fileAudit.mock.ts；
 * 2. 后端补齐以下接口（internal/kadmin，参考 datum_report.go 的审核实现）：
 *    - GET  /api/exam-files                    现有列表接口需追加 fileStatus、userId、fileType 过滤参数
 *    - GET  /api/exam-files/reported           被举报文件联查（ptmj_file + ptmj_report），分页
 *    - POST /api/exam-files/audit-approve      { fileIds }   多选批量通过（fileStatus 0→1，记录 reviewer）
 *    - POST /api/exam-files/audit-approve-user { userId }    单用户全部待审核一键通过
 *    - POST /api/exam-files/:id/audit          { decision: 'approve'|'reject', reason }
 *                                              被举报文件复审：approve→fileStatus 1（恢复上架、举报置不属实），
 *                                              reject→fileStatus 2（下架、举报置属实并下发下架通知），reason 必填并落库
 * 3. 文件列表返回可读的预览地址（参考 datumReadableFileURL）与可选的 pageCount 字段；
 * 4. 后台菜单新增“文件审核”，组件路径 /kadmin/components/FileAuditView，
 *    并在 bootstrap/permissions.go 注册按钮权限 system:file:audit。
 */
import type { File as ExamFile } from './generated/file';

import {
  approveFiles as mockApproveFiles,
  approveUserFiles as mockApproveUserFiles,
  auditReportedFile as mockAuditReportedFile,
  countPendingByUser as mockCountPendingByUser,
  listPendingFiles as mockListPendingFiles,
  listReportedFiles as mockListReportedFiles,
  previewKindOf as mockPreviewKindOf,
  rejectPendingFile as mockRejectPendingFile,
  resolvePreview as mockResolvePreview,
  type ReportedExamFile,
} from './fileAudit.mock';

const USE_MOCK = true;

export type { ExamFile, ReportedExamFile };

export interface PendingFileFilters {
  page: number;
  pageSize: number;
  /** 文件名关键词 */
  fileName?: string;
  /** 上传用户：支持用户 ID 或昵称 */
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
}

/** 待审核队列（fileStatus = 0）。 */
export function fetchPendingFiles(
  filters: PendingFileFilters,
): Promise<FileAuditPageResult<ExamFile>> {
  if (USE_MOCK) {
    return mockListPendingFiles(filters);
  }
  // 同步后启用：
  // return request<PageResult<File>>('/api/exam-files', { ... });
  throw new Error('真实接口尚未接入：请先完成后端同步（见 fileAudit.ts 头部注释）');
}

/** 被举报复审队列（fileStatus = 3，联查举报单）。 */
export function fetchReportedFiles(
  filters: ReportedFileFilters,
): Promise<FileAuditPageResult<ReportedExamFile>> {
  if (USE_MOCK) {
    return mockListReportedFiles(filters);
  }
  throw new Error('真实接口尚未接入：请先完成后端同步（见 fileAudit.ts 头部注释）');
}

/** 多选一键通过。 */
export function approveFiles(fileIds: number[]): Promise<{ approved: number }> {
  if (USE_MOCK) {
    return mockApproveFiles(fileIds);
  }
  throw new Error('真实接口尚未接入：请先完成后端同步');
}

/** 单个用户的全部待审核文件一键通过。 */
export function approveUserFiles(userId: number): Promise<{ approved: number }> {
  if (USE_MOCK) {
    return mockApproveUserFiles(userId);
  }
  throw new Error('真实接口尚未接入：请先完成后端同步');
}

/** 查询某用户当前待审核文件数量（用于二次确认文案）。 */
export function countPendingByUser(userId: number): Promise<number> {
  if (USE_MOCK) {
    return mockCountPendingByUser(userId);
  }
  throw new Error('真实接口尚未接入：请先完成后端同步');
}

/** 被举报文件复审：通过（举报不属实，恢复上架）/ 拒绝（举报属实，下架）。 */
export function auditReportedFile(input: {
  fileId: number;
  decision: 'approve' | 'reject';
  reason: string;
}): Promise<void> {
  if (USE_MOCK) {
    return mockAuditReportedFile(input).then(() => undefined);
  }
  throw new Error('真实接口尚未接入：请先完成后端同步');
}

/** 待审核文件单条拒绝（fileStatus 0→2），原因必填并返回给用户。 */
export function rejectPendingFile(input: {
  fileId: number;
  reason: string;
}): Promise<void> {
  if (USE_MOCK) {
    return mockRejectPendingFile(input).then(() => undefined);
  }
  throw new Error('真实接口尚未接入：请先完成后端同步');
}

export type PreviewKind = 'image' | 'pdf' | 'unsupported';

/** 文件预览方式：PDF 分页预览 / 图片预览 / 暂不支持（提示下载）。 */
export function previewKindOf(format: string): PreviewKind {
  return mockPreviewKindOf(format);
}

/** 解析文件预览地址与页数（同步后由后端返回可读 URL 与 pageCount）。 */
export function resolvePreview(
  file: ExamFile,
): Promise<{ url: string; pageCount?: number }> {
  if (USE_MOCK) {
    return mockResolvePreview(file);
  }
  // 同步后启用：url 取列表返回的可读 fileUrl，pageCount 由后端解析 PDF 页数。
  throw new Error('真实接口尚未接入：请先完成后端同步');
}
