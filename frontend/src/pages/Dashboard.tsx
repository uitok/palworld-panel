import React, { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, AlertTriangle, Bell, Clock, Cpu, Home, Play, RefreshCw, Square, Sword, Terminal, Trash2, Users, X, Zap } from 'lucide-react';
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
import { serverApi } from '../api/server';
import { tasksApi } from '../api/tasks';
import { useServerStore } from '../store/useServerStore';
import { StatCard } from '../components/ui/StatCard';
import { StatusBadge } from '../components/ui/StatusBadge';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import type { Job } from '../types';
import { bytesToGiB, chartTooltipFormatter, toMonitorChartPoints } from '../utils/monitor';

const stoppedMetrics = {
  server_fps: 0,
  current_players: 0,
  max_players: 32,
  uptime: 0,
  total_pals: 0,
  active_bases: 0,
  frame_time: 0,
};

const formatRAM = (bytes?: number) => {
  if (!bytes) return '0 GB';
  return `${bytesToGiB(bytes).toFixed(1)} GB`;
};

export const Dashboard: React.FC = () => {
  const { status: cachedStatus, setStatus, metrics: cachedMetrics, setMetrics, autoRefresh, refreshKey, triggerRefresh, session } = useServerStore();
  const [logSearch, setLogSearch] = useState('');
  const [logLevel, setLogLevel] = useState('');
  const [notice, setNotice] = useState<string | null>(null);
  const [resetOpen, setResetOpen] = useState(false);
  const [resetConfirmation, setResetConfirmation] = useState('');
  const [resetJob, setResetJob] = useState<Job | null>(null);
  const [resetSubmitting, setResetSubmitting] = useState(false);
  const debouncedLogSearch = useDebouncedValue(logSearch, 300);
  const canResetWorld = Boolean(session?.permissions.includes('world:reset'));

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
    queryKey: ['dashboard-logs', 80, debouncedLogSearch, logLevel, status?.status, refreshKey],
    queryFn: () => serverApi.getLogs(80, debouncedLogSearch, logLevel),
    enabled: Boolean(status),
    refetchInterval: autoRefresh && status?.status === 'running' ? 3000 : false,
    placeholderData: (previous) => previous,
  });

  const worldQuery = useQuery({
    queryKey: ['dashboard-world', status?.status, refreshKey],
    queryFn: serverApi.getWorld,
    enabled: canResetWorld,
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

  const chartData = useMemo(() => toMonitorChartPoints(historyQuery.data ?? []), [historyQuery.data]);

  const metrics = status?.status === 'running' ? (metricsQuery.data ?? cachedMetrics) : stoppedMetrics;
  const logResponse = logsQuery.data;
  const logs = logResponse?.logs ?? '';
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
    await Promise.all([
      statusQuery.refetch(),
      metricsQuery.refetch(),
      historyQuery.refetch(),
      logsQuery.refetch(),
      ...(canResetWorld ? [worldQuery.refetch()] : []),
    ]);
  };

  const control = async (action: 'start' | 'stop' | 'forceStop') => {
    const text =
      action === 'start'
        ? '启动服务器？'
        : action === 'forceStop'
          ? '通过官方 REST 强制停止服务器？REST 不可达时会返回失败。'
          : '安全停止服务器？面板会先保存世界、广播 60 秒倒计时，再停止服务。';
    if (!window.confirm(text)) return;
    try {
      if (action === 'start') await serverApi.start();
      if (action === 'stop') await serverApi.safeStop(60, '服务器将在 60 秒后安全关闭');
      if (action === 'forceStop') await serverApi.forceStop();
      setNotice(action === 'start' ? '启动请求已发送' : action === 'forceStop' ? '强制停止请求已发送' : '安全关服任务已提交，可在任务队列查看进度');
      triggerRefresh();
    } catch (error) {
      setNotice(getErrorMessage(error));
    }
  };

  const openWorldReset = async () => {
    setResetConfirmation('');
    setResetJob(null);
    setResetOpen(true);
    await worldQuery.refetch();
  };

  const resetWorld = async () => {
    const world = worldQuery.data;
    if (!world?.active_world_id || resetConfirmation !== 'RESET WORLD') return;
    setResetSubmitting(true);
    try {
      const job = await serverApi.resetWorld(world.active_world_id, resetConfirmation);
      setResetJob(job);
      const done = await tasksApi.waitForJob(job.id, setResetJob);
      setNotice(done.status === 'success' ? '世界重置已完成，验证备份已保留' : done.error || '世界重置失败');
      await Promise.all([statusQuery.refetch(), worldQuery.refetch(), logsQuery.refetch()]);
      triggerRefresh();
    } catch (error) {
      setNotice(getErrorMessage(error));
    } finally {
      setResetSubmitting(false);
    }
  };

  const logEmptyText = (() => {
    if (logs) return logs;
    switch (logResponse?.reason) {
      case 'waiting_for_output':
        return '等待服务输出...';
      case 'not_started':
        return '服务尚未启动，暂无历史日志。';
      case 'no_available_output':
        return '当前采集源没有可用输出。';
      case 'no_collection_source':
        return '没有可用的日志采集源。';
      default:
        return '暂无日志。';
    }
  })();

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
  const serverFacts = [
    { label: '游戏端口', value: status?.ports?.game || 8211, detail: `REST ${status?.ports?.rest || 8212}`, icon: <Bell size={16} />, tone: 'text-sky-700 bg-sky-50' },
    { label: '帕鲁总数', value: metrics?.total_pals || 0, detail: '来自存档与监控数据', icon: <Sword size={16} />, tone: 'text-blue-700 bg-blue-50' },
    { label: '活跃基地', value: metrics?.active_bases || 0, detail: '当前世界统计', icon: <Home size={16} />, tone: 'text-amber-700 bg-amber-50' },
    { label: 'Server FPS', value: metrics?.server_fps || 0, detail: `${metrics?.frame_time || 0} ms / frame`, icon: <Activity size={16} />, tone: 'text-emerald-700 bg-emerald-50' },
  ];

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
        <div className="rounded-xl border border-sky-200/80 bg-sky-50 px-4 py-3 text-sm font-semibold text-sky-800">
          {queryNotice}
        </div>
      )}

      <section className="pp-hero">
        <div className="pp-hero__state">
          <span className={`pp-pulse ${status?.status === 'running' ? '' : 'is-down'}`} />
          <span>
            <strong className="pp-hero__name">Palworld Dedicated Server</strong>
            <span className="pp-hero__addr">0.0.0.0:{status?.ports?.game || 8211} · REST {status?.ports?.rest || 8212}</span>
          </span>
        </div>
        <span className="pp-hero__divider" />
        <span className="pp-hero__item">运行时间<strong>{formatUptime(metrics?.uptime)}</strong></span>
        <span className="pp-hero__item">在线玩家<strong>{metrics?.current_players || 0} / {metrics?.max_players || 32}</strong></span>
        <span className="pp-hero__item">Server FPS<strong>{metrics?.server_fps || 0}</strong></span>
        <div className="pp-hero__actions">
          <button type="button" onClick={() => void control('start')} disabled={status?.status === 'running'} className="pp-btn pp-btn--ghost pp-btn--sm"><Play size={13} />启动</button>
          <button type="button" onClick={() => void control('stop')} disabled={status?.status !== 'running'} className="pp-btn pp-btn--danger-ghost pp-btn--sm"><Square size={13} />停止</button>
        </div>
      </section>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="在线玩家" value={`${metrics?.current_players || 0} / ${metrics?.max_players || 32}`} icon={<Users size={16} />} trend="来自官方 REST / 监控采样" trendType="info" color="sky" />
        <StatCard title="服务器状态" value={status?.status === 'running' ? '运行中' : '已停止'} icon={<Activity size={16} />} trend={status?.setup_step || 'prerequisites'} trendType={status?.status === 'running' ? 'up' : 'down'} color={status?.status === 'running' ? 'emerald' : 'rose'} />
        <StatCard title="系统占用" value={`${latestChart?.cpu ?? status?.cpu_percent?.toFixed(1) ?? 0}% CPU`} icon={<Cpu size={16} />} trend={latestMemoryText} trendType="neutral" color="blue" />
        <StatCard title="世界运行时间" value={formatUptime(metrics?.uptime)} icon={<Clock size={16} />} trend={status?.pending_restart ? '配置等待重启' : '配置已生效'} trendType={status?.pending_restart ? 'down' : 'up'} color="emerald" />
      </div>

      <section className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl border border-slate-200 bg-slate-200 lg:grid-cols-4">
        {serverFacts.map((fact) => (
          <div key={fact.label} className="flex min-w-0 items-center gap-3 bg-white px-4 py-4 sm:px-5">
            <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${fact.tone}`}>{fact.icon}</span>
            <span className="min-w-0">
              <span className="block truncate text-xs font-semibold text-slate-500">{fact.label}</span>
              <strong className="mt-0.5 block truncate text-lg font-bold tracking-tight text-slate-900">{fact.value}</strong>
              <span className="hidden truncate text-[11px] font-medium text-slate-400 sm:block">{fact.detail}</span>
            </span>
          </div>
        ))}
      </section>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <section className="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-sm sm:p-6 xl:col-span-2">
          <div className="mb-5 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <h3 className="text-base font-bold tracking-tight text-slate-900">在线人数与系统负载</h3>
            <span className="text-xs font-semibold text-slate-500">来自监控历史入库</span>
          </div>
          <div className="grid h-[420px] grid-cols-1 gap-6 lg:h-64 lg:grid-cols-2">
            <div className="flex min-h-0 flex-col gap-2">
              <span className="text-xs font-bold text-slate-500">在线趋势</span>
              {chartData.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={chartData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#edf1f6" />
                    <XAxis dataKey="time" stroke="#8997aa" fontSize={10} tickLine={false} axisLine={false} />
                    <YAxis stroke="#8997aa" fontSize={10} tickLine={false} axisLine={false} allowDecimals={false} />
                    <Tooltip contentStyle={{ fontSize: '12px', borderRadius: '10px', border: '1px solid #dce3ec', boxShadow: '0 12px 30px -18px rgba(8,17,31,.35)' }} />
                    <Line type="monotone" dataKey="players" name="玩家数" stroke="#356a9a" strokeWidth={2.5} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <EmptyChart />
              )}
            </div>
            <div className="flex min-h-0 flex-col gap-2">
              <span className="text-xs font-bold text-slate-500">CPU / 内存波动</span>
              {chartData.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={chartData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#edf1f6" />
                    <XAxis dataKey="time" stroke="#8997aa" fontSize={10} tickLine={false} axisLine={false} />
                    <YAxis yAxisId="percent" stroke="#8997aa" fontSize={10} tickLine={false} axisLine={false} domain={[0, 100]} tickFormatter={(value) => `${value}%`} />
                    {!hasMemoryPercent && hasMemoryUsage && (
                      <YAxis yAxisId="memory" orientation="right" stroke="#8997aa" fontSize={10} tickLine={false} axisLine={false} width={34} tickFormatter={(value) => `${value}G`} />
                    )}
                    <Tooltip formatter={chartTooltipFormatter} contentStyle={{ fontSize: '12px', borderRadius: '10px', border: '1px solid #dce3ec', boxShadow: '0 12px 30px -18px rgba(8,17,31,.35)' }} />
                    <Area yAxisId="percent" type="monotone" dataKey="cpu" name="CPU (%)" stroke="#4f7cff" fill="#dfe9ff" strokeWidth={1.5} connectNulls />
                    {hasMemoryPercent ? (
                      <Area yAxisId="percent" type="monotone" dataKey="memoryPercent" name="内存 (%)" stroke="#4c7ea8" fill="#e7eef4" strokeWidth={1.5} connectNulls />
                    ) : hasMemoryUsage ? (
                      <Area yAxisId="memory" type="monotone" dataKey="memoryGiB" name="内存用量 (GB)" stroke="#4c7ea8" fill="#e7eef4" strokeWidth={1.5} connectNulls />
                    ) : null}
                  </AreaChart>
                </ResponsiveContainer>
              ) : (
                <EmptyChart />
              )}
            </div>
          </div>
        </section>

        <section className="flex flex-col justify-between rounded-2xl border border-slate-200/80 bg-white p-5 shadow-sm sm:p-6">
          <div>
            <h3 className="text-base font-bold tracking-tight text-slate-900">进程控制</h3>
            <p className="mt-2 text-sm font-medium leading-6 text-slate-500">
              普通停止由面板托管进程，强制停止会调用 Palworld 官方 REST `/stop`。
            </p>
            <div className="mt-5 rounded-xl border border-slate-200/80 bg-slate-50 p-4">
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
              className="flex h-10 items-center justify-center gap-2 rounded-xl bg-emerald-600 px-3 text-xs font-bold text-white transition-colors hover:bg-emerald-700 disabled:opacity-40"
            >
              <Play size={14} />
              启动
            </button>
            <button
              type="button"
              onClick={() => control('stop')}
              disabled={status?.status === 'stopped' || status?.status === 'stopping'}
              className="flex h-10 items-center justify-center gap-2 rounded-xl bg-rose-600 px-3 text-xs font-bold text-white transition-colors hover:bg-rose-700 disabled:opacity-40"
            >
              <Square size={14} />
              停止
            </button>
            <button
              type="button"
              onClick={() => control('forceStop')}
              disabled={status?.status === 'stopped' || status?.status === 'stopping'}
              className="flex h-10 items-center justify-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 text-xs font-bold text-amber-800 transition-colors hover:bg-amber-100 disabled:opacity-40"
            >
              <Zap size={14} />
              强停
            </button>
          </div>
          {canResetWorld && (
            <button
              type="button"
              onClick={() => void openWorldReset()}
              className="mt-3 flex h-10 w-full items-center justify-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-3 text-xs font-bold text-rose-700 transition-colors hover:bg-rose-100"
            >
              <Trash2 size={14} />
              重置世界
            </button>
          )}
        </section>
      </div>

      <section className="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-sm sm:p-6">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <div className="rounded-lg bg-slate-100 p-2 text-slate-600">
              <Terminal size={15} />
            </div>
            <h3 className="text-base font-bold tracking-tight text-slate-900">实时日志</h3>
            {logResponse && (
              <span className="truncate rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-semibold text-slate-500">
                {logResponse.source === 'file' ? '持久日志' : logResponse.source === 'docker' ? 'Docker 输出' : '无采集源'}
              </span>
            )}
          </div>
          <button
            type="button"
            onClick={() => void refreshDashboard()}
            className="flex h-9 items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 text-xs font-semibold text-slate-600 hover:bg-slate-50"
          >
            <RefreshCw size={13} />
            刷新
          </button>
        </div>
        <div className="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-[1fr_160px]">
          <input
            type="search"
            value={logSearch}
            onChange={(event) => setLogSearch(event.target.value)}
            placeholder="搜索日志关键字"
            className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 shadow-sm focus:border-sky-500 focus:outline-none"
          />
          <select
            value={logLevel}
            onChange={(event) => setLogLevel(event.target.value)}
            className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 shadow-sm focus:border-sky-500 focus:outline-none"
          >
            <option value="">全部级别</option>
            <option value="error">Error</option>
            <option value="warn">Warn</option>
            <option value="info">Info</option>
          </select>
        </div>
        <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-xl border border-slate-800 bg-slate-950 p-4 font-mono text-[12px] leading-6 text-emerald-300 shadow-inner">
          {logEmptyText}
        </pre>
      </section>

      {resetOpen && canResetWorld && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 p-4" role="dialog" aria-modal="true" aria-labelledby="world-reset-title">
          <div className="max-h-[90dvh] w-full max-w-xl overflow-y-auto rounded-lg bg-white shadow-2xl">
            <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
              <div className="flex min-w-0 items-center gap-3">
                <div className="rounded-lg bg-rose-50 p-2 text-rose-600"><AlertTriangle size={18} /></div>
                <div className="min-w-0">
                  <h2 id="world-reset-title" className="text-base font-bold text-slate-900">重置当前世界</h2>
                  <p className="truncate font-mono text-[11px] text-slate-400">{worldQuery.data?.active_world_id || '正在读取...'}</p>
                </div>
              </div>
              <button type="button" onClick={() => setResetOpen(false)} className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50" aria-label="关闭">
                <X size={16} />
              </button>
            </div>

            <div className="space-y-5 p-5">
              {worldQuery.isLoading ? (
                <div className="py-8 text-center text-xs font-semibold text-slate-400"><RefreshCw className="mr-2 inline animate-spin" size={14} />正在读取世界信息...</div>
              ) : (
                <>
                  <dl className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div className="rounded-lg border border-slate-100 bg-slate-50 p-3"><dt className="text-[10px] font-bold text-slate-400">世界 ID</dt><dd className="mt-1 break-all font-mono text-xs font-bold text-slate-700">{worldQuery.data?.active_world_id || '-'}</dd></div>
                    <div className="rounded-lg border border-slate-100 bg-slate-50 p-3"><dt className="text-[10px] font-bold text-slate-400">存档更新时间</dt><dd className="mt-1 text-xs font-bold text-slate-700">{worldQuery.data?.last_modified ? new Date(worldQuery.data.last_modified).toLocaleString('zh-CN') : '-'}</dd></div>
                    <div className="rounded-lg border border-slate-100 bg-slate-50 p-3"><dt className="text-[10px] font-bold text-slate-400">运行状态</dt><dd className="mt-1 text-xs font-bold text-slate-700">{worldQuery.data?.server_running ? '运行中，将自动重启' : '已停止'}</dd></div>
                  </dl>

                  <div className="rounded-lg border border-emerald-100 bg-emerald-50 p-4">
                    <p className="text-xs font-bold text-emerald-800">将保留</p>
                    <p className="mt-2 text-[11px] font-semibold leading-6 text-emerald-700">服务端程序、INI 配置、Workshop Mod、PalDefender、面板数据和验证备份</p>
                  </div>

                  {!worldQuery.data?.reset_available && (
                    <div className="rounded-lg border border-amber-100 bg-amber-50 px-4 py-3 text-xs font-semibold text-amber-700">
                      当前世界不可重置：{worldQuery.data?.reset_unavailable_reason || '世界信息不可用'}
                    </div>
                  )}

                  <label className="block text-xs font-semibold text-slate-600">
                    输入 <span className="font-mono font-bold text-rose-600">RESET WORLD</span> 确认
                    <input
                      type="text"
                      value={resetConfirmation}
                      onChange={(event) => setResetConfirmation(event.target.value)}
                      autoComplete="off"
                      className="mt-2 w-full rounded-lg border border-slate-200 px-3 py-2.5 font-mono text-xs font-semibold text-slate-800 focus:border-rose-400 focus:outline-none"
                    />
                  </label>
                </>
              )}

              {resetJob && (
                <div className="rounded-lg border border-slate-100 bg-slate-50 p-4">
                  <div className="flex items-center justify-between gap-3 text-xs font-semibold text-slate-600"><span>{resetJob.message || '世界重置任务'}</span><span>{resetJob.progress}%</span></div>
                  <div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-200"><div className="h-full bg-rose-500 transition-all" style={{ width: `${resetJob.progress}%` }} /></div>
                  {resetJob.error && <p className="mt-3 break-words text-[11px] font-semibold leading-5 text-rose-700">{resetJob.error}</p>}
                </div>
              )}

              <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
                <button type="button" onClick={() => setResetOpen(false)} className="rounded-lg border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">关闭</button>
                <button
                  type="button"
                  onClick={() => void resetWorld()}
                  disabled={!worldQuery.data?.reset_available || resetConfirmation !== 'RESET WORLD' || resetSubmitting}
                  className="inline-flex items-center gap-2 rounded-lg bg-rose-600 px-4 py-2 text-xs font-bold text-white hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {resetSubmitting ? <RefreshCw className="animate-spin" size={14} /> : <Trash2 size={14} />}
                  执行重置
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

const EmptyChart = () => (
  <div className="flex h-full items-center justify-center rounded-xl border border-dashed border-slate-200 bg-slate-50 text-sm font-medium text-slate-500">
    暂无历史采样
  </div>
);
