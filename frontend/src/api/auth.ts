import { apiClient, handleRequest } from './client';
import type { AuthStatus, DevelopmentKey, Permission, Role, SessionInfo } from '../types';

const mapSession = (raw: unknown): SessionInfo => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    name: String(data.name || ''),
    role: (['admin', 'operator', 'viewer'].includes(String(data.role)) ? String(data.role) : 'viewer') as Role,
    permissions: Array.isArray(data.permissions) ? (data.permissions.map(String) as Permission[]) : [],
  };
};

const mapStatus = (raw: unknown): AuthStatus => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    initialized: Boolean(data.initialized),
    authenticated: Boolean(data.authenticated),
    user: data.user ? mapSession(data.user) : undefined,
  };
};

const mapDevelopmentKey = (raw: unknown): DevelopmentKey => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    name: String(data.name || ''),
    prefix: String(data.prefix || ''),
    created_at: String(data.created_at || ''),
    last_used_at: data.last_used_at ? String(data.last_used_at) : undefined,
    revoked_at: data.revoked_at ? String(data.revoked_at) : undefined,
    token: data.token ? String(data.token) : undefined,
  };
};

export const authApi = {
  status: () =>
    handleRequest<unknown, AuthStatus>(
      () => apiClient.get('/auth/status'),
      { initialized: false, authenticated: false },
      { map: mapStatus, quiet: true, fallbackOnError: false },
    ),
  register: (username: string, password: string) =>
    handleRequest<unknown, SessionInfo>(
      () => apiClient.post('/auth/register', { username, password }),
      { name: '', role: 'viewer', permissions: [] },
      { map: mapSession, quiet: true, fallbackOnError: false },
    ),
  login: (username: string, password: string) =>
    handleRequest<unknown, SessionInfo>(
      () => apiClient.post('/auth/login', { username, password }),
      { name: '', role: 'viewer', permissions: [] },
      { map: mapSession, quiet: true, fallbackOnError: false },
    ),
  logout: () =>
    handleRequest(
      () => apiClient.post('/auth/logout'),
      { logged_out: true },
      { quiet: true, fallbackOnError: false },
    ),
  me: () =>
    handleRequest<unknown, SessionInfo>(
      () => apiClient.get('/auth/me'),
      { name: '', role: 'viewer', permissions: [] },
      { map: mapSession, quiet: true, fallbackOnError: false },
    ),
  listKeys: () =>
    handleRequest<unknown, DevelopmentKey[]>(
      () => apiClient.get('/auth/api-keys'),
      [],
      { map: (raw) => Array.isArray(raw) ? raw.map(mapDevelopmentKey) : [], quiet: true, fallbackOnError: false },
    ),
  createKey: (name: string) =>
    handleRequest<unknown, DevelopmentKey>(
      () => apiClient.post('/auth/api-keys', { name }),
      { id: '', name, prefix: '', created_at: '' },
      { map: mapDevelopmentKey, quiet: true, fallbackOnError: false },
    ),
  revokeKey: (id: string) =>
    handleRequest(
      () => apiClient.delete(`/auth/api-keys/${encodeURIComponent(id)}`),
      { revoked: true },
      { quiet: true, fallbackOnError: false },
    ),
};
