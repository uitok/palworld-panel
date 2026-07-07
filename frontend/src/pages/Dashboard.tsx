import React, { useCallback, useEffect, useState } from 'react';
import { Activity, Bell, Clock, Cpu, Home, Play, RefreshCw, Square, Sword, Terminal, Users } from 'lucide-react';
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
import { emptyLogs, serverApi } from '../api/server';
import { useServerStore } from '../store/useServerStore';
import { StatCard } from '../components/ui/StatCard';
import { StatusBadge } from '../components/ui/StatusBadge';

const playerHistoryData = [
  { time: '04:00', players: 1 },
  { time: '05:00', players: 2 },
  { time: '06:00', players: 2 },
  { time: '07:00', players: 4 },
  { time: '08:00', players: 5 },
  { time: '09:00', players: 6 },
  { time: '10:00', players: 5 },
];

const performanceData = [
  { time: '10:10', cpu: 12 },
  { time: '10:15', cpu: 18 },
  { time: '10:20', cpu: 25 },
  { time: '10:25', cpu: 15 },
  { time: '10:30', cpu: 22 },
  { time: '10:35', cpu: 45 },
  { time: '10:40', cpu: 28 },
  { time: '10:45', cpu: 24 },
];

const stoppedMetrics = {
  server_fps: 0,
  current_players: 0,
  max_players: 32,
  uptime: 0,
  total_pals: 0,
  active_bases: 0,
  frame_time: 0,
};

export const Dashboard: React.FC = () => {
  const { status, setStatus, metrics, setMetrics, autoRefresh, refreshKey, triggerRefresh } = useServerStore();
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    const statusRes = await serverApi.getStatus();
    setStatus(statusRes);

    if (statusRes.status !== 'running') {
      setMetrics(stoppedMetrics);
      setLogs(emptyLogs);
      setLoading(false);
      return;
    }

    const [metricsRes, logsRes] = await Promise.all([serverApi.getMetrics(), serverApi.getLogs(80)]);
    setMetrics(metricsRes);
    setLogs(logsRes.logs);
    setLoading(false);
  }, [setMetrics, setStatus]);

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshKey]);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, [autoRefresh, fetchData]);

  const control = async (action: 'start' | 'stop') => {
    setLoading(true);
    if (action === 'start') await serverApi.start();
    if (action === 'stop') await serverApi.stop();
    setNotice(action === 'start' ? '启动请求已发送' : '停止请求已发送');
    triggerRefresh();
  };

  const formatRAM = (bytes?: number) => {
    if (!bytes) return '0 GB';
    return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
  };

  const formatUptime = (seconds?: number) => {
    if (!seconds) return '离线';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${days}天 ${hours}时 ${minutes}分`;
  };

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
      {notice && (
        <div className="rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3 text-xs font-semibold text-sky-700">
          {notice}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="在线玩家" value={`${metrics?.current_players || 0} / ${metrics?.max_players || 32}`} icon={<Users size={16} />} trend="来自官方 REST API" trendType="info" color="sky" />
        <StatCard title="服务器状态" value={status?.status === 'running' ? '运行中' : '已停止'} icon={<Activity size={16} />} trend={status?.setup_step || 'prerequisites'} trendType={status?.status === 'running' ? 'up' : 'down'} color={status?.status === 'running' ? 'emerald' : 'rose'} />
        <StatCard title="系统占用" value={`${status?.cpu_percent?.toFixed(1) || 0}% CPU`} icon={<Cpu size={16} />} trend={`内存 ${formatRAM(status?.memory_usage_bytes)}`} trendType="neutral" color="blue" />
        <StatCard title="世界运行时间" value={formatUptime(metrics?.uptime)} icon={<Clock size={16} />} trend={status?.pending_restart ? '配置等待重启' : '配置已生效'} trendType={status?.pending_restart ? 'down' : 'up'} color="emerald" />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="服务端端口" value={status?.ports?.game || 8211} icon={<Bell size={16} />} trend={`REST ${status?.ports?.rest || 8212}`} trendType="neutral" color="sky" />
        <StatCard title="帕鲁总数" value={metrics?.total_pals || 0} icon={<Sword size={16} />} trend="未启动时显示 0" trendType="neutral" color="blue" />
        <StatCard title="活跃基地" value={metrics?.active_bases || 0} icon={<Home size={16} />} trend="来自 metrics" trendType="neutral" color="amber" />
        <StatCard title="Server FPS" value={metrics?.server_fps || 0} icon={<Activity size={16} />} trend={`${metrics?.frame_time || 0} ms/frame`} trendType="info" color="emerald" />
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] xl:col-span-2">
          <div className="mb-5 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <h3 className="text-[14px] font-bold text-slate-800">在线人数与系统负载</h3>
            <span className="text-[11px] font-semibold text-slate-400">5 秒自动刷新</span>
          </div>
          <div className="grid h-[420px] grid-cols-1 gap-6 lg:h-64 lg:grid-cols-2">
            <div className="flex min-h-0 flex-col gap-2">
              <span className="text-[11px] font-bold text-slate-400">在线趋势</span>
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={playerHistoryData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                  <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
                  <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} />
                  <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
                  <Line type="monotone" dataKey="players" name="玩家数" stroke="#0ea5e9" strokeWidth={2.5} dot={{ r: 3, fill: '#0ea5e9' }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
            <div className="flex min-h-0 flex-col gap-2">
              <span className="text-[11px] font-bold text-slate-400">CPU 波动</span>
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={performanceData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                  <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
                  <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} />
                  <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
                  <Area type="monotone" dataKey="cpu" name="CPU (%)" stroke="#14b8a6" fill="#ccfbf1" strokeWidth={1.5} />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          </div>
        </section>

        <section className="flex flex-col justify-between rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <div>
            <h3 className="text-[14px] font-bold text-slate-800">进程控制</h3>
            <p className="mt-3 text-xs font-medium leading-relaxed text-slate-400">
              控制后端托管的 Palworld 服务端进程。配置和 Mod 变更后，请重启服务器使其生效。
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
          <div className="mt-5 grid grid-cols-2 gap-3">
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
            onClick={fetchData}
            className="flex items-center gap-1.5 rounded-lg border border-slate-200/60 bg-slate-50 px-3 py-1 text-[10px] font-semibold text-slate-500 hover:bg-slate-100"
          >
            <RefreshCw size={10} />
            刷新
          </button>
        </div>
        <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-2xl border border-slate-800 bg-slate-950 p-4 font-mono text-[11px] leading-relaxed text-emerald-300">
          {logs || '暂无日志'}
        </pre>
      </section>
    </div>
  );
};
