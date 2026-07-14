import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { palDefenderGMApi } from './paldefenderGM';

describe('PalDefender GM API', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('maps official uppercase player and inventory payloads', async () => {
    const get = vi.spyOn(apiClient, 'get');
    get.mockResolvedValueOnce({
      status: 200,
      data: {
        ok: true,
        data: {
          Meta: { PlayerCount: 1, OnlineCount: 1 },
          Players: [{ Name: 'Builder', UserId: 'steam_1', PlayerUID: 'uid_1', Status: 'Online', MapLocation: { x: 4, y: 5, z: 6 } }],
        },
      },
    });
    get.mockResolvedValueOnce({
      status: 200,
      data: {
        ok: true,
        data: {
          Meta: { Player: 'steam_1', PlayerUID: 'uid_1' },
          Inventory: { Items: { Available: true, UsedSlots: 1, MaxSlots: 42, FreeSlots: 41, Slots: { 0: { ItemID: 'Money', Count: 25 } } } },
        },
      },
    });

    const players = await palDefenderGMApi.players();
    const inventory = await palDefenderGMApi.inventory('steam_1');

    expect(players.Players[0]).toMatchObject({ Name: 'Builder', UserId: 'steam_1', MapLocation: { x: 4, y: 5, z: 6 } });
    expect(inventory.Inventory.Items.Slots['0']).toEqual({ ItemID: 'Money', Count: 25 });
    expect(inventory.Inventory.Armor.Slots).toEqual({});
    expect(get).toHaveBeenNthCalledWith(2, '/security/paldefender/gm/players/steam_1/inventory');
  });

  it('sends validated GM write payloads to encoded player routes', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ status: 200, data: { ok: true, data: { Success: true, Granted: { Items: 5 } } } });

    await palDefenderGMApi.giveItems('steam/user', [{ ItemID: 'Money', Count: 5 }]);
    await palDefenderGMApi.sendMessage('steam/user', { SendType: 'PlayerChat', Message: 'hello' });
    await palDefenderGMApi.ban('steam/user', { Reason: 'abuse', IP: true });

    expect(post).toHaveBeenNthCalledWith(1, '/security/paldefender/gm/players/steam%2Fuser/items', { Items: [{ ItemID: 'Money', Count: 5 }] });
    expect(post).toHaveBeenNthCalledWith(2, '/security/paldefender/gm/players/steam%2Fuser/message', { SendType: 'PlayerChat', Message: 'hello' });
    expect(post).toHaveBeenNthCalledWith(3, '/security/paldefender/gm/players/steam%2Fuser/ban', { Reason: 'abuse', IP: true });
  });

  it('maps the local item catalog and requests the full bounded list', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      status: 200,
      data: { ok: true, data: { items: [{ id: 'Money', name: '金币', icon: 'money' }], returned: 1 } },
    });

    const catalog = await palDefenderGMApi.items('', 5000);

    expect(catalog).toEqual({ items: [{ id: 'Money', name: '金币', icon: 'money' }], returned: 1 });
    expect(get).toHaveBeenCalledWith('/security/paldefender/gm/items?limit=5000');
  });
});
