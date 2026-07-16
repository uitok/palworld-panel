import type { AxiosResponse } from 'axios';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { apiClient } from './client';
import { setupApi } from './setup';

describe('setup API', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('submits an existing Windows server path and maps the import result', async () => {
    const path = String.raw`D:\SteamLibrary\steamapps\common\PalServer`;
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: {
        ok: true,
        data: {
          path,
          manifest_path: String.raw`D:\SteamLibrary\steamapps\appmanifest_2394010.acf`,
          build_id: '24681012',
          config_exists: true,
          already_bound: false,
          original_input: path,
        },
      },
      status: 200,
    } as AxiosResponse);

    const result = await setupApi.importServerDirectory(path);

    expect(post).toHaveBeenCalledWith('/server/import', { path });
    expect(result).toMatchObject({ path, build_id: '24681012', config_exists: true });
  });
});
