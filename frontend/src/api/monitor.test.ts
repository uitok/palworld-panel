import type { AxiosResponse } from 'axios';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { monitorApi } from './monitor';

describe('monitor API', () => {
  afterEach(() => vi.restoreAllMocks());

  it('maps and updates runtime debug logging status', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { ok: true, data: { enabled: true, path: '/var/lib/palpanel/logs/palpanel-debug.log', size: 128, max_bytes: 1024, max_files: 3 } },
      status: 200,
    } as AxiosResponse);
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({
      data: { ok: true, data: { enabled: false, path: '/var/lib/palpanel/logs/palpanel-debug.log', size: 256, max_bytes: 1024, max_files: 3 } },
      status: 200,
    } as AxiosResponse);

    await expect(monitorApi.debugStatus()).resolves.toMatchObject({ enabled: true, size: 128 });
    await expect(monitorApi.setDebug(false)).resolves.toMatchObject({ enabled: false, size: 256 });
    expect(get).toHaveBeenCalledWith('/system/debug');
    expect(put).toHaveBeenCalledWith('/system/debug', { enabled: false });
  });
});
