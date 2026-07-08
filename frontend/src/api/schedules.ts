import { apiClient, handleRequest } from './client';
import type { Alert, Job, Schedule } from '../types';
import { mapJob } from './tasks';

const mapSchedule = (raw: unknown): Schedule => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    type: String(data.type || 'backup'),
    enabled: Boolean(data.enabled),
    interval_minutes: data.interval_minutes ? Number(data.interval_minutes) : undefined,
    time_of_day: data.time_of_day ? String(data.time_of_day) : undefined,
    waittime: data.waittime ? Number(data.waittime) : undefined,
    message: data.message ? String(data.message) : undefined,
    last_run_at: data.last_run_at ? String(data.last_run_at) : undefined,
    next_run_at: data.next_run_at ? String(data.next_run_at) : undefined,
    created_at: String(data.created_at || ''),
    updated_at: String(data.updated_at || ''),
  };
};

const mapSchedules = (raw: unknown): Schedule[] => (Array.isArray(raw) ? raw.map(mapSchedule) : []);

const mapAlert = (raw: unknown): Alert => {
  const data = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  return {
    id: String(data.id || ''),
    severity: String(data.severity || 'info'),
    title: String(data.title || ''),
    message: String(data.message || ''),
    source: String(data.source || ''),
    status: String(data.status || 'open'),
    created_at: String(data.created_at || ''),
    ack_at: data.ack_at ? String(data.ack_at) : undefined,
  };
};

const mapAlerts = (raw: unknown): Alert[] => (Array.isArray(raw) ? raw.map(mapAlert) : []);

export const schedulesApi = {
  list: () =>
    handleRequest<unknown, Schedule[]>(() => apiClient.get('/schedules'), [], {
      map: mapSchedules,
      quiet: true,
    }),

  create: (schedule: Partial<Schedule>) =>
    handleRequest<unknown, Schedule>(
      () => apiClient.post('/schedules', schedule),
      mapSchedule(schedule),
      { map: mapSchedule, quiet: true, fallbackOnError: false },
    ),

  update: (id: string, schedule: Partial<Schedule>) =>
    handleRequest<unknown, Schedule>(
      () => apiClient.put(`/schedules/${encodeURIComponent(id)}`, schedule),
      mapSchedule({ ...schedule, id }),
      { map: mapSchedule, quiet: true, fallbackOnError: false },
    ),

  delete: (id: string) =>
    handleRequest<unknown, { deleted: boolean }>(
      () => apiClient.delete(`/schedules/${encodeURIComponent(id)}`),
      { deleted: true },
      { quiet: true, fallbackOnError: false },
    ),

  run: (id: string) =>
    handleRequest<unknown, Job>(
      () => apiClient.post(`/schedules/${encodeURIComponent(id)}/run`),
      { id: '', type: 'job', status: 'waiting', progress: 0, created_at: new Date().toISOString() },
      { map: mapJob, quiet: true, fallbackOnError: false },
    ),

  alerts: (limit = 100) =>
    handleRequest<unknown, Alert[]>(() => apiClient.get(`/alerts?limit=${limit}`), [], {
      map: mapAlerts,
      quiet: true,
    }),

  ackAlert: (id: string) =>
    handleRequest<unknown, { acked: boolean }>(
      () => apiClient.post(`/alerts/${encodeURIComponent(id)}/ack`),
      { acked: true },
      { quiet: true, fallbackOnError: false },
    ),
};
