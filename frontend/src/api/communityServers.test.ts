import type { AxiosResponse } from 'axios';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { communityServersApi, mapCommunityServerResult } from './communityServers';

describe('community servers api', () => {
  afterEach(() => vi.restoreAllMocks());

  it('normalizes community server data and sends filters', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { ok: true, data: {
        servers: [{ id: 123, name: '中文房间', address: '127.0.0.1', port: '8211', players: '8', max_players: 32, country: 'cn', status: 'online' }],
        total: '1', page: 1, page_size: 30, source: 'battlemetrics', stale: true, cache_age_seconds: 90,
      } },
      status: 200,
    } as AxiosResponse);

    await expect(communityServersApi.list({ region: 'cn', search: '中文', password: false })).resolves.toMatchObject({
      servers: [{ id: '123', connect: '127.0.0.1:8211', players: 8, country: 'CN' }],
      stale: true,
    });
    expect(get).toHaveBeenCalledWith('/community-servers?region=cn&search=%E4%B8%AD%E6%96%87&password=false');
  });

  it('uses safe defaults for malformed items', () => {
    expect(mapCommunityServerResult({ servers: [null], total: 'bad' })).toMatchObject({
      total: 1,
      servers: [{ name: '未命名服务器', players: 0, password: false }],
    });
  });
});

