import { apiClient, handleRequest } from './client';
import type { Job, ServerMetrics, ServerProcessStatus, ServerStatus, ServerVersionInfo } from '../types';
import { mapJob } from './tasks';

const stoppedStatus: ServerStatus = {
  status: 'stopped',
  installed: false,
  pending_restart: false,
  runtime_mode: 'wine_docker',
  setup_step: 'prerequisites',
  config_exists: false,
  container: { exists: false, status: 'missing' },
  startup_args: [],
  ports: { game: 8211, query: 27015, rest: 8212 },
  warnings: [],
  paths: {},
  cpu_percent: 0,
  memory_usage_bytes: 0,
  port: 8211,
  settings_path: '',
};

const emptyMetrics: ServerMetrics = {
  server_fps: 0,
  current_players: 0,
  max_players: 32,
  uptime: 0,
  total_pals: 0,
  active_bases: 0,
  frame_time: 0,
};

const emptyVersionInfo: ServerVersionInfo = {
  installed: false,
  current_build_id: '',
  latest_build_id: '',
  update_available: false,
  last_checked_at: '',
  source: '',
  manifest_path: '',
};

export const emptyLogs = '服务未启动，暂无日志。';

const toNumber = (value: unknown, fallback = 0) => {
  const next = Number(value);
  return Number.isFinite(next) ? next : fallback;
};

const mapContainerState = (state?: string): ServerProcessStatus => {
  switch ((state || '').toLowerCase()) {
    case 'running':
    case 'healthy':
      return 'running';
    case 'created':
    case 'restarting':
    case 'starting':
      return 'starting';
    case 'stopping':
      return 'stopping';
    case 'updating':
      return 'updating';
    case 'dead':
    case 'error':
      return 'error';
    case 'exited':
    case 'missing':
    default:
      return 'stopped';
  }
};

export const mapServerStatus = (raw: unknown): ServerStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const container =
    data.container && typeof data.container === 'object'
      ? (data.container as Record<string, unknown>)
      : {};
  const paths =
    data.paths && typeof data.paths === 'object' && !Array.isArray(data.paths)
      ? (data.paths as Record<string, string>)
      : {};
  const ports =
    data.ports && typeof data.ports === 'object' && !Array.isArray(data.ports)
      ? (data.ports as Record<string, number>)
      : stoppedStatus.ports;

  const status = mapContainerState(String(container.status || data.status || ''));

  return {
    status,
    installed: Boolean(data.installed),
    pending_restart: Boolean(data.pending_restart),
    runtime_mode: data.runtime_mode === 'windows_steamcmd' ? 'windows_steamcmd' : 'wine_docker',
    setup_step: String(data.setup_step || stoppedStatus.setup_step),
    config_exists: Boolean(data.config_exists),
    container: {
      exists: Boolean(container.exists),
      status: String(container.status || 'missing'),
    },
    startup_args: Array.isArray(data.startup_args) ? data.startup_args.map(String) : [],
    ports,
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
    paths,
    pid: data.pid ? toNumber(data.pid) : undefined,
    cpu_percent: toNumber(data.cpu_percent, stoppedStatus.cpu_percent),
    memory_usage_bytes: toNumber(data.memory_usage_bytes, stoppedStatus.memory_usage_bytes),
    port: toNumber(ports.game ?? data.port, stoppedStatus.port),
    settings_path: String(paths.palworld_settings || data.settings_path || stoppedStatus.settings_path),
    version: data.version ? String(data.version) : undefined,
  };
};

export const mapServerMetrics = (raw: unknown): ServerMetrics => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;

  return {
    server_fps: toNumber(data.server_fps ?? data.serverFPS ?? data.serverfps ?? data.fps, emptyMetrics.server_fps),
    current_players: toNumber(
      data.current_players ?? data.currentPlayerNum ?? data.currentplayernum ?? data.players,
      emptyMetrics.current_players,
    ),
    max_players: toNumber(data.max_players ?? data.maxPlayerNum ?? data.maxplayernum, emptyMetrics.max_players),
    uptime: toNumber(data.uptime ?? data.uptime_seconds, emptyMetrics.uptime),
    total_pals: toNumber(data.total_pals ?? data.pals, emptyMetrics.total_pals),
    active_bases: toNumber(data.active_bases ?? data.bases, emptyMetrics.active_bases),
    frame_time: toNumber(data.frame_time ?? data.frameTime ?? data.frametime, emptyMetrics.frame_time),
  };
};

export const mapLogs = (raw: unknown): { logs: string } => {
  if (typeof raw === 'string') {
    return { logs: raw || emptyLogs };
  }
  if (Array.isArray(raw)) {
    return { logs: raw.join('\n') || emptyLogs };
  }
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  if (Array.isArray(data.logs)) {
    return { logs: data.logs.join('\n') || emptyLogs };
  }
  return { logs: typeof data.logs === 'string' && data.logs.trim() ? data.logs : emptyLogs };
};

export const mapServerVersion = (raw: unknown): ServerVersionInfo => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    installed: Boolean(data.installed),
    current_build_id: String(data.current_build_id || ''),
    latest_build_id: String(data.latest_build_id || ''),
    update_available: Boolean(data.update_available),
    last_checked_at: String(data.last_checked_at || ''),
    source: String(data.source || ''),
    manifest_path: String(data.manifest_path || ''),
    error: data.error ? String(data.error) : undefined,
  };
};

export const serverApi = {
  getStatus: () =>
    handleRequest<unknown, ServerStatus>(() => apiClient.get('/server/status'), stoppedStatus, {
      map: mapServerStatus,
      quiet: true,
    }),

  getMetrics: () =>
    handleRequest<unknown, ServerMetrics>(() => apiClient.get('/server/metrics'), emptyMetrics, {
      map: mapServerMetrics,
      quiet: true,
    }),

  getVersion: () =>
    handleRequest<unknown, ServerVersionInfo>(() => apiClient.get('/server/version'), emptyVersionInfo, {
      map: mapServerVersion,
      quiet: true,
    }),

  getLogs: (tail = 200, search = '', level = '', since = '') => {
    const params = new URLSearchParams({ tail: String(tail) });
    if (search) params.set('search', search);
    if (level) params.set('level', level);
    if (since) params.set('since', since);
    return handleRequest<unknown, { logs: string }>(
      () => apiClient.get(`/server/logs?${params.toString()}`),
      { logs: emptyLogs },
      { map: mapLogs, quiet: true },
    );
  },

  start: () =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/start'),
      { status: 'started' },
      { quiet: true, fallbackOnError: false },
    ),

  stop: () =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/stop'),
      { status: 'stopped' },
      { quiet: true, fallbackOnError: false },
    ),

  forceStop: () =>
    handleRequest<unknown, { status: string }>(
      () => apiClient.post('/server/force-stop'),
      { status: 'stopped' },
      { quiet: true, fallbackOnError: false },
    ),

  restart: () =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/restart'),
      { status: 'restarted' },
      { quiet: true, fallbackOnError: false },
    ),

  checkVersion: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/version/check'),
      {
        id: '',
        type: 'version_check',
        status: 'waiting',
        progress: 0,
        created_at: new Date().toISOString(),
      },
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  updateIfNeeded: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/update-if-needed'),
      {
        id: '',
        type: 'smart_update',
        status: 'waiting',
        progress: 0,
        created_at: new Date().toISOString(),
      },
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  safeRestart: (waittime: number, message: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/safe-restart', { waittime, message }),
      {
        id: '',
        type: 'safe_restart',
        status: 'waiting',
        progress: 0,
        created_at: new Date().toISOString(),
      },
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  announce: (message: string) =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/announce', { message }),
      { status: 'ok' },
      { quiet: true, fallbackOnError: false },
    ),

  save: () =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/save'),
      { status: 'ok' },
      { quiet: true, fallbackOnError: false },
    ),

  shutdown: (waittime: number, message: string) =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/shutdown', { waittime, message }),
      { status: 'ok' },
      { quiet: true, fallbackOnError: false },
    ),
};
