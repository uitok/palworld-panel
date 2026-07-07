import { apiClient, handleRequest } from './client';
import type { Player, UnsupportedActionResult } from '../types';

const demoPlayers: Player[] = [
  {
    id: '76561198000000001',
    steam_id: '76561198000000001',
    nickname: 'DemoPlayer',
    level: 45,
    guild_name: 'Demo Guild',
    is_online: true,
    last_online_time: '2026-07-07 10:45:00',
    x: 120.4,
    y: -50.2,
    z: 10,
    ping: 24,
    ip: '192.168.1.45',
  },
  {
    id: '76561198000000002',
    steam_id: '76561198000000002',
    nickname: 'Builder',
    level: 18,
    guild_name: 'Workers',
    is_online: true,
    last_online_time: '2026-07-07 10:45:00',
    x: -450.1,
    y: 200.7,
    z: 5.3,
    ping: 52,
    ip: '122.45.18.99',
  },
  {
    id: '76561198000000003',
    steam_id: '76561198000000003',
    nickname: 'Rider',
    level: 50,
    guild_name: 'Speed Guild',
    is_online: false,
    last_online_time: '2026-07-06 22:15:30',
    x: 820,
    y: -840.5,
    z: 42.1,
    ip: '184.22.110.12',
  },
];

const unsupported = (message: string): UnsupportedActionResult => ({
  ok: false,
  unsupported: true,
  message,
});

const mapPlayers = (raw: unknown): Player[] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const list = Array.isArray(data.players) ? data.players : Array.isArray(raw) ? raw : [];
  if (list.length === 0) return demoPlayers;

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

export const playersApi = {
  getPlayers: () =>
    handleRequest<unknown, Player[]>(() => apiClient.get('/server/players'), demoPlayers, {
      map: mapPlayers,
      quiet: true,
    }),

  getBanList: async () => [],

  kickPlayer: async () => unsupported('当前后端未提供玩家踢出接口'),
  banPlayer: async () => unsupported('当前后端未提供玩家封禁接口'),
  unbanPlayer: async () => unsupported('当前后端未提供玩家解封接口'),
  teleportPlayer: async () => unsupported('当前后端未提供玩家传送接口'),
  giveItem: async () => unsupported('当前后端未提供发放物品接口'),
};
