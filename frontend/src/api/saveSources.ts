import { apiClient, handleRequest } from './client';
import { emptySaveIndexStatus, mapSaveIndexStatus } from './saveIndex';
import type { SaveIndexStatus, SaveSource } from '../types';

export interface SaveSourcesResponse {
  items: SaveSource[];
  active_status: SaveIndexStatus;
}

const mapSource = (raw: unknown): SaveSource => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    name: String(data.name || data.id || '未命名存档'),
    kind: String(data.kind || 'import'),
    path: data.path ? String(data.path) : undefined,
    active: Boolean(data.active),
    fingerprint: data.fingerprint ? String(data.fingerprint) : undefined,
    parser_version: data.parser_version ? String(data.parser_version) : undefined,
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
    indexed_at: data.indexed_at ? String(data.indexed_at) : undefined,
    created_at: String(data.created_at || ''),
    updated_at: String(data.updated_at || ''),
  };
};

export const saveSourcesApi = {
  list: () =>
    handleRequest<unknown, SaveSourcesResponse>(
      () => apiClient.get('/save-sources'),
      { items: [], active_status: emptySaveIndexStatus },
      {
        fallbackOnError: false,
        map: (raw) => {
          const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
          return {
            items: Array.isArray(data.items) ? data.items.map(mapSource) : [],
            active_status: data.active_status ? mapSaveIndexStatus(data.active_status) : emptySaveIndexStatus,
          };
        },
      },
    ),
  importArchive: (file: File, name: string) => {
    const body = new FormData();
    body.append('file', file);
    if (name.trim()) body.append('name', name.trim());
    return handleRequest<unknown, SaveSource>(() => apiClient.post('/save-sources/import', body, { headers: { 'Content-Type': 'multipart/form-data' } }), mapSource({}), {
      fallbackOnError: false,
      map: mapSource,
    });
  },
  activate: (id: string) => handleRequest(() => apiClient.post(`/save-sources/${encodeURIComponent(id)}/activate`), {}, { fallbackOnError: false }),
  rebuild: (id: string) => handleRequest(() => apiClient.post(`/save-sources/${encodeURIComponent(id)}/rebuild`), {}, { fallbackOnError: false }),
  rename: (id: string, name: string) => handleRequest<unknown, SaveSource>(() => apiClient.patch(`/save-sources/${encodeURIComponent(id)}`, { name }), mapSource({}), { fallbackOnError: false, map: mapSource }),
  remove: (id: string) => handleRequest(() => apiClient.delete(`/save-sources/${encodeURIComponent(id)}`), {}, { fallbackOnError: false }),
};
