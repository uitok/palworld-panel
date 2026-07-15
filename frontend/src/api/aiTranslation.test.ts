import type { AxiosResponse } from 'axios';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { aiTranslationApi, mapAITranslationConfig } from './aiTranslation';
import { apiClient } from './client';

describe('AI translation configuration', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('maps public transport metadata without requiring secret values', () => {
    expect(mapAITranslationConfig({
      configured: true,
      base_url: 'https://ai.example/v1',
      model: 'translator',
      api_key_present: true,
      timeout_seconds: 120,
      proxy_configured: true,
      proxy_url: 'socks5://127.0.0.1:10808',
      custom_header_names: ['X-Tenant-ID'],
    })).toEqual({
      configured: true,
      base_url: 'https://ai.example/v1',
      model: 'translator',
      api_key_present: true,
      timeout_seconds: 120,
      proxy_configured: true,
      proxy_url: 'socks5://127.0.0.1:10808',
      custom_header_names: ['X-Tenant-ID'],
    });
  });

  it('sends proxy and custom header changes only in the protected update request', async () => {
    const update = {
      timeout_seconds: 60,
      proxy_url: 'socks5://proxy-user:proxy-password@127.0.0.1:10808',
      custom_headers: { 'X-Tenant-ID': 'tenant-secret' },
    };
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({
      data: {
        ok: true,
        data: {
          configured: false,
          timeout_seconds: 60,
          proxy_configured: true,
          proxy_url: 'socks5://127.0.0.1:10808',
          custom_header_names: ['X-Tenant-ID'],
        },
      },
      status: 200,
    } as AxiosResponse);

    const result = await aiTranslationApi.updateConfig(update);

    expect(put).toHaveBeenCalledWith('/ai/translation/config', update);
    expect(result.proxy_url).toBe('socks5://127.0.0.1:10808');
    expect(JSON.stringify(result)).not.toContain('proxy-password');
    expect(JSON.stringify(result)).not.toContain('tenant-secret');
  });
});
