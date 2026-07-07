import { apiClient, handleRequest } from './client';
import type { ServerMetrics, ServerProcessStatus, ServerStatus } from '../types';

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
  paths: {
    palworld_settings: 'D:/WL/me/pal/data/server/Pal/Saved/Config/WindowsServer/PalWorldSettings.ini',
  },
  cpu_percent: 0,
  memory_usage_bytes: 0,
  port: 8211,
  settings_path: 'D:/WL/me/pal/data/server/Pal/Saved/Config/WindowsServer/PalWorldSettings.ini',
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

  getLogs: (tail = 200) =>
    handleRequest<unknown, { logs: string }>(
      () => apiClient.get(`/server/logs?tail=${tail}`),
      { logs: emptyLogs },
      { map: mapLogs, quiet: true },
    ),

  start: () =>
    handleRequest<{ status: string }>(() => apiClient.post('/server/start'), { status: 'started' }, { quiet: true }),

  stop: () =>
    handleRequest<{ status: string }>(() => apiClient.post('/server/stop'), { status: 'stopped' }, { quiet: true }),

  restart: () =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/restart'),
      { status: 'restarted' },
      { quiet: true },
    ),

  announce: (message: string) =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/announce', { message }),
      { status: 'ok' },
      { quiet: true },
    ),

  save: () =>
    handleRequest<{ status: string }>(() => apiClient.post('/server/save'), { status: 'ok' }, { quiet: true }),

  shutdown: (waittime: number, message: string) =>
    handleRequest<{ status: string }>(
      () => apiClient.post('/server/shutdown', { waittime, message }),
      { status: 'ok' },
      { quiet: true },
    ),
};
