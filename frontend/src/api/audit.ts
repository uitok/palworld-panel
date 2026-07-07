import { apiClient, handleRequest } from './client';
import type { AuditLog } from '../types';

const mapAuditLogs = (raw: unknown): AuditLog[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      id: String(data.id || ''),
      actor: String(data.actor || ''),
      role: String(data.role || ''),
      action: String(data.action || ''),
      target: data.target ? String(data.target) : undefined,
      status: String(data.status || ''),
      message: data.message ? String(data.message) : undefined,
      ip: data.ip ? String(data.ip) : undefined,
      created_at: String(data.created_at || ''),
    };
  });
};

export const auditApi = {
  list: (limit = 100) =>
    handleRequest<unknown, AuditLog[]>(() => apiClient.get(`/audit-logs?limit=${limit}`), [], {
      map: mapAuditLogs,
      quiet: true,
    }),
};
