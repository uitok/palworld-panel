import type { MonitorSample } from '../types';

export type MonitorChartPoint = {
  time: string;
  players: number;
  cpu: number | null;
  memoryPercent: number | null;
  memoryGiB: number | null;
};

export const formatBytes = (bytes: number) => {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
};

export const bytesToGiB = (bytes: number) => bytes / 1024 / 1024 / 1024;

export const percent = (used: number, total: number) => {
  if (!total) return null;
  return Math.min(100, Math.max(0, (used / total) * 100));
};

export const formatTime = (value: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
};

export const chartTooltipFormatter = (value: unknown, name: unknown) => {
  const label = String(name);
  const numeric = Number(Array.isArray(value) ? value[0] : value);
  if (!Number.isFinite(numeric)) return [String(value), label];
  if (label.includes('GB')) return [`${numeric.toFixed(2)} GB`, label];
  if (label.includes('%')) return [`${numeric.toFixed(1)}%`, label];
  return [numeric, label];
};

export const toMonitorChartPoints = (samples: readonly MonitorSample[]): MonitorChartPoint[] =>
  samples.map((sample) => {
    const memoryPct = percent(sample.memory_usage_bytes, sample.memory_limit_bytes);
    return {
      time: formatTime(sample.created_at),
      players: sample.current_players,
      cpu: sample.cpu_available ? Number(sample.cpu_percent.toFixed(2)) : null,
      memoryPercent: sample.memory_available && memoryPct != null ? Number(memoryPct.toFixed(2)) : null,
      memoryGiB: sample.memory_available ? Number(bytesToGiB(sample.memory_usage_bytes).toFixed(2)) : null,
    };
  });
