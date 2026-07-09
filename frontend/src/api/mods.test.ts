import { describe, expect, it, vi } from 'vitest';
import type { AxiosResponse } from 'axios';
import { apiClient } from './client';
import { mapWorkshopItem, mapWorkshopStatus, modsApi } from './mods';

describe('mods api mapping', () => {
  it('maps Workshop metadata with safe fallbacks', () => {
    expect(mapWorkshopItem(null)).toMatchObject({
      id: '',
      title: 'Untitled Workshop Item',
      tags: [],
      installed: false,
      enabled: false,
      update_available: false,
    });

    const item = mapWorkshopItem({
      id: 123456,
      title: 'Server Mod',
      preview_url: null,
      tags: ['QoL', '', 'Server'],
      file_size: '2048',
      subscriptions: '99',
      time_updated: '200',
      installed: true,
      enabled: true,
      update_available: true,
    });

    expect(item).toMatchObject({
      id: '123456',
      title: 'Server Mod',
      tags: ['QoL', 'Server'],
      file_size: 2048,
      subscriptions: 99,
      time_updated: 200,
      installed: true,
      enabled: true,
      update_available: true,
    });
    expect(item.steam_url).toContain('123456');
  });

  it('maps Workshop status without exposing key material', () => {
    expect(
      mapWorkshopStatus({
        configured: true,
        key_source: 'embedded',
        app_id: 1623730,
        key: 'should-not-be-read',
      }),
    ).toEqual({
      configured: true,
      key_source: 'embedded',
      app_id: '1623730',
    });
  });

  it('posts Workshop install requests with item_id and enable fields', async () => {
    const postSpy = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: {
        ok: true,
        data: {
          id: 'job_1',
          type: 'workshop_download',
          status: 'waiting',
          progress: 0,
          message: 'queued',
          created_at: new Date(0).toISOString(),
        },
      },
      status: 202,
    } as AxiosResponse);

    await modsApi.downloadWorkshop('123456789', true);

    expect(postSpy).toHaveBeenCalledWith('/mods/workshop', { item_id: '123456789', enable: true });
    postSpy.mockRestore();
  });
});
