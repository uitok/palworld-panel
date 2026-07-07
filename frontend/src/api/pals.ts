import { apiClient, handleRequest } from './client';
import type { Pal, UnsupportedActionResult } from '../types';

const demoPals: Pal[] = [
  {
    id: 'demo_anubis',
    name: 'Anubis',
    level: 40,
    rarity: 'Boss',
    owner_nickname: 'DemoPlayer',
    owner_steam_id: '76561198000000001',
    skills: [
      { name: 'Ground Smash', type: 'Ground', power: 100 },
      { name: 'Sand Tornado', type: 'Ground', power: 80 },
    ],
    work_suitability: [
      { type: 'Handiwork', level: 4 },
      { type: 'Mining', level: 3 },
      { type: 'Transport', level: 2 },
    ],
    health: 3820,
    max_health: 3820,
    status: 'Working',
    x: 122,
    y: -49.5,
    z: 10,
  },
  {
    id: 'demo_jetragon',
    name: 'Jetragon',
    level: 50,
    rarity: 'Boss',
    owner_nickname: 'DemoRider',
    owner_steam_id: '76561198000000003',
    skills: [
      { name: 'Dragon Meteor', type: 'Dragon', power: 150 },
      { name: 'Fire Ball', type: 'Fire', power: 120 },
    ],
    work_suitability: [{ type: 'Gathering', level: 3 }],
    health: 4800,
    max_health: 5500,
    status: 'Battling',
    x: 820,
    y: -840.5,
    z: 42.1,
  },
  {
    id: 'demo_mossanda',
    name: 'Mossanda',
    level: 30,
    rarity: 'Common',
    owner_nickname: 'DemoBuilder',
    owner_steam_id: '76561198000000004',
    skills: [{ name: 'Seed Mine', type: 'Plant', power: 65 }],
    work_suitability: [
      { type: 'Planting', level: 2 },
      { type: 'Lumbering', level: 2 },
      { type: 'Transport', level: 2 },
    ],
    health: 240,
    max_health: 3200,
    status: 'Injured',
    x: 15.8,
    y: -22.1,
    z: 1.5,
  },
];

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
    handleRequest<unknown, Pal[]>(() => apiClient.get('/pals'), demoPals, {
      map: (raw) => {
        const list = mapPals(raw);
        return list.length > 0 ? list : demoPals;
      },
      quiet: true,
    }),

  updateLevel: (_palId: string, _level: number) => unsupported('当前后端未提供帕鲁等级修改接口'),
  heal: (_palId: string) => unsupported('当前后端未提供帕鲁治疗接口'),
  delete: (_palId: string) => unsupported('当前后端未提供帕鲁释放接口'),
};
