import { apiClient, handleRequest } from './client';
import type {
  Job,
  ServerLogResponse,
  ServerMetrics,
  ServerProcessStatus,
  ServerStatus,
  ServerVersionInfo,
  WorldInfo,
} from '../types';
import { createFallbackJob, mapJob } from './tasks';

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
  server_imported: false,
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
  days: 0,
};

const emptyVersionInfo: ServerVersionInfo = {
  installed: false,
  current_build_id: '',
  latest_build_id: '',
  update_available: false,
  last_checked_at: '',
  source: '',
  manifest_path: '',
  compatibility_warnings: [],
};

export const emptyLogs = '服务未启动，暂无日志。';

const emptyLogResponse: ServerLogResponse = {
  logs: '',
  source: 'none',
  available: false,
  reason: 'not_started',
};

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
    server_imported: Boolean(data.server_imported),
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
  const body =
    data.body && typeof data.body === 'object' && !Array.isArray(data.body)
      ? (data.body as Record<string, unknown>)
      : data;

  return {
    server_fps: toNumber(body.server_fps ?? body.serverFPS ?? body.serverfps ?? body.serverFps ?? body.fps, emptyMetrics.server_fps),
    current_players: toNumber(
      body.current_players ?? body.currentPlayerNum ?? body.currentplayernum ?? body.players,
      emptyMetrics.current_players,
    ),
    max_players: toNumber(body.max_players ?? body.maxPlayerNum ?? body.maxplayernum, emptyMetrics.max_players),
    uptime: toNumber(body.uptime ?? body.uptime_seconds, emptyMetrics.uptime),
    total_pals: toNumber(body.total_pals ?? body.pals, emptyMetrics.total_pals),
    active_bases: toNumber(body.active_bases ?? body.bases, emptyMetrics.active_bases),
    frame_time: toNumber(
      body.frame_time ?? body.frameTime ?? body.frametime ?? body.server_frame_time ?? body.serverFrameTime ?? body.serverframetime,
      emptyMetrics.frame_time,
    ),
    days: toNumber(body.days, emptyMetrics.days),
  };
};

export const mapLogs = (raw: unknown): ServerLogResponse => {
  if (typeof raw === 'string') {
    return { logs: raw, source: 'none', available: Boolean(raw) };
  }
  if (Array.isArray(raw)) {
    const logs = raw.join('\n');
    return { logs, source: 'none', available: Boolean(logs) };
  }
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const logs = Array.isArray(data.logs) ? data.logs.join('\n') : typeof data.logs === 'string' ? data.logs : '';
  const source = ['file', 'docker', 'none'].includes(String(data.source)) ? String(data.source) : 'none';
  return {
    logs,
    source: source as ServerLogResponse['source'],
    available: typeof data.available === 'boolean' ? data.available : Boolean(logs),
    reason: data.reason ? String(data.reason) : undefined,
    updated_at: data.updated_at ? String(data.updated_at) : undefined,
  };
};

const mapWorld = (raw: unknown): WorldInfo => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    active_world_id: String(data.active_world_id || ''),
    save_exists: Boolean(data.save_exists),
    last_modified: data.last_modified ? String(data.last_modified) : undefined,
    server_running: Boolean(data.server_running),
    reset_available: Boolean(data.reset_available),
    reset_unavailable_reason: data.reset_unavailable_reason ? String(data.reset_unavailable_reason) : undefined,
  };
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
    game_version: data.game_version ? String(data.game_version) : undefined,
    compatibility_target: data.compatibility_target ? String(data.compatibility_target) : undefined,
    compatible: typeof data.compatible === 'boolean' ? data.compatible : undefined,
    compatibility_warnings: Array.isArray(data.compatibility_warnings) ? data.compatibility_warnings.map(String) : [],
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
    return handleRequest<unknown, ServerLogResponse>(
      () => apiClient.get(`/server/logs?${params.toString()}`),
      emptyLogResponse,
      { map: mapLogs, quiet: true },
    );
  },

  getWorld: () =>
    handleRequest<unknown, WorldInfo>(
      () => apiClient.get('/server/world'),
      {
        active_world_id: '',
        save_exists: false,
        server_running: false,
        reset_available: false,
        reset_unavailable_reason: 'world_not_found',
      },
      { map: mapWorld, quiet: true, fallbackOnError: false },
    ),

  resetWorld: (worldId: string, confirmation: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/world/reset', { world_id: worldId, confirmation }),
      createFallbackJob('world_reset', undefined, ''),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

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
      createFallbackJob('version_check', undefined, ''),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  updateIfNeeded: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/update-if-needed'),
      createFallbackJob('smart_update', undefined, ''),
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  safeRestart: (waittime: number, message: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/safe-restart', { waittime, message }),
      createFallbackJob('safe_restart', undefined, ''),
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
