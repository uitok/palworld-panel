import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, AlertCircle, Cpu, HardDrive, Network, RefreshCw, ShieldCheck } from 'lucide-react';
import { Area, AreaChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { getErrorMessage } from '../api/client';
import { monitorApi } from '../api/monitor';
import { useServerStore } from '../store/useServerStore';
import type { MonitorSample } from '../types';
import { StatCard } from '../components/ui/StatCard';
import { StatusBadge } from '../components/ui/StatusBadge';

type ChartPoint = {
  time: string;
  players: number;
  cpu: number | null;
  memory: number | null;
};

const formatBytes = (bytes: number) => {
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

const percent = (used: number, total: number) => {
  if (!total) return null;
  return Math.min(100, Math.max(0, (used / total) * 100));
};

const formatTime = (value: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
};

export const Monitor: React.FC = () => {
  const { refreshKey, autoRefresh } = useServerStore();
  const [snapshot, setSnapshot] = useState<MonitorSample | null>(null);
  const [history, setHistory] = useState<MonitorSample[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [nextSnapshot, nextHistory] = await Promise.all([monitorApi.snapshot(), monitorApi.history(120)]);
      setSnapshot(nextSnapshot.sample);
      setHistory(nextHistory);
      setError(null);
    } catch (loadError) {
      setSnapshot(null);
      setHistory([]);
      setError(getErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load, refreshKey]);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = window.setInterval(load, 15000);
    return () => window.clearInterval(interval);
  }, [autoRefresh, load]);

  const chartData = useMemo<ChartPoint[]>(
    () =>
      history.map((sample) => {
        const memoryPct = percent(sample.memory_usage_bytes, sample.memory_limit_bytes);
        return {
          time: formatTime(sample.created_at),
          players: sample.current_players,
          cpu: sample.cpu_available ? Number(sample.cpu_percent.toFixed(2)) : null,
          memory: sample.memory_available && memoryPct != null ? Number(memoryPct.toFixed(2)) : null,
        };
      }),
    [history],
  );

  const memoryPct = snapshot ? percent(snapshot.memory_usage_bytes, snapshot.memory_limit_bytes) : null;
  const diskUsedPct = snapshot
    ? percent(Math.max(0, snapshot.disk_total_bytes - snapshot.disk_free_bytes), snapshot.disk_total_bytes)
    : null;

  if (loading && !snapshot && history.length === 0) {
    return (
      <div className="flex h-full items-center justify-center p-12 text-xs font-semibold text-slate-400">
        <RefreshCw className="mr-2 animate-spin text-sky-500" size={16} />
        正在读取监控数据...
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {error && (
        <div className="rounded-2xl border border-rose-100 bg-rose-50 px-5 py-3 text-xs font-semibold text-rose-700">
          <AlertCircle className="mr-2 inline" size={14} />
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-4">
        <StatCard
          title="在线玩家"
          value={snapshot ? `${snapshot.current_players} / ${snapshot.max_players || '-'}` : '不可用'}
          icon={<Activity size={16} />}
          trend="来自监控采样"
          trendType="info"
          color="sky"
        />
        <StatCard
          title="CPU"
          value={snapshot?.cpu_available ? `${snapshot.cpu_percent.toFixed(1)}%` : '不可用'}
          icon={<Cpu size={16} />}
          trend={snapshot?.cpu_available ? '已采集' : snapshot?.unavailable_reason || '未采集'}
          trendType={snapshot?.cpu_available ? 'neutral' : 'down'}
          color="blue"
        />
        <StatCard
          title="内存"
          value={snapshot?.memory_available ? formatBytes(snapshot.memory_usage_bytes) : '不可用'}
          icon={<Network size={16} />}
          trend={memoryPct == null ? '未采集' : `${memoryPct.toFixed(1)}% of ${formatBytes(snapshot?.memory_limit_bytes || 0)}`}
          trendType={snapshot?.memory_available ? 'neutral' : 'down'}
          color="emerald"
        />
        <StatCard
          title="磁盘"
          value={snapshot?.disk_available ? formatBytes(snapshot.disk_free_bytes) : '不可用'}
          icon={<HardDrive size={16} />}
          trend={diskUsedPct == null ? '未采集' : `已用 ${diskUsedPct.toFixed(1)}%`}
          trendType={snapshot?.disk_available ? 'neutral' : 'down'}
          color="amber"
        />
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <ChartCard title="在线人数历史">
          {chartData.length > 0 ? (
            <LineChart data={chartData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
              <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} allowDecimals={false} />
              <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
              <Line type="monotone" dataKey="players" name="玩家数" stroke="#0ea5e9" strokeWidth={2.5} dot={false} />
            </LineChart>
          ) : (
            <EmptyChart />
          )}
        </ChartCard>

        <ChartCard title="CPU / 内存历史">
          {chartData.length > 0 ? (
            <AreaChart data={chartData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
              <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
              <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} domain={[0, 100]} />
              <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
              <Area type="monotone" dataKey="cpu" name="CPU (%)" stroke="#2563eb" fill="#dbeafe" strokeWidth={1.5} connectNulls />
              <Area type="monotone" dataKey="memory" name="内存 (%)" stroke="#14b8a6" fill="#ccfbf1" strokeWidth={1.5} connectNulls />
            </AreaChart>
          ) : (
            <EmptyChart />
          )}
        </ChartCard>
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="mb-4 flex items-center gap-2 border-b border-slate-100 pb-3">
          <ShieldCheck size={16} className="text-emerald-500" />
          <h3 className="text-[14px] font-bold text-slate-800">健康检查</h3>
        </div>
        <div className="grid grid-cols-1 gap-4 text-xs font-semibold text-slate-500 md:grid-cols-2 xl:grid-cols-4">
          <Health label="REST" healthy={Boolean(snapshot?.rest_healthy)} unavailableText="不可达" />
          <Health label="RCON" healthy={Boolean(snapshot?.rcon_healthy)} unavailableText="未启用或不可达" />
          <Health label="Game Port" healthy={Boolean(snapshot?.game_port_healthy)} unavailableText="不可达" />
          <Health label="Query Port" healthy={Boolean(snapshot?.query_port_healthy)} unavailableText="不可达" />
        </div>
        {snapshot?.unavailable_reason && (
          <div className="mt-4 rounded-2xl border border-amber-100 bg-amber-50 p-4 text-xs font-semibold leading-6 text-amber-800">
            {snapshot.unavailable_reason}
          </div>
        )}
        <p className="mt-4 text-[11px] font-medium text-slate-400">
          最近采样：{snapshot?.created_at ? new Date(snapshot.created_at).toLocaleString('zh-CN') : '未采集'}
        </p>
      </section>
    </div>
  );
};

const ChartCard: React.FC<{ title: string; children: React.ReactElement }> = ({ title, children }) => (
  <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
    <h3 className="mb-4 text-[14px] font-bold text-slate-800">{title}</h3>
    <div className="h-72 text-xs">
      <ResponsiveContainer width="100%" height="100%">
        {children}
      </ResponsiveContainer>
    </div>
  </section>
);

const EmptyChart = () => (
  <div className="flex h-full items-center justify-center rounded-2xl border border-dashed border-slate-200 bg-slate-50 text-xs font-semibold text-slate-400">
    暂无历史采样
  </div>
);

const Health: React.FC<{ label: string; healthy: boolean; unavailableText: string }> = ({
  label,
  healthy,
  unavailableText,
}) => (
  <div className="flex items-center justify-between rounded-2xl border border-slate-100 bg-slate-50 p-4">
    <span className="text-slate-500">{label}</span>
    <StatusBadge status={healthy ? 'success' : 'failed'} customText={healthy ? '健康' : unavailableText} />
  </div>
);
