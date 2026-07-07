import React, { useEffect, useMemo, useState } from 'react';
import { Activity, Cpu, Network, ShieldCheck } from 'lucide-react';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { useServerStore } from '../store/useServerStore';
import { StatCard } from '../components/ui/StatCard';

const initialLatency = [
  { time: '10:40:00', ping: 24 },
  { time: '10:40:10', ping: 26 },
  { time: '10:40:20', ping: 25 },
  { time: '10:40:30', ping: 32 },
  { time: '10:40:40', ping: 24 },
  { time: '10:40:50', ping: 28 },
  { time: '10:41:00', ping: 24 },
];

export const Monitor: React.FC = () => {
  const { metrics, autoRefresh, status } = useServerStore();
  const [latencyData, setLatencyData] = useState(initialLatency);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      setLatencyData((prev) => {
        const nextTime = new Date().toTimeString().split(' ')[0];
        const nextPing = Math.floor(20 + Math.random() * 15);
        return [...prev.slice(1), { time: nextTime, ping: nextPing }];
      });
    }, 2000);
    return () => clearInterval(interval);
  }, [autoRefresh]);

  const frameTimeData = useMemo(
    () =>
      Array.from({ length: 8 }).map((_, index) => ({
        time: String(index + 1),
        frame: Number(((metrics?.frame_time || 16.6) + (Math.random() - 0.5)).toFixed(2)),
      })),
    [metrics?.frame_time],
  );

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
        <StatCard title="Server FPS" value={`${metrics?.server_fps || 0} FPS`} icon={<Activity size={16} />} trend={status?.status === 'running' ? '运行中' : '服务未启动'} trendType={status?.status === 'running' ? 'up' : 'neutral'} color="emerald" />
        <StatCard title="帧耗时" value={`${metrics?.frame_time || 0} ms`} icon={<Cpu size={16} />} trend="来自 metrics 接口" trendType="neutral" color="blue" />
        <StatCard title="网络丢包" value="0.0%" icon={<Network size={16} />} trend="前端诊断模拟值" trendType="info" color="sky" />
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <ChartCard title="网络延迟波动">
          <AreaChart data={latencyData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
            <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
            <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} />
            <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
            <Area type="monotone" dataKey="ping" name="Ping" stroke="#0ea5e9" fill="#e0f2fe" strokeWidth={1.5} />
          </AreaChart>
        </ChartCard>

        <ChartCard title="帧耗时波动">
          <AreaChart data={frameTimeData} margin={{ top: 5, right: 10, left: -25, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
            <XAxis dataKey="time" stroke="#94a3b8" fontSize={10} tickLine={false} />
            <YAxis stroke="#94a3b8" fontSize={10} tickLine={false} />
            <Tooltip contentStyle={{ fontSize: '11px', borderRadius: '12px', border: '1px solid #f1f5f9' }} />
            <Area type="monotone" dataKey="frame" name="Frame Time" stroke="#14b8a6" fill="#ccfbf1" strokeWidth={1.5} />
          </AreaChart>
        </ChartCard>
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="mb-4 flex items-center gap-2 border-b border-slate-100 pb-3">
          <ShieldCheck size={16} className="text-emerald-500" />
          <h3 className="text-[14px] font-bold text-slate-800">环境诊断</h3>
        </div>
        <div className="grid grid-cols-1 gap-4 text-xs font-semibold text-slate-500 md:grid-cols-2 xl:grid-cols-4">
          <Diag label="Runtime" value={status?.runtime_mode || 'wine_docker'} />
          <Diag label="Game Port" value={String(status?.ports?.game || 8211)} />
          <Diag label="REST Port" value={String(status?.ports?.rest || 8212)} />
          <Diag label="Setup Step" value={status?.setup_step || 'prerequisites'} />
        </div>
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

const Diag: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div className="rounded-2xl border border-slate-100 bg-slate-50 p-4">
    <span className="text-slate-400">{label}</span>
    <p className="mt-1 break-all font-bold text-slate-800">{value}</p>
  </div>
);
