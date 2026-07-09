import React, { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, Bell, Clock, Cpu, Home, Play, RefreshCw, Square, Sword, Terminal, Users, Zap } from 'lucide-react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { getErrorMessage } from '../api/client';
import { monitorApi } from '../api/monitor';
import { emptyLogs, serverApi } from '../api/server';
import { useServerStore } from '../store/useServerStore';
import { StatCard } from '../components/ui/StatCard';
import { StatusBadge } from '../components/ui/StatusBadge';
import { useDebouncedValue } from '../hooks/useDebouncedValue';

type ChartPoint = {
  time: string;
  players: number;
  cpu: number | null;
  memoryPercent: number | null;
  memoryGiB: number | null;
};

const stoppedMetrics = {
  server_fps: 0,
  current_players: 0,
  max_players: 32,
  uptime: 0,
  total_pals: 0,
  active_bases: 0,
  frame_time: 0,
};

const formatTime = (value: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
};

const bytesToGiB = (bytes: number) => bytes / 1024 / 1024 / 1024;

const formatRAM = (bytes?: number) => {
  if (!bytes) return '0 GB';
  return `${bytesToGiB(bytes).toFixed(1)} GB`;
};

const percent = (used: number, total: number) => {
  if (!total) return null;
  return Math.min(100, Math.max(0, (used / total) * 100));
};

const chartTooltipFormatter = (value: unknown, name: unknown) => {
  const label = String(name);
  const numeric = Number(Array.isArray(value) ? value[0] : value);
  if (!Number.isFinite(numeric)) return [String(value), label];
  if (label.includes('GB')) return [`${numeric.toFixed(2)} GB`, label];
  if (label.includes('%')) return [`${numeric.toFixed(1)}%`, label];
  return [numeric, label];
};

export const Dashboard: React.FC = () => {
  const { status: cachedStatus, setStatus, metrics: cachedMetrics, setMetrics, autoRefresh, refreshKey, triggerRefresh } = useServerStore();
  const [logSearch, setLogSearch] = useState('');
  const [logLevel, setLogLevel] = useState('');
  const [notice, setNotice] = useState<string | null>(null);
  const debouncedLogSearch = useDebouncedValue(logSearch, 300);

  const statusQuery = useQuery({
    queryKey: ['dashboard-status', refreshKey],
    queryFn: serverApi.getStatus,
    refetchInterval: autoRefresh ? 5000 : false,
    placeholderData: (previous) => previous,
  });

  const status = statusQuery.data ?? cachedStatus;
  const metricsQuery = useQuery({
    queryKey: ['dashboard-metrics', refreshKey],
    queryFn: serverApi.getMetrics,
    enabled: status?.status === 'running',
    refetchInterval: autoRefresh && status?.status === 'running' ? 5000 : false,
    placeholderData: (previous) => previous,
  });

  const historyQuery = useQuery({
    queryKey: ['dashboard-history', 48, refreshKey],
    queryFn: () => monitorApi.history(48),
    placeholderData: (previous) => previous,
  });

  const logsQuery = useQuery({
    queryKey: ['dashboard-logs', 80, debouncedLogSearch, logLevel, refreshKey],
    queryFn: () => serverApi.getLogs(80, debouncedLogSearch, logLevel),
    enabled: status?.status === 'running',
    placeholderData: (previous) => previous,
  });

  useEffect(() => {
    if (!statusQuery.data) return;
    setStatus(statusQuery.data);
    if (statusQuery.data.status !== 'running') {
      setMetrics(stoppedMetrics);
    }
  }, [setMetrics, setStatus, statusQuery.data]);

  useEffect(() => {
    if (metricsQuery.data) setMetrics(metricsQuery.data);
  }, [metricsQuery.data, setMetrics]);

  const chartData = useMemo<ChartPoint[]>(() => {
    return (historyQuery.data ?? []).map((sample) => {
      const memoryPct = percent(sample.memory_usage_bytes, sample.memory_limit_bytes);
      return {
        time: formatTime(sample.created_at),
        players: sample.current_players,
        cpu: sample.cpu_available ? Number(sample.cpu_percent.toFixed(2)) : null,
        memoryPercent: sample.memory_available && memoryPct != null ? Number(memoryPct.toFixed(2)) : null,
        memoryGiB: sample.memory_available ? Number(bytesToGiB(sample.memory_usage_bytes).toFixed(2)) : null,
      };
    });
  }, [historyQuery.data]);

  const metrics = status?.status === 'running' ? (metricsQuery.data ?? cachedMetrics) : stoppedMetrics;
  const logs = status?.status === 'running' ? (logsQuery.data?.logs ?? '') : emptyLogs;
  const loading = statusQuery.isLoading && !status;
  const queryNotice =
    notice ||
    (statusQuery.error
      ? getErrorMessage(statusQuery.error)
      : metricsQuery.error
        ? getErrorMessage(metricsQuery.error)
        : logsQuery.error
          ? getErrorMessage(logsQuery.error)
          : null);

  const refreshDashboard = async () => {
    await Promise.all([statusQuery.refetch(), metricsQuery.refetch(), historyQuery.refetch(), logsQuery.refetch()]);
  };

  const control = async (action: 'start' | 'stop' | 'forceStop') => {
    const text =
      action === 'start'
        ? '启动服务器？'
        : action === 'forceStop'
          ? '通过官方 REST 强制停止服务器？REST 不可达时会返回失败。'
          : '停止服务器？请确认在线玩家已经收到通知。';
    if (!window.confirm(text)) return;
    try {
      if (action === 'start') await serverApi.start();
      if (action === 'stop') await serverApi.stop();
      if (action === 'forceStop') await serverApi.forceStop();
      setNotice(action === 'start' ? '启动请求已发送' : action === 'forceStop' ? '强制停止请求已发送' : '停止请求已发送');
      triggerRefresh();
    } catch (error) {
      setNotice(getErrorMessage(error));
    }
  };

  const formatUptime = (seconds?: number) => {
    if (!seconds) return '离线';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${days}天 ${hours}时 ${minutes}分`;
  };

  const latestChart = useMemo(() => chartData[chartData.length - 1], [chartData]);
  const hasMemoryPercent = chartData.some((point) => point.memoryPercent != null);
  const hasMemoryUsage = chartData.some((point) => point.memoryGiB != null);
  const latestMemoryText =
    latestChart?.memoryPercent != null
      ? `${latestChart.memoryPercent.toFixed(1)}% 内存`
      : latestChart?.memoryGiB != null
        ? `${latestChart.memoryGiB.toFixed(1)} GB 内存`
        : `内存 ${formatRAM(status?.memory_usage_bytes)}`;

  if (loading && !status) {
    return (
      <div className="flex h-full items-center justify-center p-12 text-xs font-semibold text-slate-400">
        <RefreshCw className="mr-2 animate-spin text-sky-500" size={16} />
        正在加载仪表盘...
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {queryNotice && (
        <div className="rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3 text-xs font-semibold text-sky-700">
          {queryNotice}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="在线玩家" value={`${metrics?.current_players || 0} / ${metrics?.max_players || 32}`} icon={<Users size={16} />} trend="来自官方 REST / 监控采样" trendType="info" color="sky" />
        <StatCard title="服务器状态" value={status?.status === 'running' ? '运行中' : '已停止'} icon={<Activity size={16} />} trend={status?.setup_step || 'prerequisites'} trendType={status?.status === 'running' ? 'up' : 'down'} color={status?.status === 'running' ? 'emerald' : 'rose'} />
        <StatCard title="系统占用" value={`${latestChart?.cpu ?? status?.cpu_percent?.toFixed(1) ?? 0}% CPU`} icon={<Cpu size={16} />} trend={latestMemoryText} trendType="neutral" color="blue" />
        <StatCard title="世界运行时间" value={formatUptime(metrics?.uptime)} icon={<Clock size={16} />} trend={status?.pending_restart ? '配置等待重启' : '配置已生效'} trendType={status?.pending_restart ? 'down' : 'up'} color="emerald" />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="服务端口" value={status?.ports?.game || 8211} icon={<Bell size={16} />} trend={`REST ${status?.ports?.rest || 8212}`} trendType="neutral" color="sky" />
        <StatCard title="帕鲁总数" value={metrics?.total_pals || 0} icon={<Sword size={16} />} trend="后端未提供时显示 0" trendType="neutral" color="blue" />
        <StatCard title="活跃基地" value={metrics?.active_bases || 0} icon={<Home size={16} />} trend="来自 metrics" trendType="neutral" color="amber" />
        <StatCard title="Server FPS" value={metrics?.server_fps || 0} icon={<Activity size={16} />} trend={`${metrics?.frame_time || 0} ms/frame`} trendType="info" color="emerald" />
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] xl:col-span-2">
          <div className="mb-5 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <h3 className="text-[14px] font-bold text-slate-800">在线人数与系统负载</h3>
            <span className="text-[11px] font-semibold text-slate-400">来自监控历史入库</span>
          </div>
          <div className="grid h-[420px] grid-cols-1 gap-6 lg:h-64 lg:grid-cols-2">
            <div className="flex min-h-0 flex-col gap-2">
              <span className="text-[11px] font-bold text-slate-400">在线趋势</span>
              {chartData.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={chartData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                    <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
                    <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} allowDecimals={false} />
                    <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
                    <Line type="monotone" dataKey="players" name="玩家数" stroke="#0ea5e9" strokeWidth={2.5} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <EmptyChart />
              )}
            </div>
            <div className="flex min-h-0 flex-col gap-2">
              <span className="text-[11px] font-bold text-slate-400">CPU / 内存波动</span>
              {chartData.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={chartData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                    <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
                    <YAxis yAxisId="percent" stroke="#94a3b8" fontSize={10} tickLine={false} domain={[0, 100]} tickFormatter={(value) => `${value}%`} />
                    {!hasMemoryPercent && hasMemoryUsage && (
                      <YAxis yAxisId="memory" orientation="right" stroke="#94a3b8" fontSize={10} tickLine={false} width={34} tickFormatter={(value) => `${value}G`} />
                    )}
                    <Tooltip formatter={chartTooltipFormatter} contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
                    <Area yAxisId="percent" type="monotone" dataKey="cpu" name="CPU (%)" stroke="#2563eb" fill="#dbeafe" strokeWidth={1.5} connectNulls />
                    {hasMemoryPercent ? (
                      <Area yAxisId="percent" type="monotone" dataKey="memoryPercent" name="内存 (%)" stroke="#14b8a6" fill="#ccfbf1" strokeWidth={1.5} connectNulls />
                    ) : hasMemoryUsage ? (
                      <Area yAxisId="memory" type="monotone" dataKey="memoryGiB" name="内存用量 (GB)" stroke="#14b8a6" fill="#ccfbf1" strokeWidth={1.5} connectNulls />
                    ) : null}
                  </AreaChart>
                </ResponsiveContainer>
              ) : (
                <EmptyChart />
              )}
            </div>
          </div>
        </section>

        <section className="flex flex-col justify-between rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <div>
            <h3 className="text-[14px] font-bold text-slate-800">进程控制</h3>
            <p className="mt-3 text-xs font-medium leading-relaxed text-slate-400">
              普通停止由面板托管进程，强制停止会调用 Palworld 官方 REST `/stop`。
            </p>
            <div className="mt-5 rounded-2xl border border-slate-100 bg-slate-50 p-4">
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs font-semibold text-slate-600">当前状态</span>
                <StatusBadge status={status?.status || 'stopped'} />
              </div>
              <p className="mt-3 break-all font-mono text-[10px] text-slate-400">
                {status?.paths?.palworld_settings || '配置路径未初始化'}
              </p>
            </div>
          </div>
          <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <button
              type="button"
              onClick={() => control('start')}
              disabled={status?.status === 'running' || status?.status === 'starting'}
              className="flex items-center justify-center gap-2 rounded-2xl bg-emerald-500 py-3 text-xs font-semibold text-white transition-all hover:bg-emerald-600 disabled:opacity-40"
            >
              <Play size={14} />
              启动
            </button>
            <button
              type="button"
              onClick={() => control('stop')}
              disabled={status?.status === 'stopped' || status?.status === 'stopping'}
              className="flex items-center justify-center gap-2 rounded-2xl bg-rose-500 py-3 text-xs font-semibold text-white transition-all hover:bg-rose-600 disabled:opacity-40"
            >
              <Square size={14} />
              停止
            </button>
            <button
              type="button"
              onClick={() => control('forceStop')}
              disabled={status?.status === 'stopped' || status?.status === 'stopping'}
              className="flex items-center justify-center gap-2 rounded-2xl border border-amber-200 bg-amber-50 py-3 text-xs font-semibold text-amber-700 transition-all hover:bg-amber-100 disabled:opacity-40"
            >
              <Zap size={14} />
              强停
            </button>
          </div>
        </section>
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <div className="rounded-lg bg-slate-100 p-1.5 text-slate-500">
              <Terminal size={15} />
            </div>
            <h3 className="text-[14px] font-bold text-slate-800">实时日志</h3>
          </div>
          <button
            type="button"
            onClick={() => void refreshDashboard()}
            className="flex items-center gap-1.5 rounded-lg border border-slate-200/60 bg-slate-50 px-3 py-1 text-[10px] font-semibold text-slate-500 hover:bg-slate-100"
          >
            <RefreshCw size={10} />
            刷新
          </button>
        </div>
        <div className="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-[1fr_160px]">
          <input
            type="search"
            value={logSearch}
            onChange={(event) => setLogSearch(event.target.value)}
            placeholder="搜索日志关键字"
            className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
          />
          <select
            value={logLevel}
            onChange={(event) => setLogLevel(event.target.value)}
            className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
          >
            <option value="">全部级别</option>
            <option value="error">Error</option>
            <option value="warn">Warn</option>
            <option value="info">Info</option>
          </select>
        </div>
        <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-2xl border border-slate-800 bg-slate-950 p-4 font-mono text-[11px] leading-relaxed text-emerald-300">
          {logs || (queryNotice ? '后端不可用或接口未实现' : '暂无日志')}
        </pre>
      </section>
    </div>
  );
};

const EmptyChart = () => (
  <div className="flex h-full items-center justify-center rounded-2xl border border-dashed border-slate-200 bg-slate-50 text-xs font-semibold text-slate-400">
    暂无历史采样
  </div>
);
