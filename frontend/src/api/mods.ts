import { apiClient, handleRequest } from './client';
import type { Job, ModItem } from '../types';
import { mapJob } from './tasks';

const mapMod = (raw: unknown): ModItem => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    name: String(data.name || data.package_name || 'Unnamed Mod'),
    source: String(data.source || 'upload'),
    package_name: String(data.package_name || ''),
    path: String(data.path || ''),
    version: data.version ? String(data.version) : undefined,
    enabled: Boolean(data.enabled),
    created_at: data.created_at ? String(data.created_at) : undefined,
    updated_at: data.updated_at ? String(data.updated_at) : undefined,
  };
};
const mapMods = (raw: unknown): ModItem[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map(mapMod);
};

const fallbackJob = (type: string, message: string): Job => ({
  id: `local_${Date.now()}`,
  type,
  status: 'waiting',
  progress: 0,
  message,
  created_at: new Date().toISOString(),
});

export const modsApi = {
  list: () =>
    handleRequest<unknown, ModItem[]>(() => apiClient.get('/mods'), [], {
      map: mapMods,
      quiet: true,
    }),

  upload: (file: File, enable: boolean) => {
    const form = new FormData();
    form.append('file', file);
    form.append('enable', String(enable));

    return handleRequest<unknown, ModItem>(
      () =>
        apiClient.post('/mods/upload', form, {
          headers: { 'Content-Type': 'multipart/form-data' },
        }),
      {
        id: `upload_${Date.now()}`,
        name: file.name,
        source: 'upload',
        package_name: '',
        path: '',
        enabled: enable,
      },
      { map: mapMod, quiet: true, fallbackOnError: false },
    );
  },

  downloadWorkshop: (itemId: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/mods/workshop', { item_id: itemId }),
      fallbackJob('workshop_download', '已提交 Workshop 下载任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  setEnabled: (id: string, enabled: boolean) =>
    handleRequest<unknown, ModItem>(
      () => apiClient.post(`/mods/${id}/${enabled ? 'enable' : 'disable'}`),
      {
        id,
        name: id,
        source: 'upload',
        package_name: '',
        path: '',
        enabled,
      },
      { map: mapMod, quiet: true, fallbackOnError: false },
    ),

  delete: (id: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.delete(`/mods/${id}`),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),
};
