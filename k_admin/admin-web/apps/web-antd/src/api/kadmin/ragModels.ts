import { request } from './client';

export interface RagProviderConfig {
  model: string;
  baseUrl: string;
  apiKeySet: boolean;
  timeoutSeconds: number;
}

export interface RagModelConfig {
  enabled: boolean;
  chat: RagProviderConfig;
  embedding: RagProviderConfig;
  vision: RagProviderConfig;
  ocr: RagProviderConfig;
  rerank: RagProviderConfig;
  dimensions: number;
  encryptedStore: boolean;
}

export interface RagProviderInput extends Omit<RagProviderConfig, 'apiKeySet'> {
  apiKey: string;
  clearApiKey: boolean;
}

export interface RagModelConfigInput {
  enabled: boolean;
  chat: RagProviderInput;
  embedding: RagProviderInput;
  vision: RagProviderInput;
  ocr: RagProviderInput;
  rerank: RagProviderInput;
  dimensions: number;
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
