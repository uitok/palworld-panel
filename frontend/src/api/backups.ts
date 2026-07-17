import { apiClient, handleRequest } from './client';
import type { BackupInfo, BackupVerifyResult, Job, WebDAVConfig, WebDAVConfigUpdate } from '../types';
import { createFallbackJob, mapJob } from './tasks';

const mapBackups = (raw: unknown): BackupInfo[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      name: String(data.name || ''),
      path: String(data.path || ''),
      size_bytes: Number(data.size_bytes || 0),
      created_at: String(data.created_at || ''),
      reason: data.reason ? String(data.reason) : undefined,
      status: data.status ? String(data.status) : undefined,
    };
  });
};

const mapVerify = (raw: unknown): BackupVerifyResult => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    name: String(data.name || ''),
    valid: Boolean(data.valid),
    format: String(data.format || 'unknown'),
    checked_files: Number(data.checked_files || 0),
    errors: Array.isArray(data.errors) ? data.errors.map(String) : [],
  };
};

const mapWebDAVConfig = (raw: unknown): WebDAVConfig => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    enabled: Boolean(data.enabled),
    base_url: String(data.base_url || ''),
    username: String(data.username || ''),
    remote_path: String(data.remote_path || 'PalPanel'),
    upload_after_backup: Boolean(data.upload_after_backup),
    password_configured: Boolean(data.password_configured),
  };
};

const emptyWebDAVConfig: WebDAVConfig = {
  enabled: false,
  base_url: '',
  username: '',
  remote_path: 'PalPanel',
  upload_after_backup: false,
  password_configured: false,
};

export const backupsApi = {
  list: () =>
    handleRequest<unknown, BackupInfo[]>(() => apiClient.get('/backups'), [], {
      map: mapBackups,
      quiet: true,
    }),

  create: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/backup'),
      createFallbackJob('backup', '已提交备份任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  restore: (name: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post(`/backups/${encodeURIComponent(name)}/restore`),
      createFallbackJob('restore', '已提交恢复任务', ''),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  verify: (name: string) =>
    handleRequest<unknown, BackupVerifyResult>(
      () => apiClient.post(`/backups/${encodeURIComponent(name)}/verify`),
      { name, valid: false, format: 'unknown', checked_files: 0, errors: [] },
      { map: mapVerify, quiet: true, fallbackOnError: false },
    ),

  delete: (name: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.delete(`/backups/${encodeURIComponent(name)}`),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),

  getWebDAVConfig: () =>
    handleRequest<unknown, WebDAVConfig>(
      () => apiClient.get('/backups/webdav/config'),
      emptyWebDAVConfig,
      { map: mapWebDAVConfig, quiet: true, fallbackOnError: false },
    ),

  updateWebDAVConfig: (update: WebDAVConfigUpdate) =>
    handleRequest<unknown, WebDAVConfig>(
      () => apiClient.put('/backups/webdav/config', update),
      emptyWebDAVConfig,
      { map: mapWebDAVConfig, quiet: true, fallbackOnError: false },
    ),

  testWebDAVConfig: (update: WebDAVConfigUpdate) =>
    handleRequest<unknown, { connected: boolean }>(
      () => apiClient.post('/backups/webdav/test', update),
      { connected: false },
      { quiet: true, fallbackOnError: false },
    ),

  uploadWebDAV: (name: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post(`/backups/${encodeURIComponent(name)}/upload-webdav`),
      createFallbackJob('webdav_upload', '已提交 WebDAV 上传任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  download: (name: string) =>
    handleRequest<unknown, Blob>(
      () => apiClient.get(`/backups/${encodeURIComponent(name)}/download`, { responseType: 'blob' }),
      new Blob(),
      {
        map: (raw) => (raw instanceof Blob ? raw : new Blob([raw as BlobPart])),
        quiet: true,
        fallbackOnError: false,
      },
    ),

  downloadUrl: (name: string) => `${apiClient.defaults.baseURL}/backups/${encodeURIComponent(name)}/download`,
};
