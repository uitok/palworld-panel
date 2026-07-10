import { apiClient, handleRequest } from './client';
import type { AITranslation, Job, ModItem, WorkshopItem, WorkshopSearchResponse, WorkshopStatus } from '../types';
import { mapJob } from './tasks';
import { AI_OPERATION_TIMEOUT_MS } from './requestTimeouts';

const stringArray = (raw: unknown): string[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => String(item || '').trim()).filter(Boolean);
};

const numberValue = (raw: unknown) => {
  const value = Number(raw);
  return Number.isFinite(value) ? value : undefined;
};

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
    workshop_id: data.workshop_id ? String(data.workshop_id) : undefined,
    preview_url: data.preview_url ? String(data.preview_url) : undefined,
    steam_url: data.steam_url ? String(data.steam_url) : undefined,
    summary: data.summary ? String(data.summary) : undefined,
    tags: stringArray(data.tags),
    file_size: numberValue(data.file_size),
    subscriptions: numberValue(data.subscriptions),
    time_updated: numberValue(data.time_updated),
    last_checked_at: data.last_checked_at ? String(data.last_checked_at) : undefined,
    created_at: data.created_at ? String(data.created_at) : undefined,
    updated_at: data.updated_at ? String(data.updated_at) : undefined,
  };
};
const mapMods = (raw: unknown): ModItem[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map(mapMod);
};

export const mapWorkshopItem = (raw: unknown): WorkshopItem => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const id = String(data.id || data.publishedfileid || '');
  return {
    id,
    title: String(data.title || data.name || id || 'Untitled Workshop Item'),
    summary: data.summary ? String(data.summary) : undefined,
    preview_url: data.preview_url ? String(data.preview_url) : undefined,
    steam_url: String(data.steam_url || `https://steamcommunity.com/sharedfiles/filedetails/?id=${id}`),
    tags: stringArray(data.tags),
    file_size: numberValue(data.file_size),
    subscriptions: numberValue(data.subscriptions),
    time_created: numberValue(data.time_created),
    time_updated: numberValue(data.time_updated),
    installed: Boolean(data.installed),
    enabled: Boolean(data.enabled),
    update_available: Boolean(data.update_available),
    mod_id: data.mod_id ? String(data.mod_id) : undefined,
    translation: data.translation ? mapTranslation(data.translation) : undefined,
  };
};

const mapTranslation = (raw: unknown): AITranslation => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    text: String(data.text || ''),
    target_language: String(data.target_language || 'zh-CN'),
    model: String(data.model || ''),
    generated_at: String(data.generated_at || ''),
    cached: Boolean(data.cached),
  };
};

const mapWorkshopSearchResponse = (raw: unknown): WorkshopSearchResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const items = Array.isArray(data.items) ? data.items.map(mapWorkshopItem) : [];
  return {
    items,
    next_cursor: data.next_cursor ? String(data.next_cursor) : undefined,
    total: Number(data.total) || items.length,
    page_size: Number(data.page_size) || items.length,
  };
};

export const mapWorkshopStatus = (raw: unknown): WorkshopStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    configured: Boolean(data.configured),
    key_source: data.key_source ? String(data.key_source) : undefined,
    app_id: String(data.app_id || '1623730'),
  };
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

  workshopStatus: () =>
    handleRequest<unknown, WorkshopStatus>(
      () => apiClient.get('/mods/workshop/status'),
      { configured: true, key_source: 'embedded', app_id: '1623730' },
      { map: mapWorkshopStatus, quiet: true, fallbackOnError: false },
    ),

  searchWorkshop: (params: { q?: string; sort?: string; cursor?: string; page_size?: number; tags?: string[] }) =>
    handleRequest<unknown, WorkshopSearchResponse>(
      () =>
        apiClient.get('/mods/workshop/search', {
          params: {
            q: params.q || undefined,
            sort: params.sort || undefined,
            cursor: params.cursor || undefined,
            page_size: params.page_size || undefined,
            tags: params.tags && params.tags.length > 0 ? params.tags.join(',') : undefined,
          },
        }),
      { items: [], total: 0, page_size: params.page_size || 24 },
      { map: mapWorkshopSearchResponse, quiet: true, fallbackOnError: false },
    ),

  getWorkshopItem: (itemId: string) =>
    handleRequest<unknown, WorkshopItem>(
      () => apiClient.get(`/mods/workshop/${itemId}`),
      mapWorkshopItem({ id: itemId }),
      { map: mapWorkshopItem, quiet: true, fallbackOnError: false },
    ),

  translateWorkshop: (itemId: string, force = false) =>
    handleRequest<unknown, AITranslation>(
      () => apiClient.post(`/mods/workshop/${itemId}/translate`, { force }, { timeout: AI_OPERATION_TIMEOUT_MS }),
      { text: '', target_language: 'zh-CN', model: '', generated_at: '', cached: false },
      { map: mapTranslation, quiet: true, fallbackOnError: false },
    ),

  downloadWorkshop: (itemId: string, enable = false) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/mods/workshop', { item_id: itemId, enable }),
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
