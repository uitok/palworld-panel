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
  timeout_seconds: 90,
  proxy_configured: false,
  proxy_url: '',
  custom_header_names: [],
};

export const mapAITranslationConfig = (raw: unknown): AITranslationConfig => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    configured: Boolean(data.configured),
    base_url: String(data.base_url || ''),
    model: String(data.model || ''),
    api_key_present: Boolean(data.api_key_present),
    timeout_seconds: Math.max(1, Number(data.timeout_seconds) || 90),
    proxy_configured: Boolean(data.proxy_configured),
    proxy_url: String(data.proxy_url || ''),
    custom_header_names: Array.isArray(data.custom_header_names)
      ? data.custom_header_names.map((name) => String(name)).filter(Boolean)
      : [],
  };
};

export const aiTranslationApi = {
  getConfig: () =>
    handleRequest<unknown, AITranslationConfig>(
      () => apiClient.get('/ai/translation/config'),
      emptyConfig,
      { map: mapAITranslationConfig, quiet: true, fallbackOnError: false },
    ),

  updateConfig: (update: AITranslationConfigUpdate) =>
    handleRequest<unknown, AITranslationConfig>(
      () => apiClient.put('/ai/translation/config', update),
      emptyConfig,
      { map: mapAITranslationConfig, quiet: true, fallbackOnError: false },
    ),

  testConfig: (update: AITranslationConfigUpdate) =>
    handleRequest<AITranslationTestResult>(
      () => apiClient.post('/ai/translation/test', update, { timeout: AI_OPERATION_TIMEOUT_MS }),
      {
        ok: false,
        base_url: '',
        model: '',
        message: '',
        timeout_seconds: 90,
        proxy_configured: false,
        custom_header_names: [],
      },
      { quiet: true, fallbackOnError: false },
    ),
};
