import { apiClient, handleRequest } from './client';
import type { Job, PalDefenderRelease, PalDefenderStatus, TokenResult, UE4SSDependencyStatus, UE4SSDependencyState } from '../types';
import { createFallbackJob, mapJob } from './tasks';

export const palDefenderPanelPermissions = [
  'REST.Version.Read',
  'REST.Players.Read',
  'REST.Pals.Read',
  'REST.Pals.Give',
  'REST.PalTemplates.Give',
  'REST.Items.Read',
  'REST.Items.Give',
  'REST.Techs.Read',
  'REST.Techs.Learn',
  'REST.Techs.Forget',
  'REST.Progression.Read',
  'REST.Progression.Give',
  'REST.Messages.Send.PlayerChat',
  'REST.Messages.Send.GlobalChat',
  'REST.Messages.Send.GuildChat',
  'REST.Messages.Send.Log.Normal',
  'REST.Messages.Send.Log.Important',
  'REST.Messages.Send.Log.VeryImportant',
  'REST.Messages.Broadcast',
  'REST.Messages.Alert',
  'REST.Punishments.Kick',
  'REST.Punishments.Ban',
  'REST.Punishments.Unban',
  'REST.Reload.Config',
] as const;

const fallbackStatus: PalDefenderStatus = {
  installed: false,
  release_source: 'github_latest',
  needs_first_start: false,
  files: {},
  paths: {},
  rest_api_enabled: false,
  warnings: [],
  load_verified: false,
  ue4ss: {
    state: 'not_checked',
    installed: false,
    compatible: false,
    files: {},
    path: '',
    message: 'UE4SS status has not been checked.',
    load_verified: false,
  },
};

const dependencyStates = new Set<UE4SSDependencyState>([
  'not_checked',
  'checking',
  'missing',
  'installing',
  'installed',
  'incompatible',
  'failed',
  'rollback_required',
]);

const mapUE4SS = (raw: unknown): UE4SSDependencyStatus => {
  const data = raw && typeof raw === 'object' ? (raw as Record<string, unknown>) : {};
  const rawState = String(data.state || 'not_checked') as UE4SSDependencyState;
  return {
    state: dependencyStates.has(rawState) ? rawState : 'not_checked',
    installed: Boolean(data.installed),
    version: data.version ? String(data.version) : undefined,
    compatible: Boolean(data.compatible),
    files:
      data.files && typeof data.files === 'object' && !Array.isArray(data.files)
        ? (data.files as Record<string, boolean>)
        : {},
    path: String(data.path || ''),
    message: String(data.message || ''),
    error: data.error ? String(data.error) : undefined,
    archive_sha256: data.archive_sha256 ? String(data.archive_sha256) : undefined,
    load_verified: Boolean(data.load_verified),
    load_evidence: data.load_evidence ? String(data.load_evidence) : undefined,
  };
};
const mapStatus = (raw: unknown): PalDefenderStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    installed: Boolean(data.installed),
    version: data.version ? String(data.version) : undefined,
    release_source: String(data.release_source || 'github_latest'),
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
    ue4ss: mapUE4SS(data.ue4ss),
    load_verified: Boolean(data.load_verified),
    load_evidence: data.load_evidence ? String(data.load_evidence) : undefined,
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
    permissions: Array.isArray(data.permissions) ? data.permissions.map(String) : [...palDefenderPanelPermissions],
    path: String(data.path || ''),
  };
};

export const securityApi = {
	installUE4SS: () =>
		handleRequest<unknown, Job>(
			() => apiClient.post('/security/ue4ss/install'),
			createFallbackJob('ue4ss_install', '已提交 UE4SS 安装任务'),
			{ map: mapJob, quiet: true, fallbackOnError: false },
		),
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
      createFallbackJob('paldefender_install', '已提交 PalDefender 安装任务'),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  update: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/security/paldefender/update'),
      createFallbackJob('paldefender_update', '已提交 PalDefender 更新任务'),
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

  createToken: (name = 'AdminPanel', permissions: string[] = [...palDefenderPanelPermissions]) =>
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
