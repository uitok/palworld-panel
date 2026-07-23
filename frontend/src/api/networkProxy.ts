import { apiClient, handleRequest } from './client';
import type { NetworkProxyConfig, NetworkProxyConfigUpdate, NetworkProxyTestResult } from '../types';

const emptyEndpoint = {
  enabled: false,
  configured: false,
  url: '',
  authentication_configured: false,
  source: 'managed' as const,
  requires_restart: false as const,
  effective_for_next_task: true as const,
};

const emptyConfig: NetworkProxyConfig = {
  install: { ...emptyEndpoint },
  community: { ...emptyEndpoint },
};

const mapEndpoint = (raw: unknown): NetworkProxyConfig['install'] => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const scheme = String(data.scheme || '').toLowerCase();
  return {
    enabled: Boolean(data.enabled),
    configured: Boolean(data.configured),
    url: String(data.url || ''),
    ...(scheme === 'http' || scheme === 'https' || scheme === 'socks5' || scheme === 'socks5h' ? { scheme } : {}),
    authentication_configured: Boolean(data.authentication_configured),
    source: data.source === 'environment' ? 'environment' : 'managed',
    requires_restart: false,
    effective_for_next_task: true,
  };
};

export const mapNetworkProxyConfig = (raw: unknown): NetworkProxyConfig => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return { install: mapEndpoint(data.install), community: mapEndpoint(data.community) };
};

export const networkProxyApi = {
  getConfig: () =>
    handleRequest<unknown, NetworkProxyConfig>(
      () => apiClient.get('/settings/network-proxy'),
      emptyConfig,
      { map: mapNetworkProxyConfig, quiet: true, fallbackOnError: false },
    ),

  updateConfig: (update: NetworkProxyConfigUpdate) =>
    handleRequest<unknown, NetworkProxyConfig>(
      () => apiClient.put('/settings/network-proxy', update),
      emptyConfig,
      { map: mapNetworkProxyConfig, quiet: true, fallbackOnError: false },
    ),

  test: (scope: 'install' | 'community') =>
    handleRequest<NetworkProxyTestResult>(
      () => apiClient.post('/settings/network-proxy/test', { scope }, { timeout: 20_000 }),
      { ok: false, scope, target: '', latency_ms: 0, http_status: 0, proxy_scheme: 'http', proxy_enabled: false, message: '', host_ok: false, host_latency_ms: 0 },
      { quiet: true, fallbackOnError: false },
    ),
};
