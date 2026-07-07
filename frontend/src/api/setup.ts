import { apiClient, handleRequest } from './client';
import type {
  Job,
  Prerequisite,
  RuntimeMode,
  StartupConfig,
  StartupResponse,
  ValidationIssue,
} from '../types';
import { mapJob } from './tasks';

const defaultStartup: StartupConfig = {
  port: 8211,
  players: 32,
  public_lobby: false,
  public_ip: '',
  public_port: 8211,
  log_format: 'text',
  use_perf_threads: true,
  no_async_loading_thread: true,
  use_multithread_for_ds: true,
  number_of_worker_threads_server: 0,
  workshop_dir: '',
  no_mods: false,
};

const mapPrerequisites = (raw: unknown): Prerequisite[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      id: String(data.id || ''),
      label: String(data.label || data.id || ''),
      ok: Boolean(data.ok),
      required: Boolean(data.required),
      message: data.message ? String(data.message) : undefined,
    };
  });
};

const mapIssues = (raw: unknown): ValidationIssue[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      field: data.field ? String(data.field) : undefined,
      severity: String(data.severity || 'info'),
      message: String(data.message || ''),
    };
  });
};

export const mapStartup = (raw: unknown): StartupResponse => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    startup: { ...defaultStartup, ...((data.startup as Partial<StartupConfig>) || data) },
    args: Array.isArray(data.args) ? data.args.map(String) : [],
    issues: mapIssues(data.issues),
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

export const setupApi = {
  getPrerequisites: () =>
    handleRequest<unknown, Prerequisite[]>(
      () => apiClient.get('/server/prerequisites'),
      [],
      { map: mapPrerequisites, quiet: true },
    ),

  getRuntime: () =>
    handleRequest<unknown, { mode: RuntimeMode }>(
      () => apiClient.get('/server/runtime'),
      { mode: 'wine_docker' },
      {
        map: (raw) => {
          const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
          const mode = data.mode === 'windows_steamcmd' ? 'windows_steamcmd' : 'wine_docker';
          return { mode };
        },
        quiet: true,
      },
    ),

  setRuntime: (mode: RuntimeMode) =>
    handleRequest<unknown, { mode: RuntimeMode }>(
      () => apiClient.put('/server/runtime', { mode }),
      { mode },
      { quiet: true },
    ),

  getStartup: () =>
    handleRequest<unknown, StartupResponse>(
      () => apiClient.get('/server/startup'),
      { startup: defaultStartup, args: [], issues: [] },
      { map: mapStartup, quiet: true },
    ),

  setStartup: (startup: StartupConfig) =>
    handleRequest<unknown, StartupResponse>(
      () => apiClient.put('/server/startup', startup),
      { startup, args: [], issues: [] },
      { map: mapStartup, quiet: true },
    ),

  bootstrap: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/bootstrap'),
      fallbackJob('bootstrap', '已提交开服初始化任务'),
      { map: mapJob, quiet: true },
    ),

  install: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/install'),
      fallbackJob('install', '已提交安装任务'),
      { map: mapJob, quiet: true },
    ),

  initializeConfig: () =>
    handleRequest<unknown, { path?: string }>(
      () => apiClient.post('/server/initialize-config'),
      {},
      { quiet: true },
    ),
};
