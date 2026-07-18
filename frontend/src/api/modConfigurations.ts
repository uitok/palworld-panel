import { apiClient, handleRequest } from './client';
import type {
  ModConfigBackup,
  ModConfigDocument,
  ModConfigFile,
  ModConfigurationAdapter,
  ModConfigurationField,
} from '../types';

const objectValue = (raw: unknown): Record<string, unknown> => (
  raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
);

const mapFile = (raw: unknown): ModConfigFile => {
  const value = objectValue(raw);
  return {
    id: String(value.id || ''),
    name: String(value.name || ''),
    path: String(value.path || ''),
    extension: String(value.extension || ''),
    size: Number(value.size || 0),
    modified_at: String(value.modified_at || ''),
    revision: String(value.revision || ''),
    executable: Boolean(value.executable),
    risk: value.risk ? String(value.risk) : undefined,
  };
};

const mapField = (raw: unknown): ModConfigurationField => {
  const value = objectValue(raw);
  const type = ['boolean', 'integer', 'number', 'string'].includes(String(value.type))
    ? String(value.type) as ModConfigurationField['type']
    : 'string';
  return {
    path: String(value.path || ''),
    label: String(value.label || value.path || ''),
    type,
    value: value.value,
    min: value.min == null ? undefined : Number(value.min),
    max: value.max == null ? undefined : Number(value.max),
  };
};

const mapDocument = (raw: unknown): ModConfigDocument => {
  const value = objectValue(raw);
  return {
    file: mapFile(value.file),
    content: String(value.content || ''),
    format: String(value.format || ''),
    fields: Array.isArray(value.fields) ? value.fields.map(mapField) : [],
  };
};

const mapAdapters = (raw: unknown): ModConfigurationAdapter[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const value = objectValue(item);
    return {
      id: String(value.id || ''),
      name: String(value.name || ''),
      description: String(value.description || ''),
      workshop_id: value.workshop_id ? String(value.workshop_id) : undefined,
      available: Boolean(value.available),
      reload_behavior: String(value.reload_behavior || 'restart_required'),
      files: Array.isArray(value.files) ? value.files.map(mapFile) : [],
    };
  });
};

const mapBackups = (raw: unknown): ModConfigBackup[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const value = objectValue(item);
    return {
      id: String(value.id || ''),
      revision: String(value.revision || ''),
      size: Number(value.size || 0),
      created_at: String(value.created_at || ''),
    };
  });
};

const documentFallback: ModConfigDocument = {
  file: { id: '', name: '', path: '', extension: '', size: 0, modified_at: '', revision: '', executable: false },
  content: '',
  format: '',
  fields: [],
};

const fileQuery = (fileID: string) => ({ params: fileID ? { file: fileID } : undefined });

export const modConfigurationsApi = {
  listAdapters: () => handleRequest<unknown, ModConfigurationAdapter[]>(
    () => apiClient.get('/mods/configurations'),
    [],
    { map: mapAdapters, quiet: true, fallbackOnError: false },
  ),

  getAdapter: (adapterID: string, fileID: string) => handleRequest<unknown, ModConfigDocument>(
    () => apiClient.get(`/mods/configurations/${encodeURIComponent(adapterID)}`, fileQuery(fileID)),
    documentFallback,
    { map: mapDocument, quiet: true, fallbackOnError: false },
  ),

  saveAdapter: (adapterID: string, fileID: string, content: string, revision: string, confirmExecutable = false) =>
    handleRequest<unknown, ModConfigDocument>(
      () => apiClient.put(`/mods/configurations/${encodeURIComponent(adapterID)}`, {
        content,
        revision,
        confirm_executable: confirmExecutable || undefined,
      }, fileQuery(fileID)),
      documentFallback,
      { map: mapDocument, quiet: true, fallbackOnError: false },
    ),

  listAdapterBackups: (adapterID: string, fileID: string) => handleRequest<unknown, ModConfigBackup[]>(
    () => apiClient.get(`/mods/configurations/${encodeURIComponent(adapterID)}/backups`, fileQuery(fileID)),
    [],
    { map: mapBackups, quiet: true, fallbackOnError: false },
  ),

  restoreAdapter: (adapterID: string, fileID: string, backupID: string, revision: string) =>
    handleRequest<unknown, ModConfigDocument>(
      () => apiClient.post(
        `/mods/configurations/${encodeURIComponent(adapterID)}/backups/${encodeURIComponent(backupID)}/restore`,
        { revision },
        fileQuery(fileID),
      ),
      documentFallback,
      { map: mapDocument, quiet: true, fallbackOnError: false },
    ),

  listFiles: (modID: string) => handleRequest<unknown, ModConfigFile[]>(
    () => apiClient.get(`/mods/${encodeURIComponent(modID)}/files`),
    [],
    { map: (raw) => Array.isArray(raw) ? raw.map(mapFile) : [], quiet: true, fallbackOnError: false },
  ),

  getFile: (modID: string, fileID: string) => handleRequest<unknown, ModConfigDocument>(
    () => apiClient.get(`/mods/${encodeURIComponent(modID)}/files/${encodeURIComponent(fileID)}`),
    documentFallback,
    { map: mapDocument, quiet: true, fallbackOnError: false },
  ),

  saveFile: (modID: string, fileID: string, content: string, revision: string, confirmExecutable = false) =>
    handleRequest<unknown, ModConfigDocument>(
      () => apiClient.put(`/mods/${encodeURIComponent(modID)}/files/${encodeURIComponent(fileID)}`, {
        content,
        revision,
        confirm_executable: confirmExecutable || undefined,
      }),
      documentFallback,
      { map: mapDocument, quiet: true, fallbackOnError: false },
    ),

  listFileBackups: (modID: string, fileID: string) => handleRequest<unknown, ModConfigBackup[]>(
    () => apiClient.get(`/mods/${encodeURIComponent(modID)}/files/${encodeURIComponent(fileID)}/backups`),
    [],
    { map: mapBackups, quiet: true, fallbackOnError: false },
  ),

  restoreFile: (modID: string, fileID: string, backupID: string, revision: string) =>
    handleRequest<unknown, ModConfigDocument>(
      () => apiClient.post(
        `/mods/${encodeURIComponent(modID)}/files/${encodeURIComponent(fileID)}/backups/${encodeURIComponent(backupID)}/restore`,
        { revision },
      ),
      documentFallback,
      { map: mapDocument, quiet: true, fallbackOnError: false },
    ),
};
