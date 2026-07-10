import { apiClient, handleRequest } from './client';
import type { Permission, Role, SessionInfo } from '../types';

const mapSession = (raw: unknown): SessionInfo => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    name: String(data.name || ''),
    role: (['admin', 'operator', 'viewer'].includes(String(data.role)) ? String(data.role) : 'viewer') as Role,
    permissions: Array.isArray(data.permissions) ? (data.permissions.map(String) as Permission[]) : [],
  };
};

export const authApi = {
  me: () =>
    handleRequest<unknown, SessionInfo>(
      () => apiClient.get('/auth/me'),
      { name: '', role: 'viewer', permissions: [] },
      { map: mapSession, quiet: true, fallbackOnError: false },
    ),
};
