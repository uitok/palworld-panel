import { apiClient, handleRequest } from './client';
import { emptySummary, entityListQuery, mapSummary } from './entityList';
import { emptySaveIndexStatus, mapSaveIndexStatus } from './saveIndex';
import type { Base, EntityListParams, EntityListResponse, UnsupportedActionResult } from '../types';

const mapBase = (raw: unknown): Base => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const location = data.location && typeof data.location === 'object' ? (data.location as Record<string, unknown>) : {};
  return {
    id: String(data.id || ''),
    name: String(data.name || 'Unknown Base'),
    guild_id: String(data.guild_id || ''),
    guild_name: String(data.guild_name || data.guild || ''),
    x: Number(data.x ?? location.x ?? 0),
    y: Number(data.y ?? location.y ?? 0),
    z: Number(data.z ?? location.z ?? 0),
    structures_count: Number(data.structures_count || 0),
    pals_count: Number(data.pals_count || (Array.isArray(data.workers) ? data.workers.length : 0)),
    status: data.status === 'Raid' ? 'Raid' : 'Safe',
    online_members: Array.isArray(data.online_members) ? data.online_members.map(String) : [],
    workers: Array.isArray(data.workers) ? (data.workers as Base['workers']) : [],
    containers: Array.isArray(data.containers) ? data.containers.map(String) : [],
  };
};

const mapBases = (raw: unknown): Base[] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const list = Array.isArray(raw) ? raw : Array.isArray(data.bases) ? data.bases : [];
  return list.map(mapBase);
};

const mapBasesList = (raw: unknown): EntityListResponse<Base> => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const items = mapBases(raw);
  return {
    items,
    status: data.status ? mapSaveIndexStatus(data.status) : emptySaveIndexStatus,
    summary: data.summary ? mapSummary(data.summary) : { ...emptySummary, total: items.length, returned: items.length },
  };
};

const unsupported = (message: string): Promise<UnsupportedActionResult> =>
  Promise.resolve({ ok: false, unsupported: true, message });

export const basesApi = {
  getBasesList: (params: EntityListParams = {}) =>
    handleRequest<unknown, EntityListResponse<Base>>(
      () => apiClient.get(`/bases${entityListQuery(params)}`),
      { items: [], status: emptySaveIndexStatus, summary: emptySummary },
      {
        map: mapBasesList,
        quiet: true,
      },
    ),

  getBases: () =>
    handleRequest<unknown, Base[]>(() => apiClient.get('/bases'), [], {
      map: mapBases,
      quiet: true,
    }),

  cleanStructures: (_baseId: string) => unsupported('当前后端未提供基地清理接口'),
  backupBase: (_baseId: string) => unsupported('当前后端未提供单基地备份接口，请使用全服备份'),
};
