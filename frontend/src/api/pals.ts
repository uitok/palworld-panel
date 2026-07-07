import { apiClient, handleRequest } from './client';
import type { Pal, UnsupportedActionResult } from '../types';

const mapPal = (raw: unknown): Pal => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || data.instance_id || ''),
    name: String(data.name || data.character_id || 'Unknown Pal'),
    level: Number(data.level || 1),
    rarity: data.rarity === 'Boss' || data.rarity === 'Rare' ? data.rarity : 'Common',
    owner_nickname: String(data.owner_nickname || data.owner || ''),
    owner_steam_id: String(data.owner_steam_id || ''),
    skills: Array.isArray(data.skills) ? (data.skills as Pal['skills']) : [],
    work_suitability: Array.isArray(data.work_suitability) ? (data.work_suitability as Pal['work_suitability']) : [],
    health: Number(data.health || 0),
    max_health: Number(data.max_health || data.health || 0),
    status:
      data.status === 'Healthy' ||
      data.status === 'Injured' ||
      data.status === 'Working' ||
      data.status === 'Battling' ||
      data.status === 'Dead'
        ? data.status
        : 'Healthy',
    x: Number(data.x || 0),
    y: Number(data.y || 0),
    z: Number(data.z || 0),
  };
};

const mapPals = (raw: unknown): Pal[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map(mapPal);
};

const unsupported = (message: string): Promise<UnsupportedActionResult> =>
  Promise.resolve({ ok: false, unsupported: true, message });

export const palsApi = {
  getPals: () =>
    handleRequest<unknown, Pal[]>(() => apiClient.get('/pals'), [], {
      map: mapPals,
      quiet: true,
    }),

  updateLevel: (_palId: string, _level: number) => unsupported('当前后端未提供帕鲁等级修改接口'),
  heal: (_palId: string) => unsupported('当前后端未提供帕鲁治疗接口'),
  delete: (_palId: string) => unsupported('当前后端未提供帕鲁释放接口'),
};
