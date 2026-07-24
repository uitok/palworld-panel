import { apiClient, handleRequest } from './client';
import { emptySummary, entityListQuery, mapSummary } from './entityList';
import { emptySaveIndexStatus, mapSaveIndexStatus } from './saveIndex';
import type { Base, EntityListParams, EntityListResponse, SaveIndexStatus } from '../types';

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

  getBase: (baseId: string) =>
    handleRequest<unknown, { base: Base; status: SaveIndexStatus }>(
      () => apiClient.get(`/bases/${encodeURIComponent(baseId)}`),
      { base: mapBase({}), status: emptySaveIndexStatus },
      {
        map: (raw) => {
          const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
          return {
            base: mapBase(data.base),
            status: data.status ? mapSaveIndexStatus(data.status) : emptySaveIndexStatus,
          };
        },
        quiet: true,
        fallbackOnError: false,
      },
    ),

  cleanBase: (baseId: string) =>
    handleRequest<unknown, { cleaned: boolean; saved: boolean; base: Base }>(
      () => apiClient.post(`/bases/${encodeURIComponent(baseId)}/clean`),
      { cleaned: false, saved: false, base: mapBase({}) },
      {
        map: (raw) => {
          const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
          return { cleaned: Boolean(data.cleaned), saved: Boolean(data.saved), base: mapBase(data.base) };
        },
        quiet: true,
        fallbackOnError: false,
      },
    ),
};
