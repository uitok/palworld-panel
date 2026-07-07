import { apiClient, handleRequest } from './client';
import type { Player, PlayerAccessEntry, UnsupportedActionResult } from '../types';

const unsupported = (message: string): UnsupportedActionResult => ({
  ok: false,
  unsupported: true,
  message,
});

const mapPlayers = (raw: unknown): Player[] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const body = data.body && typeof data.body === 'object' ? (data.body as Record<string, unknown>) : data;
  const list = Array.isArray(body.players) ? body.players : Array.isArray(raw) ? raw : [];

  return list.map((item, index) => {
    const player = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    const steamId = String(player.userId || player.steam_id || player.playerId || `online_${index}`);
    return {
      id: steamId,
      steam_id: steamId,
      nickname: String(player.name || player.nickname || player.playerName || `Player ${index + 1}`),
      level: Number(player.level || 0),
      guild_name: String(player.guild_name || player.guild || '未知公会'),
      is_online: player.is_online == null ? true : Boolean(player.is_online),
      last_online_time: String(player.last_online_time || new Date().toISOString()),
      x: Number(player.location_x || player.x || 0),
      y: Number(player.location_y || player.y || 0),
      z: Number(player.location_z || player.z || 0),
      ping: player.ping ? Number(player.ping) : undefined,
      ip: player.ip ? String(player.ip) : undefined,
    };
  });
};

const mapAccessEntries = (raw: unknown): PlayerAccessEntry[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      steam_id: String(data.steam_id || ''),
      nickname: data.nickname ? String(data.nickname) : undefined,
      reason: data.reason ? String(data.reason) : undefined,
      created_at: data.created_at ? String(data.created_at) : undefined,
      updated_at: data.updated_at ? String(data.updated_at) : undefined,
    };
  });
};

export const playersApi = {
  getPlayers: () =>
    handleRequest<unknown, Player[]>(() => apiClient.get('/server/players'), [], {
      map: mapPlayers,
      quiet: true,
    }),

  getBanList: () =>
    handleRequest<unknown, PlayerAccessEntry[]>(() => apiClient.get('/players/bans'), [], {
      map: mapAccessEntries,
      quiet: true,
    }),

  getWhitelist: () =>
    handleRequest<unknown, PlayerAccessEntry[]>(() => apiClient.get('/players/whitelist'), [], {
      map: mapAccessEntries,
      quiet: true,
    }),

  banPlayer: (steam_id: string, nickname = '', reason = '') =>
    handleRequest<unknown, PlayerAccessEntry>(
      () => apiClient.post('/players/bans', { steam_id, nickname, reason }),
      { steam_id, nickname, reason },
      { quiet: true, fallbackOnError: false },
    ),

  unbanPlayer: (steam_id: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.delete(`/players/bans/${encodeURIComponent(steam_id)}`),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),

  setWhitelist: (players: PlayerAccessEntry[]) =>
    handleRequest<unknown, { players: PlayerAccessEntry[] }>(
      () => apiClient.put('/players/whitelist', { players }),
      { players },
      { quiet: true, fallbackOnError: false },
    ),

  kickPlayer: (steam_id: string) =>
    handleRequest<unknown, UnsupportedActionResult>(
      () => apiClient.post(`/players/${encodeURIComponent(steam_id)}/kick`),
      unsupported('当前运行环境暂不支持踢出玩家'),
      { quiet: true, fallbackOnError: false },
    ),

  teleportPlayer: async () => unsupported('当前后端未提供玩家传送接口'),
  giveItem: async () => unsupported('当前后端未提供发放物品接口'),
};
