import { apiClient, handleRequest } from './client';
import type { BackupInfo, Job } from '../types';
import { mapJob } from './tasks';

const mapBackups = (raw: unknown): BackupInfo[] => {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const data = (item && typeof item === 'object' ? item : {}) as Record<string, unknown>;
    return {
      name: String(data.name || ''),
      path: String(data.path || ''),
      size_bytes: Number(data.size_bytes || 0),
      created_at: String(data.created_at || ''),
      reason: data.reason ? String(data.reason) : undefined,
      status: data.status ? String(data.status) : undefined,
    };
  });
};

export const backupsApi = {
  list: () =>
    handleRequest<unknown, BackupInfo[]>(() => apiClient.get('/backups'), [], {
      map: mapBackups,
      quiet: true,
    }),

  create: () =>
    handleRequest<unknown, Job>(
      () => apiClient.post('/server/backup'),
      {
        id: `local_${Date.now()}`,
        type: 'backup',
        status: 'waiting',
        progress: 0,
        message: '已提交备份任务',
        created_at: new Date().toISOString(),
      },
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  restore: (name: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post(`/backups/${encodeURIComponent(name)}/restore`),
      {
        id: '',
        type: 'restore',
        status: 'waiting',
        progress: 0,
        message: '已提交恢复任务',
        created_at: new Date().toISOString(),
      },
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),
};
