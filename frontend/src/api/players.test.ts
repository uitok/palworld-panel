import { describe, expect, it } from 'vitest';
import { mapAccessEntries, mapPlayer, mapSaveInventoryContainers } from './players';

describe('mapAccessEntries', () => {
  it('treats null and non-array payloads as empty arrays', () => {
    expect(mapAccessEntries(null)).toEqual([]);
    expect(mapAccessEntries({})).toEqual([]);
  });

  it('maps array and wrapped players payloads', () => {
    const entry = { steam_id: '76561198000000001', nickname: 'Tester', reason: 'manual' };

    expect(mapAccessEntries([entry])).toMatchObject([entry]);
    expect(mapAccessEntries({ players: [entry] })).toMatchObject([entry]);
  });
});

describe('save player detail mappers', () => {
  it('maps one save-index player without treating binary data as display text', () => {
    expect(mapPlayer({
      steam_id: 'steam_1',
      player_uid: 'uid_1',
      nickname: 'Builder',
      level: 45,
      location_x: 10,
      location_y: 20,
      location_z: 30,
    })).toMatchObject({
      steam_id: 'steam_1',
      player_uid: 'uid_1',
      nickname: 'Builder',
      level: 45,
      x: 10,
      y: 20,
      z: 30,
    });
    expect(mapPlayer({ raw: '\u0000\u0001' })).toBeNull();
  });

  it('defaults missing online state to offline and maps backend online metadata', () => {
    expect(mapPlayer({ player_uid: 'uid-offline' })).toMatchObject({
      is_online: false,
      online_source: 'none',
      online_stale: false,
    });
    expect(mapPlayer({
      player_uid: 'uid-live',
      is_online: true,
      online_source: 'rest+paldefender',
      online_stale: true,
      gm_user_id: 'steam_76561198000000001',
    })).toMatchObject({
      is_online: true,
      online_source: 'rest+paldefender',
      online_stale: true,
      gm_user_id: 'steam_76561198000000001',
    });
  });

  it('maps parsed inventory slots and ignores entries without an ItemID', () => {
    expect(mapSaveInventoryContainers({
      containers: [{
        container_id: 'bag_1',
        owner_type: 'player',
        owner_id: 'uid_1',
        slots: [
          { slot: 0, item_id: 'Money', item_name: '金币', count: 25, durability: 0 },
          { slot: 1, item_id: '', count: 1 },
        ],
      }],
    })).toEqual([{ container_id: 'bag_1', owner_type: 'player', owner_id: 'uid_1', slots: [{ slot: 0, item_id: 'Money', item_name: '金币', count: 25, durability: 0 }] }]);
  });
});
