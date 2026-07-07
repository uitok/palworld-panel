import { apiClient, handleRequest } from './client';
import type { Job, PalDefenderRelease, PalDefenderStatus, TokenResult } from '../types';
import { mapJob } from './tasks';

const fallbackStatus: PalDefenderStatus = {
  installed: false,
  needs_first_start: false,
  files: {},
  paths: {},
  rest_api_enabled: false,
  warnings: [],
};
const mapStatus = (raw: unknown): PalDefenderStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    installed: Boolean(data.installed),
    version: data.version ? String(data.version) : undefined,
    needs_first_start: Boolean(data.needs_first_start),
    files:
      data.files && typeof data.files === 'object' && !Array.isArray(data.files)
        ? (data.files as Record<string, boolean>)
        : {},
    paths:
      data.paths && typeof data.paths === 'object' && !Array.isArray(data.paths)
        ? (data.paths as Record<string, string>)
        : {},
    rest_api_enabled: Boolean(data.rest_api_enabled),
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
  };
};

const mapReleases = (raw: unknown): PalDefenderRelease[] => {
  if (!Array.isArray(raw)) return [];
  return raw as PalDefenderRelease[];
};

const mapToken = (raw: unknown): TokenResult => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    name: String(data.name || 'AdminPanel'),
    token: String(data.token || ''),
    permissions: Array.isArray(data.permissions) ? data.permissions.map(String) : ['REST.*'],
    path: String(data.path || ''),
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

export const securityApi = {
  releases: () =>
    handleRequest<unknown, PalDefenderRelease[]>(
      () => apiClient.get('/security/paldefender/releases'),
      [],
      { map: mapReleases, quiet: true },
    ),

  status: () =>
    handleRequest<unknown, PalDefenderStatus>(
      () => apiClient.get('/security/paldefender/status'),
      fallbackStatus,
      { map: mapStatus, quiet: true },
    ),

  install: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/security/paldefender/install'),
      fallbackJob('paldefender_install', '已提交 PalDefender 安装任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  update: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/security/paldefender/update'),
      fallbackJob('paldefender_update', '已提交 PalDefender 更新任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  rollback: () =>
    handleRequest<unknown, PalDefenderStatus>(
      () => apiClient.post('/security/paldefender/rollback'),
      fallbackStatus,
      { map: mapStatus, quiet: true, fallbackOnError: false },
    ),

  getConfig: () =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.get('/security/paldefender/config'),
      {},
      { quiet: true },
    ),

  putConfig: (config: Record<string, unknown>) =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.put('/security/paldefender/config', config),
      config,
      { quiet: true, fallbackOnError: false },
    ),

  applyPreset: (name = 'balanced') =>
    handleRequest<unknown, Record<string, unknown>>(
      () => apiClient.post('/security/paldefender/apply-preset', { name }),
      {},
      { quiet: true, fallbackOnError: false },
    ),

  createToken: (name = 'AdminPanel', permissions: string[] = ['REST.*']) =>
    handleRequest<unknown, TokenResult>(
      () => apiClient.post('/security/paldefender/rest-token', { name, permissions }),
      { name, token: '', permissions, path: '' },
      { map: mapToken, quiet: true, fallbackOnError: false },
    ),

  reloadConfig: () =>
    handleRequest<unknown, { reloaded: boolean }>(
      () => apiClient.post('/security/paldefender/reload-config'),
      { reloaded: false },
      { quiet: true, fallbackOnError: false },
    ),
};
