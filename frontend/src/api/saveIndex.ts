import { apiClient, handleRequest } from './client';
import type { SaveIndexStatus } from '../types';

export const emptySaveIndexStatus: SaveIndexStatus = {
  enabled: false,
  state: 'disabled',
  stale: false,
  source_path: '',
  updated_at: '',
  duration_ms: 0,
  warnings: [],
  counts: {
    players: 0,
    guilds: 0,
    bases: 0,
    pals: 0,
    containers: 0,
    map_entities: 0,
  },
};

export const mapSaveIndexStatus = (raw: unknown): SaveIndexStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const counts = data.counts && typeof data.counts === 'object' ? (data.counts as Record<string, unknown>) : {};
  return {
    enabled: Boolean(data.enabled),
    state: String(data.state || emptySaveIndexStatus.state),
    stale: Boolean(data.stale),
    source_path: String(data.source_path || ''),
    updated_at: String(data.updated_at || ''),
    duration_ms: Number(data.duration_ms || 0),
    error: data.error ? String(data.error) : undefined,
    warnings: Array.isArray(data.warnings) ? data.warnings.map(String) : [],
    counts: {
      players: Number(counts.players || 0),
      guilds: Number(counts.guilds || 0),
      bases: Number(counts.bases || 0),
      pals: Number(counts.pals || 0),
      containers: Number(counts.containers || 0),
      map_entities: Number(counts.map_entities || 0),
    },
    parser: data.parser ? String(data.parser) : undefined,
    cache_path: data.cache_path ? String(data.cache_path) : undefined,
  };
};

export const saveIndexApi = {
  getStatus: () =>
    handleRequest<unknown, SaveIndexStatus>(
      () => apiClient.get('/save/index/status'),
      emptySaveIndexStatus,
      { map: mapSaveIndexStatus, quiet: true },
    ),

  rebuild: () =>
    handleRequest<unknown, { status: SaveIndexStatus }>(
      () => apiClient.post('/save/index/rebuild'),
      { status: emptySaveIndexStatus },
      { quiet: true, fallbackOnError: false },
    ),
};
