import type { PalDefenderGMPlayer, Player } from '../types';

export interface UnifiedPlayer {
  key: string;
  name: string;
  identifier: string;
  playerUID: string;
  platform: string;
  online: boolean;
  guildName: string;
  level: number;
  x: number;
  y: number;
  z: number;
  lastOnline: string;
  save?: Player;
  gm?: PalDefenderGMPlayer;
}

const identifierForGM = (player: PalDefenderGMPlayer) => player.UserId || player.PlayerUID;
const gmOnline = (player?: PalDefenderGMPlayer) => player?.Status.toLowerCase() === 'online';

export const mergePlayers = (savePlayers: Player[], gmPlayers: PalDefenderGMPlayer[]): UnifiedPlayer[] => {
  const result: UnifiedPlayer[] = [];
  for (const save of savePlayers) {
    const aliases = playerIdentityAliases(save.player_uid, save.steam_id);
    const existing = result.find((player) => identitiesOverlap(aliases, playerIdentityAliases(player.identifier, player.playerUID, player.save?.steam_id, player.save?.player_uid)));
    if (existing) {
      if (!existing.save || save.level >= existing.save.level) {
        existing.save = save;
        existing.name = save.nickname || existing.name;
        existing.identifier = save.steam_id || save.player_uid || existing.identifier;
        existing.playerUID = save.player_uid || existing.playerUID;
        existing.level = Math.max(existing.level, save.level);
        existing.guildName = save.guild_name || existing.guildName;
        existing.online = existing.online || save.is_online;
        existing.x = save.x;
        existing.y = save.y;
        existing.z = save.z;
        existing.lastOnline = save.last_online_time || existing.lastOnline;
      }
      continue;
    }
    result.push({
      key: save.player_uid || save.steam_id || `save-${result.length}`,
      name: save.nickname,
      identifier: save.steam_id || save.player_uid || '',
      playerUID: save.player_uid || '',
      platform: (save.steam_id || '').split('_')[0] || '',
      online: save.is_online,
      guildName: save.guild_name,
      level: save.level,
      x: save.x,
      y: save.y,
      z: save.z,
      lastOnline: save.last_online_time,
      save,
    });
  }
  for (const gm of gmPlayers) {
    const aliases = playerIdentityAliases(gm.UserId, gm.PlayerUID);
    const existing = result.find((player) => identitiesOverlap(aliases, playerIdentityAliases(player.identifier, player.playerUID, player.save?.steam_id, player.save?.player_uid)));
    if (existing) {
      existing.gm = gm;
      existing.identifier = identifierForGM(gm) || existing.identifier;
      existing.playerUID = gm.PlayerUID || existing.playerUID;
      existing.name = gm.Name || existing.name;
      existing.online = gmOnline(gm);
      existing.guildName = gm.GuildName || existing.guildName;
      existing.platform = (gm.UserId || existing.identifier).split('_')[0] || existing.platform;
      if (existing.online) {
        existing.x = gm.MapLocation.x;
        existing.y = gm.MapLocation.y;
        existing.z = gm.MapLocation.z;
      }
      continue;
    }
    result.push({ key: identifierForGM(gm), name: gm.Name, identifier: identifierForGM(gm), playerUID: gm.PlayerUID, platform: gm.UserId.split('_')[0] || '', online: gmOnline(gm), guildName: gm.GuildName, level: 0, x: gm.MapLocation.x, y: gm.MapLocation.y, z: gm.MapLocation.z, lastOnline: '', gm });
  }
  return result.sort((left, right) => Number(right.online) - Number(left.online) || left.name.localeCompare(right.name, 'zh-CN'));
};

const playerIdentityAliases = (...values: Array<string | undefined>): Set<string> => {
  const aliases = new Set<string>();
  for (const value of values) {
    const normalized = String(value || '').trim().toLowerCase().replace(/[^a-z0-9]/g, '');
    if (!normalized) continue;
    aliases.add(normalized);
    if (normalized.startsWith('steam') && normalized.length > 5) aliases.add(normalized.slice(5));
  }
  return aliases;
};

const identitiesOverlap = (left: Set<string>, right: Set<string>) => [...left].some((value) => right.has(value));
