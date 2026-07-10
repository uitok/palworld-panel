import { apiClient, handleRequest } from './client';
import type {
  AITranslationConfig,
  AITranslationConfigUpdate,
  AITranslationTestResult,
} from '../types';
import { AI_OPERATION_TIMEOUT_MS } from './requestTimeouts';

const emptyConfig: AITranslationConfig = {
  configured: false,
  base_url: '',
  model: '',
  api_key_present: false,
};

const mapConfig = (raw: unknown): AITranslationConfig => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    configured: Boolean(data.configured),
    base_url: String(data.base_url || ''),
    model: String(data.model || ''),
    api_key_present: Boolean(data.api_key_present),
  };
};

export const aiTranslationApi = {
  getConfig: () =>
    handleRequest<unknown, AITranslationConfig>(
      () => apiClient.get('/ai/translation/config'),
      emptyConfig,
      { map: mapConfig, quiet: true, fallbackOnError: false },
    ),

  updateConfig: (update: AITranslationConfigUpdate) =>
    handleRequest<unknown, AITranslationConfig>(
      () => apiClient.put('/ai/translation/config', update),
      emptyConfig,
      { map: mapConfig, quiet: true, fallbackOnError: false },
    ),

  testConfig: (update: AITranslationConfigUpdate) =>
    handleRequest<AITranslationTestResult>(
      () => apiClient.post('/ai/translation/test', update, { timeout: AI_OPERATION_TIMEOUT_MS }),
      { ok: false, base_url: '', model: '', message: '' },
      { quiet: true, fallbackOnError: false },
    ),
};
