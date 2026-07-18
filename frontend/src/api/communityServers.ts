import { apiClient, handleRequest } from './client';

export interface CommunityServer {
  id: string;
  name: string;
  address: string;
  port: number;
  connect: string;
  players: number;
  max_players: number;
  password: boolean;
  country: string;
  version?: string;
  description?: string;
  status: 'online' | 'offline' | string;
  updated_at?: string;
}

export interface CommunityServerQuery {
  region?: 'cn' | 'global';
  search?: string;
  min_players?: number;
  max_players?: number;
  password?: boolean;
  version?: string;
  status?: 'online' | 'offline' | 'all';
  page?: number;
  page_size?: number;
}

export interface CommunityServerResult {
  servers: CommunityServer[];
  total: number;
  source_total: number;
  page: number;
  page_size: number;
  source: string;
  fetched_at: string;
  stale: boolean;
  cache_age_seconds: number;
}

export interface CommunityServerSourceStatus {
  source: string;
  base_url: string;
  proxy_configured: boolean;
  reachable: boolean;
  cache_available: boolean;
  cache_fresh: boolean;
  cached_queries: number;
  last_attempt_at?: string;
  last_success_at?: string;
  last_error?: string;
  next_refresh_at?: string;
  rate_limit_per_minute: number;
}

const emptyResult: CommunityServerResult = {
  servers: [], total: 0, source_total: 0, page: 1, page_size: 30,
  source: 'battlemetrics', fetched_at: '', stale: false, cache_age_seconds: 0,
};

const toNumber = (value: unknown, fallback = 0) => {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
};

export const mapCommunityServerResult = (raw: unknown): CommunityServerResult => {
  const data = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {};
  const servers = Array.isArray(data.servers) ? data.servers.map((rawServer) => {
    const server = rawServer && typeof rawServer === 'object' ? rawServer as Record<string, unknown> : {};
    const address = String(server.address || '');
    const port = toNumber(server.port);
    return {
      id: String(server.id || `${address}:${port}`),
      name: String(server.name || '未命名服务器'),
      address,
      port,
      connect: String(server.connect || (address && port ? `${address}:${port}` : '')),
      players: toNumber(server.players),
      max_players: toNumber(server.max_players),
      password: Boolean(server.password),
      country: String(server.country || '').toUpperCase(),
      version: server.version ? String(server.version) : undefined,
      description: server.description ? String(server.description) : undefined,
      status: String(server.status || 'offline'),
      updated_at: server.updated_at ? String(server.updated_at) : undefined,
    } satisfies CommunityServer;
  }) : [];
  return {
    servers,
    total: toNumber(data.total, servers.length),
    source_total: toNumber(data.source_total, servers.length),
    page: Math.max(1, toNumber(data.page, 1)),
    page_size: Math.max(1, toNumber(data.page_size, 30)),
    source: String(data.source || 'battlemetrics'),
    fetched_at: String(data.fetched_at || ''),
    stale: Boolean(data.stale),
    cache_age_seconds: Math.max(0, toNumber(data.cache_age_seconds)),
  };
};

const queryString = (query: CommunityServerQuery) => {
  const params = new URLSearchParams();
  Object.entries(query).forEach(([key, value]) => {
    if (value !== undefined && value !== '') params.set(key, String(value));
  });
  const encoded = params.toString();
  return encoded ? `?${encoded}` : '';
};

export const communityServersApi = {
  list: (query: CommunityServerQuery = {}) => handleRequest<unknown, CommunityServerResult>(
    () => apiClient.get(`/community-servers${queryString(query)}`),
    emptyResult,
    { map: mapCommunityServerResult, quiet: true, fallbackOnError: false },
  ),
  refresh: (query: CommunityServerQuery = {}) => handleRequest<unknown, CommunityServerResult>(
    () => apiClient.post(`/community-servers/refresh${queryString(query)}`),
    emptyResult,
    { map: mapCommunityServerResult, quiet: true, fallbackOnError: false },
  ),
  sourceStatus: () => handleRequest<unknown, CommunityServerSourceStatus>(
    () => apiClient.get('/community-servers/source-status'),
    {
      source: 'battlemetrics', base_url: '', proxy_configured: false, reachable: false,
      cache_available: false, cache_fresh: false, cached_queries: 0, rate_limit_per_minute: 30,
    },
    { quiet: true, fallbackOnError: false },
  ),
};

