import { apiClient, handleRequest } from './client';
import { emptySummary, entityListQuery, mapSummary } from './entityList';
import { emptySaveIndexStatus, mapSaveIndexStatus } from './saveIndex';
import type { EntityListParams, EntityListResponse, Player, PlayerAccessEntry, UnsupportedActionResult } from '../types';

const mapPlayers = (raw: unknown): Player[] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const body = data.body && typeof data.body === 'object' ? (data.body as Record<string, unknown>) : data;
  const list = Array.isArray(body.players) ? body.players : Array.isArray(raw) ? raw : [];

  return list.flatMap((item) => {
    const player = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    const steamId = String(player.userId || player.steam_id || player.playerId || player.player_uid || '').trim();
    const playerUid = String(player.player_uid || '').trim();
    if (!steamId && !playerUid) return [];
    return [
      {
        id: steamId || playerUid,
        steam_id: steamId,
        player_uid: playerUid,
        nickname: String(player.name || player.nickname || player.playerName || '未知玩家'),
        level: Number(player.level || 0),
        guild_id: String(player.guild_id || ''),
        guild_name: String(player.guild_name || player.guild || '未知公会'),
        is_online: player.is_online == null ? true : Boolean(player.is_online),
        last_online_time: String(player.last_online_time || ''),
        x: Number(player.location_x || player.x || 0),
        y: Number(player.location_y || player.y || 0),
        z: Number(player.location_z || player.z || 0),
        ping: player.ping ? Number(player.ping) : undefined,
        ip: player.ip ? String(player.ip) : undefined,
        inventory_summary:
          player.inventory_summary && typeof player.inventory_summary === 'object'
            ? (player.inventory_summary as Record<string, unknown>)
            : undefined,
      },
    ];
  });
};

const mapPlayersList = (raw: unknown): EntityListResponse<Player> => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const items = mapPlayers(raw);
  return {
    items,
    status: data.status ? mapSaveIndexStatus(data.status) : emptySaveIndexStatus,
    summary: data.summary ? mapSummary(data.summary) : { ...emptySummary, total: items.length, returned: items.length },
  };
};

export const mapAccessEntries = (raw: unknown): PlayerAccessEntry[] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const list = Array.isArray(raw) ? raw : Array.isArray(data.players) ? data.players : [];
  return list.flatMap((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    const steamId = String(data.steam_id || '').trim();
    if (!steamId) return [];
    return [
      {
        steam_id: steamId,
        nickname: data.nickname ? String(data.nickname) : undefined,
        reason: data.reason ? String(data.reason) : undefined,
        created_at: data.created_at ? String(data.created_at) : undefined,
        updated_at: data.updated_at ? String(data.updated_at) : undefined,
      },
    ];
  });
};

export const playersApi = {
  getPlayersList: (params: EntityListParams = {}) =>
    handleRequest<unknown, EntityListResponse<Player>>(
      () => apiClient.get(`/players${entityListQuery(params)}`),
      { items: [], status: emptySaveIndexStatus, summary: emptySummary },
      {
        map: mapPlayersList,
        quiet: true,
      },
    ),

  getPlayers: () =>
    handleRequest<unknown, Player[]>(() => apiClient.get('/players'), [], {
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

  addLocalBan: (entry: PlayerAccessEntry) =>
    handleRequest<unknown, PlayerAccessEntry>(
      () => apiClient.post('/players/bans', entry),
      entry,
      { quiet: true, fallbackOnError: false },
    ),

  deleteLocalBan: (steam_id: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.delete(`/players/bans/${encodeURIComponent(steam_id)}`),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),

  banPlayer: (steam_id: string, nickname = '', reason = '') =>
    handleRequest<unknown, { player_id: string; ban?: PlayerAccessEntry }>(
      () => apiClient.post(`/players/${encodeURIComponent(steam_id)}/ban`, { userid: steam_id, nickname, reason }),
      { player_id: steam_id, ban: { steam_id, nickname, reason } },
      { quiet: true, fallbackOnError: false },
    ),

  unbanPlayer: (steam_id: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.post(`/players/${encodeURIComponent(steam_id)}/unban`, { userid: steam_id }),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),

  setWhitelist: (players: PlayerAccessEntry[]) =>
    handleRequest<unknown, { players: PlayerAccessEntry[] }>(
      () => apiClient.put('/players/whitelist', { players }),
      { players },
      { quiet: true, fallbackOnError: false },
    ),

  kickPlayer: (steam_id: string, message = '') =>
    handleRequest<unknown, { player_id: string }>(
      () => apiClient.post(`/players/${encodeURIComponent(steam_id)}/kick`, { userid: steam_id, message }),
      { player_id: steam_id },
      { quiet: true, fallbackOnError: false },
    ),

  teleportPlayer: async (): Promise<UnsupportedActionResult> => ({
    ok: false,
    unsupported: true,
    message: '当前后端未提供玩家传送接口',
  }),
  giveItem: async (): Promise<UnsupportedActionResult> => ({
    ok: false,
    unsupported: true,
    message: '当前后端未提供发放物品接口',
  }),
};
