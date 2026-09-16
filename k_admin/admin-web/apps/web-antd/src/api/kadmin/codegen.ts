import { request } from './client';

export interface CodegenCandidate {
  comment: string;
  name: string;
}

export interface CodegenColumn {
  creatable: boolean;
  control: string;
  editable: boolean;
  goType: string;
  isPK: boolean;
  label: string;
  listed: boolean;
  name: string;
  queryable: boolean;
  required: boolean;
  tsType: string;
}

export interface CodegenTableConfig {
  businessName: string;
  className: string;
  columns: CodegenColumn[];
  createdAt: string;
  generated: boolean;
  id: number;
  moduleName: string;
  routePrefix: string;
  tableName: string;
  updatedAt: string;
}

export interface CodegenImportPayload {
  businessName?: string;
  className?: string;
  moduleName?: string;
  routePrefix?: string;
  tableName: string;
}

export interface CodegenUpdatePayload {
  businessName: string;
  className: string;
  columns: CodegenColumn[];
  moduleName: string;
  routePrefix: string;
}

export interface CodegenArtifact {
  content: string;
  path: string;
}

export interface CodegenPreviewResult {
  artifacts: CodegenArtifact[];
}

export interface CodegenGenerateResult {
  conflicts: string[];
  menuUri: string;
  note: string;
  overwritten: string[];
  permissionCount: number;
  written: string[];
}

function queryString(filters: { keyword?: string }) {
  const params = new URLSearchParams();
  if (filters.keyword) {
    params.set('keyword', filters.keyword);
  }
  const query = params.toString();
  return query ? `?${query}` : '';
}

export function getCodegenCandidates(filters: { keyword?: string } = {}) {
  return request<CodegenCandidate[]>(`/api/codegen/candidates${queryString(filters)}`);
}

export function getCodegenTables(filters: { keyword?: string } = {}) {
  return request<CodegenTableConfig[]>(`/api/codegen/tables${queryString(filters)}`);
}

export function importCodegenTable(payload: CodegenImportPayload) {
  return request<CodegenTableConfig>('/api/codegen/tables/import', {
    body: JSON.stringify(payload),
    method: 'POST',
  });
}

export function getCodegenTable(id: number) {
  return request<CodegenTableConfig>(`/api/codegen/configs/${id}`);
}

export function updateCodegenTable(id: number, payload: CodegenUpdatePayload) {
  return request<CodegenTableConfig>(`/api/codegen/configs/${id}`, {
    body: JSON.stringify(payload),
    method: 'PUT',
  });
}

export function deleteCodegenTable(id: number) {
  return request<boolean>(`/api/codegen/configs/${id}`, { method: 'DELETE' });
}

export function previewCodegenTable(id: number) {
  return request<CodegenPreviewResult>(`/api/codegen/configs/${id}/preview`, {
    method: 'POST',
  });
}

export function generateCodegenTable(
  id: number,
  payload: { confirmOverwrite: boolean },
) {
  return request<CodegenGenerateResult>(`/api/codegen/configs/${id}/generate`, {
    body: JSON.stringify(payload),
    method: 'POST',
  });
}
