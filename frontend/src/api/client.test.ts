import { describe, expect, it, vi } from 'vitest';
import { handleRequest, unwrapApiData } from './client';

describe('api client response handling', () => {
  it('unwraps backend { ok, data } envelopes', () => {
    const result = unwrapApiData({ data: { ok: true, data: [{ id: 'job_1' }] }, status: 200 }, []);
    expect(result).toEqual([{ id: 'job_1' }]);
  });

  it('converts null array payloads to an empty array', () => {
    const result = unwrapApiData({ data: { ok: true, data: null }, status: 200 }, [{ id: 'old' }]);
    expect(result).toEqual([]);
  });

  it('returns the fallback without warning for quiet expected failures', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    const result = await handleRequest(
      () => Promise.reject({ response: { status: 502 } }),
      { logs: '暂无日志' },
      { quiet: true },
    );

    expect(result).toEqual({ logs: '暂无日志' });
    expect(warnSpy).not.toHaveBeenCalled();
    warnSpy.mockRestore();
  });
});
