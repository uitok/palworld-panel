import { describe, expect, it, vi } from 'vitest';
import { ApiError, handleRequest, unwrapApiData } from './client';

describe('api client response handling', () => {
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
});
