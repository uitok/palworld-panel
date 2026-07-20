import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AxiosResponse } from 'axios';
import { apiClient } from './client';
import { backupsApi } from './backups';

describe('backups api', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('disables the global request timeout for backup downloads', async () => {
    const blob = new Blob(['backup']);
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { ok: true, data: blob },
      status: 200,
    } as AxiosResponse);

    await expect(backupsApi.download('palpanel manual.zip')).resolves.toBe(blob);
    expect(get).toHaveBeenCalledWith('/backups/palpanel%20manual.zip/download', {
      responseType: 'blob',
      timeout: 0,
    });
  });
});
