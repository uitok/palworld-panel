import { apiClient, handleRequest } from './client';
import type { Base, UnsupportedActionResult } from '../types';

const demoBases: Base[] = [
  {
    id: 'demo_base_01',
    name: 'Red Ridge Base',
    guild_name: 'Demo Guild',
    x: 120,
    y: -50,
    z: 10,
    structures_count: 142,
    pals_count: 15,
    status: 'Safe',
    online_members: ['DemoPlayer'],
  },
  {
    id: 'demo_base_02',
    name: 'Dragon Peak',
    guild_name: 'Speed Guild',
    x: 820,
    y: -840,
    z: 42,
    structures_count: 12,
    pals_count: 1,
    status: 'Raid',
    online_members: [],
  },
];

const mapBase = (raw: unknown): Base => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    name: String(data.name || 'Unknown Base'),
    guild_name: String(data.guild_name || data.guild || ''),
    x: Number(data.x || 0),
    y: Number(data.y || 0),
    z: Number(data.z || 0),
    structures_count: Number(data.structures_count || 0),
    pals_count: Number(data.pals_count || 0),
    status: data.status === 'Raid' ? 'Raid' : 'Safe',
    online_members: Array.isArray(data.online_members) ? data.online_members.map(String) : [],
  };
};

const mapBases = (raw: unknown): Base[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map(mapBase);
};

const unsupported = (message: string): Promise<UnsupportedActionResult> =>
  Promise.resolve({ ok: false, unsupported: true, message });

export const basesApi = {
  getBases: () =>
    handleRequest<unknown, Base[]>(() => apiClient.get('/bases'), demoBases, {
      map: (raw) => {
        const list = mapBases(raw);
        return list.length > 0 ? list : demoBases;
      },
      quiet: true,
    }),

  cleanStructures: (_baseId: string) => unsupported('当前后端未提供基地清理接口'),
  backupBase: (_baseId: string) => unsupported('当前后端未提供单基地备份接口，请使用全服备份'),
};
