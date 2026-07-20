import { describe, expect, it } from 'vitest';
import type { PalDefenderGMPlayer, Player } from '../types';
import { mergePlayers } from './playerCenterMerge';

describe('PlayerCenter player merging', () => {
  it('deduplicates a save snapshot and an online-only save record with Steam prefix differences', () => {
    const saves = [
      {
        player_uid: 'save-player-uid', steam_id: '76561198370732375', nickname: '玛卡巴卡', level: 37,
        guild_name: '无名公会', is_online: false, x: 1, y: 2, z: 3, last_online_time: '',
      },
      {
        player_uid: 'live-player-uid', steam_id: 'steam_76561198370732375', nickname: '玛卡巴卡', level: 0,
        guild_name: '', is_online: true, x: 10, y: 20, z: 30, last_online_time: '',
      },
    ] as Player[];

    const merged = mergePlayers(saves, []);

    expect(merged).toHaveLength(1);
    expect(merged[0]).toMatchObject({ name: '玛卡巴卡', online: true, level: 37, guildName: '无名公会' });
  });

  it('merges save and live records across Steam and GUID formatting differences', () => {
    const saves = [
      {
        player_uid: 'ABCDEF00-0000-0000-0000-000000000001', steam_id: '76561198000000001', nickname: 'Builder', level: 40,
        guild_name: 'Guild', is_online: false, x: 1, y: 2, z: 3, last_online_time: '',
      },
      {
        player_uid: 'abcdef00000000000000000000000001', steam_id: 'steam_76561198000000001', nickname: 'Builder', level: 39,
        guild_name: 'Guild', is_online: false, x: 1, y: 2, z: 3, last_online_time: '',
      },
    ] as Player[];
    const live = [{
      UserId: 'steam_76561198000000001', PlayerUID: 'abcdef00-0000-0000-0000-000000000001', Name: 'Builder',
      GuildName: 'Guild', Status: 'Online', MapLocation: { x: 10, y: 20, z: 30 },
    }] as PalDefenderGMPlayer[];

    const merged = mergePlayers(saves, live);

    expect(merged).toHaveLength(1);
    expect(merged[0]).toMatchObject({ name: 'Builder', online: true, level: 40, x: 10, y: 20, z: 30 });
    expect(merged[0].save).toBeDefined();
    expect(merged[0].gm).toBeDefined();
  });
});
