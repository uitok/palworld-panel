import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AxiosResponse } from 'axios';
import { apiClient } from './client';
import { basesApi } from './bases';

afterEach(() => vi.restoreAllMocks());

describe('bases api', () => {
  it('loads a read-only base detail response', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { ok: true, data: { base: { id: 'base-1', name: 'Main', x: 1, y: 2, z: 3 }, status: { state: 'ready' } } },
      status: 200,
    } as AxiosResponse);

    const result = await basesApi.getBase('base/1');

    expect(get).toHaveBeenCalledWith('/bases/base%2F1');
    expect(result.base).toMatchObject({ id: 'base-1', name: 'Main', x: 1, y: 2, z: 3 });
    expect(result.status.state).toBe('ready');
  });

  it('posts one confirmed base cleanup request', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { ok: true, data: { cleaned: true, saved: true, base: { id: 'base-1', name: 'Main' } } },
      status: 200,
    } as AxiosResponse);

    const result = await basesApi.cleanBase('base/1');

    expect(post).toHaveBeenCalledWith('/bases/base%2F1/clean');
    expect(result).toMatchObject({ cleaned: true, saved: true, base: { id: 'base-1', name: 'Main' } });
  });
});
