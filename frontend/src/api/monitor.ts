import { apiClient, handleRequest } from './client';
import type { DebugLogStatus, MonitorSample, MonitorSnapshot } from '../types';

const emptyDebugStatus: DebugLogStatus = {
  enabled: false,
  path: '',
  size: 0,
  max_bytes: 20 * 1024 * 1024,
  max_files: 5,
};

const emptySample: MonitorSample = {
  id: '',
  created_at: '',
  cpu_available: false,
  cpu_percent: 0,
  memory_available: false,
  memory_usage_bytes: 0,
  memory_limit_bytes: 0,
  disk_available: false,
  disk_free_bytes: 0,
  disk_total_bytes: 0,
  current_players: 0,
  max_players: 0,
  rest_healthy: false,
  rcon_healthy: false,
  game_port_healthy: false,
  query_port_healthy: false,
};

const mapSample = (raw: unknown): MonitorSample => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    created_at: String(data.created_at || ''),
    cpu_available: Boolean(data.cpu_available),
    cpu_percent: Number(data.cpu_percent || 0),
    memory_available: Boolean(data.memory_available),
    memory_usage_bytes: Number(data.memory_usage_bytes || 0),
    memory_limit_bytes: Number(data.memory_limit_bytes || 0),
    disk_available: Boolean(data.disk_available),
    disk_free_bytes: Number(data.disk_free_bytes || 0),
    disk_total_bytes: Number(data.disk_total_bytes || 0),
    current_players: Number(data.current_players || 0),
    max_players: Number(data.max_players || 0),
    rest_healthy: Boolean(data.rest_healthy),
    rcon_healthy: Boolean(data.rcon_healthy),
    game_port_healthy: Boolean(data.game_port_healthy),
    query_port_healthy: Boolean(data.query_port_healthy),
    unavailable_reason: data.unavailable_reason ? String(data.unavailable_reason) : undefined,
  };
};

const mapSnapshot = (raw: unknown): MonitorSnapshot => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return { sample: mapSample(data.sample) };
};

const mapHistory = (raw: unknown): MonitorSample[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map(mapSample);
};

const mapDebugStatus = (raw: unknown): DebugLogStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    enabled: Boolean(data.enabled),
    path: String(data.path || ''),
    size: Number(data.size || 0),
    max_bytes: Number(data.max_bytes || 20 * 1024 * 1024),
    max_files: Number(data.max_files || 5),
  };
};

export const monitorApi = {
  snapshot: () =>
    handleRequest<unknown, MonitorSnapshot>(
      () => apiClient.get('/monitor/snapshot'),
      { sample: emptySample },
      { map: mapSnapshot, quiet: true },
    ),

  history: (limit = 120) =>
    handleRequest<unknown, MonitorSample[]>(
      () => apiClient.get(`/monitor/history?limit=${limit}`),
      [],
      { map: mapHistory, quiet: true },
    ),

  debugStatus: () =>
    handleRequest<unknown, DebugLogStatus>(
      () => apiClient.get('/system/debug'),
      emptyDebugStatus,
      { map: mapDebugStatus, quiet: true, fallbackOnError: true },
    ),

  setDebug: (enabled: boolean) =>
    handleRequest<unknown, DebugLogStatus>(
      () => apiClient.put('/system/debug', { enabled }),
      emptyDebugStatus,
      { map: mapDebugStatus, quiet: true, fallbackOnError: false },
    ),
};
