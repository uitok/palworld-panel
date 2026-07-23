import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { modConfigurationsApi } from './modConfigurations';

describe('mod configuration api', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('maps adapters and requests opaque file ids', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
	  status: 200,
      data: {
        ok: true,
        data: [{
          id: 'paldefender', name: 'PalDefender', description: 'security', available: true,
          reload_behavior: 'online_reload', files: [{ id: 'opaque', name: 'Config.json', path: 'Config.json', extension: '.json', size: 12, modified_at: '2026-07-18T00:00:00Z', revision: 'rev', executable: false }],
        }],
      },
    });
    const adapters = await modConfigurationsApi.listAdapters();
    expect(adapters[0]).toMatchObject({ id: 'paldefender', files: [{ id: 'opaque' }] });

    const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
	  status: 200,
      data: { ok: true, data: { file: adapters[0].files[0], content: '{}', format: 'json', fields: [] } },
    });
    await modConfigurationsApi.getAdapter('paldefender', 'opaque');
    expect(get).toHaveBeenCalledWith('/mods/configurations/paldefender', { params: { file: 'opaque' } });
  });

  it('sends revision and executable confirmation when saving Lua', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({
	  status: 200,
      data: { ok: true, data: { file: { id: 'lua', executable: true }, content: 'Range = 2', format: 'lua' } },
    });
    await modConfigurationsApi.saveFile('local/mod', 'lua', 'Range = 2', 'revision-1', true);
    expect(put).toHaveBeenCalledWith('/mods/local%2Fmod/files/lua', {
      content: 'Range = 2', revision: 'revision-1', confirm_executable: true,
    });
  });
});
