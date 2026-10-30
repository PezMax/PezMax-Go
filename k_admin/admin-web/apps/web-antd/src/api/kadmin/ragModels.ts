import { request } from './client';

export type RagModelKind = 'chat' | 'embedding' | 'ocr' | 'rerank' | 'vision';

export interface RagModelItem {
  id: string;
  kind: RagModelKind;
  name: string;
  model: string;
  baseUrl: string;
  apiKeySet: boolean;
  timeoutSeconds: number;
  active: boolean;
}

export interface RagModelConfig {
  enabled: boolean;
  dimensions: number;
  models: RagModelItem[];
  encryptedStore: boolean;
}

export interface RagModelInput {
  id: string;
  kind: RagModelKind;
  name: string;
  model: string;
  baseUrl: string;
  apiKey: string;
  clearApiKey: boolean;
  timeoutSeconds: number;
  active: boolean;
}

export interface RagModelConfigInput {
  enabled: boolean;
  dimensions: number;
  models: RagModelInput[];
}

export function getRagModelConfig() {
  return request<RagModelConfig>('/api/rag/models');
}

export function updateRagModelConfig(config: RagModelConfigInput) {
  return request<RagModelConfig>('/api/rag/models', {
    method: 'PUT',
    body: JSON.stringify(config),
  });
}
