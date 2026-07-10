import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiClient, ApiError, currentApiBaseUrl, defaultBackendUrl, handleRequest, readBackendUrl, unwrapApiData, writeBackendUrl } from './client';
import { storageKeys } from '../config/defaults';

describe('api client response handling', () => {
  beforeEach(() => {
    localStorage.clear();
    window.history.replaceState({}, '', 'http://localhost:3000/');
  });

  it('unwraps backend { ok, data } envelopes', () => {
    const result = unwrapApiData({ data: { ok: true, data: [{ id: 'job_1' }] }, status: 200 }, []);
    expect(result).toEqual([{ id: 'job_1' }]);
  });

  it('converts null array payloads to an empty array', () => {
    const result = unwrapApiData({ data: { ok: true, data: null }, status: 200 }, [{ id: 'old' }]);
    expect(result).toEqual([]);
  });

  it('throws by default when the backend request fails', async () => {
    await expect(
      handleRequest(() => Promise.reject({ response: { status: 502 } }), { logs: '暂无日志' }, { quiet: true }),
    ).rejects.toBeInstanceOf(ApiError);
  });

  it('returns the fallback only when fallbackOnError is explicitly enabled', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    const result = await handleRequest(
      () => Promise.reject({ response: { status: 502 } }),
      { logs: '暂无日志' },
      { quiet: true, fallbackOnError: true },
    );

    expect(result).toEqual({ logs: '暂无日志' });
    expect(warnSpy).not.toHaveBeenCalled();
    warnSpy.mockRestore();
  });

  it('throws backend error envelopes when fallback is disabled', async () => {
    await expect(
      handleRequest(() => Promise.resolve({ data: { ok: false, error: { code: 'unsupported', message: '接口未实现' } }, status: 501 }), {}),
    ).rejects.toMatchObject({ message: '接口未实现', code: 'unsupported' });
  });

  it('builds API base URL from the configured backend URL', () => {
    expect(defaultBackendUrl()).toBe('http://127.0.0.1:64217');
    expect(currentApiBaseUrl()).toBe('http://127.0.0.1:64217/api');
    expect(defaultBackendUrl(false)).toBe('');
    expect(currentApiBaseUrl(false)).toBe('/api');

    writeBackendUrl('127.0.0.1:65000');
    expect(currentApiBaseUrl()).toBe('http://127.0.0.1:65000/api');
    expect(localStorage.getItem(storageKeys.backendUrl)).toBe('127.0.0.1:65000');

    writeBackendUrl('http://127.0.0.1:65001/api/');
    expect(currentApiBaseUrl()).toBe('http://127.0.0.1:65001/api');
  });

  it('adapts stored loopback backend URLs when the page is opened from a LAN host', () => {
    const locationSpy = vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      hostname: '192.168.200.4',
    } as Location);
    writeBackendUrl('http://127.0.0.1:65000');

    expect(currentApiBaseUrl()).toBe('http://192.168.200.4:65000/api');
    locationSpy.mockRestore();
  });

  it('prefers an explicitly configured same-origin proxy over a stored absolute URL', () => {
    writeBackendUrl('http://175.30.217.48:64217');

    expect(readBackendUrl(true, '/api')).toBe('/api');
    expect(currentApiBaseUrl(true, '/api')).toBe('/api');
  });

  it('migrates legacy backend URL storage once when read', () => {
    localStorage.setItem('palsphere_backend_url', 'http://127.0.0.1:65002');

    expect(currentApiBaseUrl()).toBe('http://127.0.0.1:65002/api');
    expect(localStorage.getItem(storageKeys.backendUrl)).toBe('http://127.0.0.1:65002');
    expect(localStorage.getItem('palsphere_backend_url')).toBeNull();
  });

  it('retries development network failures through the same-origin API proxy', async () => {
    writeBackendUrl('http://127.0.0.1:64217');
    const bases: string[] = [];

    const response = await apiClient.get('/health', {
      adapter: async (config) => {
        bases.push(String(config.baseURL));
        if (bases.length === 1) {
          return Promise.reject(Object.assign(new Error('Network Error'), {
            isAxiosError: true,
            code: 'ERR_NETWORK',
            config,
            toJSON: () => ({}),
          }));
        }
        return {
          data: { ok: true, data: { status: 'ok' } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        };
      },
    });

    expect(response.data).toEqual({ ok: true, data: { status: 'ok' } });
    expect(bases).toEqual(['http://127.0.0.1:64217/api', '/api']);
  });
});
