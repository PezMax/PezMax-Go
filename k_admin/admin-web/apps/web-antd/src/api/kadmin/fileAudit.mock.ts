/**
 * 文件审核 mock 数据源（仅预览阶段使用）。
 *
 * 通过 fileAudit.ts 的 USE_MOCK 开关接入；同步 kadmin 后台时整个文件可删除。
 * mock 的字段结构与 ptmj_file / ptmj_report 保持一致，便于真实接口无缝替换。
 */
import type { File as ExamFile } from './generated/file';

/** 被举报文件（文件信息 + 关联举报单）。 */
export interface ReportedExamFile extends ExamFile {
  reportId: number;
  reportReason: string;
  reportResult: string;
  reporterName: string;
  reportTime: string;
}

const NOW = new Date('2026-09-27 10:00:00').getTime();

function daysAgo(days: number, hour = 9, minute = 30) {
  const time = new Date(NOW - days * 24 * 3600 * 1000);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${time.getFullYear()}-${pad(time.getMonth() + 1)}-${pad(
    time.getDate(),
  )} ${pad(hour)}:${pad(minute)}:00`;
}

interface MockUser {
  userId: number;
  name: string;
  school: string;
}

const MOCK_USERS: MockUser[] = [
  { userId: 1001, name: '陈晓东', school: '市第一中学' },
  { userId: 1002, name: '林小雨', school: '实验中学' },
  { userId: 1003, name: '王志强', school: '外国语学校' },
  { userId: 1004, name: '赵一敏', school: '育才中学' },
  { userId: 1005, name: '刘思洋', school: '市第一中学' },
  { userId: 1006, name: '孙嘉怡', school: '实验中学' },
  { userId: 1007, name: '周子涛', school: '外国语学校' },
  { userId: 1008, name: '吴可悦', school: '育才中学' },
];

interface MockFileSeed {
  user: MockUser;
  fileName: string;
  fileFormat: string;
  fileType: number;
  fileSubject: string;
  fileYear: number;
  fileSizeMb: number;
  days: number;
}

const PENDING_SEEDS: MockFileSeed[] = [
  { user: MOCK_USERS[0]!, fileName: '2024-2025学年度第一学期期末考试-高一数学.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '数学', fileYear: 2025, fileSizeMb: 2.4, days: 1 },
  { user: MOCK_USERS[0]!, fileName: '2024-2025学年度第一学期期末考试-高一数学参考答案.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '数学', fileYear: 2025, fileSizeMb: 1.1, days: 1 },
  { user: MOCK_USERS[0]!, fileName: '高二物理期中复习资料（含答案）.pdf', fileFormat: 'pdf', fileType: 2, fileSubject: '物理', fileYear: 2025, fileSizeMb: 3.6, days: 2 },
  { user: MOCK_USERS[0]!, fileName: '英语完形填空专项训练（一）.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '英语', fileYear: 2025, fileSizeMb: 0.9, days: 3 },
  { user: MOCK_USERS[0]!, fileName: '英语完形填空专项训练（二）.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '英语', fileYear: 2025, fileSizeMb: 0.9, days: 3 },
  { user: MOCK_USERS[1]!, fileName: '2025年中考化学模拟卷（三）.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '化学', fileYear: 2025, fileSizeMb: 1.8, days: 1 },
  { user: MOCK_USERS[1]!, fileName: '语文古诗文默写整理.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '语文', fileYear: 2025, fileSizeMb: 0.6, days: 2 },
  { user: MOCK_USERS[1]!, fileName: '某中学月考数学试卷（转档版）.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '数学', fileYear: 2025, fileSizeMb: 0.4, days: 4 },
  { user: MOCK_USERS[2]!, fileName: '补考复习提纲-高二生物.pdf', fileFormat: 'pdf', fileType: 4, fileSubject: '生物', fileYear: 2024, fileSizeMb: 1.2, days: 5 },
  { user: MOCK_USERS[3]!, fileName: '高三物理一轮复习讲义.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '物理', fileYear: 2026, fileSizeMb: 5.2, days: 1 },
  { user: MOCK_USERS[3]!, fileName: '高三物理一轮复习配套习题.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '物理', fileYear: 2026, fileSizeMb: 2.8, days: 1 },
  { user: MOCK_USERS[3]!, fileName: '2026届高三上学期期末化学试卷.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '化学', fileYear: 2026, fileSizeMb: 2.1, days: 2 },
  { user: MOCK_USERS[3]!, fileName: '实验中学高二英语期中卷.pdf', fileFormat: 'pdf', fileType: 5, fileSubject: '英语', fileYear: 2025, fileSizeMb: 1.5, days: 6 },
  { user: MOCK_USERS[4]!, fileName: '试卷扫描件-20241012.jpg', fileFormat: 'jpg', fileType: 1, fileSubject: '数学', fileYear: 2024, fileSizeMb: 1.9, days: 7 },
  { user: MOCK_USERS[4]!, fileName: '试卷扫描件-20241013.jpg', fileFormat: 'jpg', fileType: 1, fileSubject: '数学', fileYear: 2024, fileSizeMb: 2.2, days: 7 },
  { user: MOCK_USERS[5]!, fileName: '初中数学竞赛真题汇编.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '数学', fileYear: 2025, fileSizeMb: 4.4, days: 2 },
  { user: MOCK_USERS[5]!, fileName: '高中数学错题本模板.txt', fileFormat: 'txt', fileType: 3, fileSubject: '数学', fileYear: 2025, fileSizeMb: 0.02, days: 3 },
  { user: MOCK_USERS[6]!, fileName: '2025年秋季学期期中考试-高二语文.pdf', fileFormat: 'pdf', fileType: 2, fileSubject: '语文', fileYear: 2025, fileSizeMb: 1.7, days: 1 },
  { user: MOCK_USERS[6]!, fileName: '2025年秋季学期期中考试-高二语文答案.pdf', fileFormat: 'pdf', fileType: 2, fileSubject: '语文', fileYear: 2025, fileSizeMb: 0.8, days: 1 },
  { user: MOCK_USERS[6]!, fileName: '期中考试-高二语文答题卡.png', fileFormat: 'png', fileType: 2, fileSubject: '语文', fileYear: 2025, fileSizeMb: 1.3, days: 1 },
  { user: MOCK_USERS[7]!, fileName: '外校交流卷-高一英语（含听力）.pdf', fileFormat: 'pdf', fileType: 5, fileSubject: '英语', fileYear: 2025, fileSizeMb: 3.1, days: 4 },
  { user: MOCK_USERS[7]!, fileName: '力学专题复习-常见模型总结.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '物理', fileYear: 2026, fileSizeMb: 2.6, days: 2 },
];

const REPORTED_SEEDS: Array<MockFileSeed & { reason: string; reporter: string; reportDays: number; result: string }> = [
  { user: MOCK_USERS[0]!, fileName: '市重点联考数学卷（内部资料）.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '数学', fileYear: 2025, fileSizeMb: 2.0, days: 8, reason: '试卷内含培训机构广告二维码，疑似引流', reporter: '匿名用户', reportDays: 2, result: '0' },
  { user: MOCK_USERS[1]!, fileName: '英语阅读理解100篇（含解析）.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '英语', fileYear: 2025, fileSizeMb: 6.8, days: 9, reason: '答案页包含外部网盘链接，疑似诱导下载', reporter: '用户8827', reportDays: 1, result: '0' },
  { user: MOCK_USERS[2]!, fileName: '物理竞赛辅导讲义-电磁学.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '物理', fileYear: 2024, fileSizeMb: 4.2, days: 12, reason: '文件与标题不符，实为其他学校的付费讲义', reporter: '用户5190', reportDays: 3, result: '0' },
  { user: MOCK_USERS[4]!, fileName: '期末考试生物试卷及答案.pdf', fileFormat: 'pdf', fileType: 1, fileSubject: '生物', fileYear: 2025, fileSizeMb: 1.6, days: 6, reason: '涉嫌抄袭他人已上传的试卷，内容完全一致', reporter: '用户3312', reportDays: 1, result: '0' },
  { user: MOCK_USERS[6]!, fileName: '化学方程式配平技巧总结.pdf', fileFormat: 'pdf', fileType: 3, fileSubject: '化学', fileYear: 2026, fileSizeMb: 0.7, days: 3, reason: '内容含大量错误答案，误导学生复习', reporter: '匿名用户', reportDays: 2, result: '0' },
];

function buildExamFile(seed: MockFileSeed, fileId: number, status: number): ExamFile {
  const createTime = daysAgo(seed.days, 8 + (fileId % 9), (fileId * 13) % 60);
  return {
    fileId,
    userId: seed.user.userId,
    fileName: seed.fileName,
    fileUrl: `minio://ptmj-files/exam/${fileId}/${encodeURIComponent(seed.fileName)}`,
    fileSize: Math.round(seed.fileSizeMb * 1024 * 1024),
    fileFormat: seed.fileFormat,
    fileYear: seed.fileYear,
    fileType: seed.fileType,
    fileSchool: seed.user.school,
    fileSubject: seed.fileSubject,
    reviewer: '',
    fileStatus: status,
    delFlag: 0,
    createBy: seed.user.name,
    createTime,
    updateBy: '',
    updateTime: createTime,
    remark: '',
  };
}

const pendingFiles: ExamFile[] = PENDING_SEEDS.map((seed, index) =>
  buildExamFile(seed, 9001 + index, 0),
);

const reportedFiles: ReportedExamFile[] = REPORTED_SEEDS.map((seed, index) => {
  const file = buildExamFile(seed, 8801 + index, 3);
  return {
    ...file,
    reportId: 510 + index,
    reportReason: seed.reason,
    reportResult: seed.result,
    reporterName: seed.reporter,
    reportTime: daysAgo(seed.reportDays, 14 + index, 20),
  };
});

/** 已通过 / 已拒绝的历史计数（预览阶段为演示数据，操作后累加）。 */
export const sessionStats = { approved: 0, rejected: 0 };

function delay<T>(value: T, ms = 320): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), ms);
  });
}

export function listPendingFiles(params: {
  page: number;
  pageSize: number;
  fileName?: string;
  user?: string;
  fileType?: number;
}): Promise<{ items: ExamFile[]; total: number }> {
  const { page, pageSize, fileName, user, fileType } = params;
  const keyword = fileName?.trim();
  const userKeyword = user?.trim();
  const items = pendingFiles
    .filter((file) => (!keyword || file.fileName.includes(keyword)))
    .filter((file) =>
      !userKeyword ||
      file.createBy.includes(userKeyword) ||
      String(file.userId).includes(userKeyword),
    )
    .filter((file) => fileType === undefined || file.fileType === fileType)
    .sort((a, b) => b.createTime.localeCompare(a.createTime));
  const start = (page - 1) * pageSize;
  return delay({ items: items.slice(start, start + pageSize), total: items.length });
}

export function listReportedFiles(params: {
  page: number;
  pageSize: number;
  fileName?: string;
}): Promise<{ items: ReportedExamFile[]; total: number }> {
  const { page, pageSize, fileName } = params;
  const keyword = fileName?.trim();
  const items = reportedFiles.filter(
    (file) => !keyword || file.fileName.includes(keyword) || file.reportReason.includes(keyword!),
  );
  const start = (page - 1) * pageSize;
  return delay({ items: items.slice(start, start + pageSize), total: items.length });
}

export function approveFiles(fileIds: number[]): Promise<{ approved: number }> {
  const idSet = new Set(fileIds);
  for (let i = pendingFiles.length - 1; i >= 0; i -= 1) {
    const file = pendingFiles[i]!;
    if (idSet.has(file.fileId)) {
      file.fileStatus = 1;
      file.reviewer = 'admin';
      file.updateTime = daysAgo(0, 10, 0);
      pendingFiles.splice(i, 1);
      sessionStats.approved += 1;
    }
  }
  return delay({ approved: idSet.size });
}

export function approveUserFiles(userId: number): Promise<{ approved: number }> {
  let approved = 0;
  for (let i = pendingFiles.length - 1; i >= 0; i -= 1) {
    const file = pendingFiles[i]!;
    if (file.userId === userId) {
      file.fileStatus = 1;
      file.reviewer = 'admin';
      file.updateTime = daysAgo(0, 10, 0);
      pendingFiles.splice(i, 1);
      approved += 1;
      sessionStats.approved += 1;
    }
  }
  return delay({ approved });
}

export function countPendingByUser(userId: number): Promise<number> {
  return delay(pendingFiles.filter((file) => file.userId === userId).length, 120);
}

/** 待审核文件拒绝（fileStatus 0→2，原因随结果返回用户）。 */
export function rejectPendingFile(input: {
  fileId: number;
  reason: string;
}): Promise<Record<string, never>> {
  const file = pendingFiles.find((item) => item.fileId === input.fileId);
  if (file) {
    file.fileStatus = 2;
    file.reviewer = 'admin';
    file.updateTime = daysAgo(0, 10, 0);
    file.remark = `拒绝原因：${input.reason}`;
    pendingFiles.splice(pendingFiles.indexOf(file), 1);
    sessionStats.rejected += 1;
  }
  return delay({}, 360);
}

export function auditReportedFile(input: {
  fileId: number;
  decision: 'approve' | 'reject';
  reason: string;
}): Promise<Record<string, never>> {
  const index = reportedFiles.findIndex((file) => file.fileId === input.fileId);
  if (index >= 0) {
    const file = reportedFiles[index]!;
    file.reportResult = input.decision === 'approve' ? '2' : '1';
    file.reviewer = 'admin';
    file.updateTime = daysAgo(0, 10, 0);
    reportedFiles.splice(index, 1);
    if (input.decision === 'approve') {
      sessionStats.approved += 1;
    } else {
      sessionStats.rejected += 1;
    }
  }
  return delay({}, 420);
}

// ---------------------------------------------------------------------------
// mock 预览资源：离线生成样例 PDF / 图片，避免预览阶段依赖 MinIO
// ---------------------------------------------------------------------------

export type PreviewKind = 'image' | 'pdf' | 'unsupported';

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

function samplePageCount(fileId: number) {
  return 3 + (fileId % 8);
}

/** 手工构造一份合法的多页 PDF（ASCII 内容），供离线预览演示翻页。 */
function buildSamplePdf(pageCount: number): Blob {
  const parts: string[] = [];
  const offsets: number[] = [];
  let length = 0;
  const push = (text: string) => {
    parts.push(text);
    length += text.length;
  };
  const beginObject = (num: number) => {
    offsets[num] = length;
    push(`${num} 0 obj\n`);
  };

  push('%PDF-1.4\n');
  beginObject(1);
  push('<< /Type /Catalog /Pages 2 0 R >>\nendobj\n');
  const kids = Array.from({ length: pageCount }, (_, i) => `${3 + i * 2} 0 R`).join(' ');
  beginObject(2);
  push(`<< /Type /Pages /Kids [${kids}] /Count ${pageCount} >>\nendobj\n`);

  for (let i = 0; i < pageCount; i += 1) {
    const pageObject = 3 + i * 2;
    const contentObject = pageObject + 1;
    beginObject(pageObject);
    push(
      `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] ` +
        `/Resources << /Font << /F1 ${3 + pageCount * 2} 0 R >> >> ` +
        `/Contents ${contentObject} 0 R >>\nendobj\n`,
    );
    const stream = [
      '0.82 0.85 0.93 RG 1.5 w 36 36 523 770 re S',
      'BT /F1 24 Tf 118 744 Td (PezMax File Audit Preview) Tj ET',
      `BT /F1 54 Tf 196 412 Td (Page ${i + 1} / ${pageCount}) Tj ET`,
      'BT /F1 13 Tf 172 64 Td (sample document for offline preview) Tj ET',
    ].join('\n');
    beginObject(contentObject);
    push(`<< /Length ${stream.length} >>\nstream\n${stream}\nendstream\nendobj\n`);
  }

  const fontObject = 3 + pageCount * 2;
  const totalObjects = fontObject;
  beginObject(fontObject);
  push('<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n');

  const xrefStart = length;
  let xref = `xref\n0 ${totalObjects + 1}\n0000000000 65535 f \n`;
  for (let n = 1; n <= totalObjects; n += 1) {
    xref += `${String(offsets[n]).padStart(10, '0')} 00000 n \n`;
  }
  push(xref);
  push(`trailer\n<< /Size ${totalObjects + 1} /Root 1 0 R >>\nstartxref\n${xrefStart}\n%%EOF`);
  return new Blob(parts, { type: 'application/pdf' });
}

function buildSampleImage(seedText: string): string {
  const canvas = document.createElement('canvas');
  canvas.width = 760;
  canvas.height = 1000;
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    return '';
  }
  ctx.fillStyle = '#f5f6fa';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#dee3f0';
  ctx.fillRect(48, 48, canvas.width - 96, canvas.height - 96);
  ctx.fillStyle = '#4353ff';
  ctx.fillRect(48, 48, canvas.width - 96, 96);
  ctx.fillStyle = '#ffffff';
  ctx.font = '600 30px sans-serif';
  ctx.fillText('PezMax 文件审核预览', 84, 110);
  ctx.fillStyle = '#3a3f51';
  ctx.font = '24px sans-serif';
  ctx.fillText(seedText.slice(0, 26), 84, 240);
  ctx.fillStyle = '#8a90a5';
  ctx.font = '18px sans-serif';
  ctx.fillText('样例图片 · 第 1 页 / 共 1 页', 84, 300);
  for (let i = 0; i < 8; i += 1) {
    ctx.fillStyle = i % 2 === 0 ? '#e6e9f5' : '#eef1f8';
    ctx.fillRect(84, 380 + i * 58, canvas.width - 168, 34);
  }
  return canvas.toDataURL('image/png');
}

const previewUrlCache = new Map<number, string>();

/** 解析文件的可预览 URL（mock：离线生成；真实接口：MinIO 公网地址）。 */
export async function resolvePreview(
  file: ExamFile,
): Promise<{ url: string; pageCount?: number }> {
  const kind = previewKindOf(file.fileFormat);
  if (kind === 'unsupported') {
    return { url: '' };
  }
  const cached = previewUrlCache.get(file.fileId);
  if (cached) {
    return { url: cached, pageCount: kind === 'pdf' ? samplePageCount(file.fileId) : 1 };
  }
  const url =
    kind === 'pdf'
      ? URL.createObjectURL(buildSamplePdf(samplePageCount(file.fileId)))
      : buildSampleImage(file.fileName);
  previewUrlCache.set(file.fileId, url);
  await delay(url, 200);
  return { url, pageCount: kind === 'pdf' ? samplePageCount(file.fileId) : 1 };
}
