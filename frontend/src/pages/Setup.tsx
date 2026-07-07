import React, { useCallback, useEffect, useState } from 'react';
import { CheckCircle2, Download, Play, RefreshCw, Save, Server, Settings2, Wand2 } from 'lucide-react';
import { setupApi } from '../api/setup';
import { serverApi } from '../api/server';
import { tasksApi } from '../api/tasks';
import type { Job, Prerequisite, RuntimeMode, ServerStatus, StartupConfig, StartupResponse } from '../types';
import { StatusBadge } from '../components/ui/StatusBadge';

const runtimeLabels: Record<RuntimeMode, string> = {
  windows_steamcmd: 'Windows SteamCMD（推荐正式服）',
  wine_docker: 'Docker + Wine（兼容 Windows Mod）',
};

export const Setup: React.FC = () => {
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [prerequisites, setPrerequisites] = useState<Prerequisite[]>([]);
  const [runtime, setRuntime] = useState<RuntimeMode>('wine_docker');
  const [startup, setStartup] = useState<StartupResponse | null>(null);
  const [activeJob, setActiveJob] = useState<Job | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    const [nextStatus, checks, runtimeRes, startupRes] = await Promise.all([
      serverApi.getStatus(),
      setupApi.getPrerequisites(),
      setupApi.getRuntime(),
      setupApi.getStartup(),
    ]);
    setStatus(nextStatus);
    setPrerequisites(checks);
    setRuntime(runtimeRes.mode);
    setStartup(startupRes);
    setLoading(false);
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const trackJob = async (job: Job) => {
    setActiveJob(job);
    const done = await tasksApi.waitForJob(job.id, setActiveJob);
    setMessage(done.status === 'success' ? '任务已完成' : done.error || '任务执行失败');
    await refresh();
  };

  const setRuntimeMode = async (mode: RuntimeMode) => {
    setRuntime(mode);
    await setupApi.setRuntime(mode);
    await refresh();
  };

  const updateStartup = (key: keyof StartupConfig, value: string | number | boolean) => {
    if (!startup) return;
    setStartup({ ...startup, startup: { ...startup.startup, [key]: value } });
  };

  const saveStartup = async () => {
    if (!startup) return;
    const saved = await setupApi.setStartup(startup.startup);
    setStartup(saved);
    setMessage('启动参数已保存');
  };

  const initializeConfig = async () => {
    const result = await setupApi.initializeConfig();
    setMessage(result.path ? `配置已初始化：${result.path}` : '配置已初始化');
    await refresh();
  };

  const requiredMissing = prerequisites.some((item) => item.required && !item.ok);

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 className="flex items-center gap-2 text-[16px] font-bold text-slate-800">
              <Wand2 size={18} className="text-sky-500" />
              开服流程
            </h3>
            <p className="mt-1 text-xs font-medium text-slate-400">
              按后端状态机完成环境检查、服务端安装、配置初始化、启动参数和启动验证。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={refresh}
              className="flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50"
            >
              <RefreshCw size={14} />
              刷新
            </button>
            <button
              type="button"
              onClick={() => setupApi.bootstrap().then(trackJob)}
              disabled={requiredMissing || Boolean(activeJob && activeJob.status === 'running')}
              className="flex items-center gap-2 rounded-xl bg-sky-500 px-4 py-2 text-xs font-semibold text-white hover:bg-sky-600 disabled:opacity-40"
            >
              <Download size={14} />
              一键初始化
            </button>
          </div>
        </div>

        {message && (
          <div className="mt-4 rounded-2xl border border-sky-100 bg-sky-50 px-4 py-3 text-xs font-semibold text-sky-700">
            {message}
          </div>
        )}

        {activeJob && (
          <div className="mt-4 rounded-2xl border border-slate-100 bg-slate-50 p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="truncate text-xs font-bold text-slate-700">{activeJob.message || activeJob.type}</p>
                {activeJob.error && <p className="mt-1 text-[11px] font-medium text-rose-600">{activeJob.error}</p>}
              </div>
              <StatusBadge status={activeJob.status === 'running' ? 'running_job' : activeJob.status} />
            </div>
            <div className="mt-3 h-2 overflow-hidden rounded-full bg-white">
              <div className="h-full rounded-full bg-sky-500 transition-all" style={{ width: `${activeJob.progress}%` }} />
            </div>
          </div>
        )}
      </section>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <h4 className="mb-4 flex items-center gap-2 text-sm font-bold text-slate-800">
            <CheckCircle2 size={16} className="text-emerald-500" />
            环境检查
          </h4>
          <div className="flex flex-col gap-3">
            {loading ? (
              <p className="text-xs font-semibold text-slate-400">正在检查环境...</p>
            ) : prerequisites.length > 0 ? (
              prerequisites.map((item) => (
                <div key={item.id} className="rounded-2xl border border-slate-100 bg-slate-50/70 p-3">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-xs font-bold text-slate-700">{item.label}</span>
                    <StatusBadge status={item.ok ? 'success' : item.required ? 'failed' : 'Warning'} />
                  </div>
                  {item.message && <p className="mt-1 break-all text-[11px] font-medium text-slate-400">{item.message}</p>}
                </div>
              ))
            ) : (
              <p className="text-xs font-semibold text-slate-400">暂无环境检查结果，后端未就绪时会显示空状态。</p>
            )}
          </div>
        </section>

        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <h4 className="mb-4 flex items-center gap-2 text-sm font-bold text-slate-800">
            <Server size={16} className="text-sky-500" />
            Runtime
          </h4>
          <div className="flex flex-col gap-3">
            {(['windows_steamcmd', 'wine_docker'] as RuntimeMode[]).map((mode) => (
              <button
                type="button"
                key={mode}
                onClick={() => setRuntimeMode(mode)}
                className={`rounded-2xl border p-4 text-left transition-all ${
                  runtime === mode
                    ? 'border-sky-200 bg-sky-50 text-sky-800'
                    : 'border-slate-100 bg-slate-50/70 text-slate-600 hover:border-slate-200'
                }`}
              >
                <span className="text-xs font-bold">{runtimeLabels[mode]}</span>
                <p className="mt-1 text-[11px] font-medium opacity-75">
                  {mode === 'windows_steamcmd'
                    ? '使用本机 SteamCMD 管理 Windows 版服务端。'
                    : '使用 Docker + Wine 运行 Windows 版服务端和 Mod。'}
                </p>
              </button>
            ))}
          </div>
        </section>

        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <h4 className="mb-4 flex items-center gap-2 text-sm font-bold text-slate-800">
            <Settings2 size={16} className="text-indigo-500" />
            当前状态
          </h4>
          <div className="grid grid-cols-2 gap-3 text-xs font-semibold">
            <StatusItem label="服务端" value={status?.installed ? '已安装' : '未安装'} ok={Boolean(status?.installed)} />
            <StatusItem label="配置文件" value={status?.config_exists ? '已初始化' : '未初始化'} ok={Boolean(status?.config_exists)} />
            <StatusItem label="进程" value={status?.status || 'stopped'} ok={status?.status === 'running'} />
            <StatusItem label="配置状态" value={status?.pending_restart ? '待重启' : '已生效'} ok={!status?.pending_restart} />
          </div>
          {status?.warnings && status.warnings.length > 0 && (
            <div className="mt-4 rounded-2xl border border-amber-100 bg-amber-50 p-3 text-[11px] font-medium text-amber-800">
              {status.warnings.join(' / ')}
            </div>
          )}
          <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
            <button type="button" onClick={() => setupApi.install().then(trackJob)} className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">
              安装
            </button>
            <button type="button" onClick={initializeConfig} className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">
              初始化配置
            </button>
            <button type="button" onClick={() => serverApi.start().then(refresh)} className="flex items-center justify-center gap-1.5 rounded-xl bg-emerald-500 px-3 py-2 text-xs font-bold text-white hover:bg-emerald-600">
              <Play size={13} />
              启动
            </button>
          </div>
        </section>
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h4 className="text-sm font-bold text-slate-800">启动参数</h4>
            <p className="mt-1 text-xs font-medium text-slate-400">
              这些参数会参与 PalServer.exe 启动命令，保存后下次启动生效。
            </p>
          </div>
          <button
            type="button"
            onClick={saveStartup}
            className="flex items-center justify-center gap-2 rounded-xl bg-slate-900 px-4 py-2 text-xs font-bold text-white hover:bg-slate-800"
          >
            <Save size={14} />
            保存启动参数
          </button>
        </div>

        {startup && (
          <>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
              <NumberField label="监听端口" value={startup.startup.port} onChange={(value) => updateStartup('port', value)} />
              <NumberField label="最大玩家" value={startup.startup.players} onChange={(value) => updateStartup('players', value)} />
              <NumberField label="公网端口" value={startup.startup.public_port || 8211} onChange={(value) => updateStartup('public_port', value)} />
              <NumberField label="工作线程" value={startup.startup.number_of_worker_threads_server || 0} onChange={(value) => updateStartup('number_of_worker_threads_server', value)} />
              <TextField label="公网 IP" value={startup.startup.public_ip || ''} onChange={(value) => updateStartup('public_ip', value)} />
              <TextField label="Workshop 目录" value={startup.startup.workshop_dir || ''} onChange={(value) => updateStartup('workshop_dir', value)} />
              <SelectField label="日志格式" value={startup.startup.log_format} onChange={(value) => updateStartup('log_format', value)} />
              <div className="grid grid-cols-1 gap-2 rounded-2xl border border-slate-100 bg-slate-50/70 p-3">
                {[
                  ['public_lobby', '公开大厅'],
                  ['use_perf_threads', '性能线程'],
                  ['no_async_loading_thread', '禁用异步加载线程'],
                  ['use_multithread_for_ds', 'DS 多线程'],
                  ['no_mods', '禁用 Mod'],
                ].map(([key, label]) => (
                  <label key={key} className="flex items-center gap-2 text-xs font-semibold text-slate-600">
                    <input
                      type="checkbox"
                      checked={Boolean(startup.startup[key as keyof StartupConfig])}
                      onChange={(event) => updateStartup(key as keyof StartupConfig, event.target.checked)}
                      className="h-4 w-4 rounded border-slate-300 text-sky-500 focus:ring-sky-500"
                    />
                    {label}
                  </label>
                ))}
              </div>
            </div>

            {startup.issues.length > 0 && (
              <div className="mt-4 rounded-2xl border border-amber-100 bg-amber-50 p-3">
                {startup.issues.map((issue, index) => (
                  <p key={index} className="text-[11px] font-semibold text-amber-800">
                    {issue.field ? `${issue.field}: ` : ''}
                    {issue.message}
                  </p>
                ))}
              </div>
            )}

            <div className="mt-4 rounded-2xl bg-slate-950 p-4 text-[11px] font-semibold text-emerald-300">
              <pre className="overflow-x-auto whitespace-pre-wrap">{startup.args.join(' ') || '保存后生成启动参数'}</pre>
            </div>
          </>
        )}
      </section>
    </div>
  );
};

const StatusItem: React.FC<{ label: string; value: string; ok: boolean }> = ({ label, value, ok }) => (
  <div className="rounded-2xl border border-slate-100 bg-slate-50/70 p-3">
    <span className="text-[11px] text-slate-400">{label}</span>
    <p className={`mt-1 font-bold ${ok ? 'text-emerald-600' : 'text-slate-700'}`}>{value}</p>
  </div>
);

const NumberField: React.FC<{ label: string; value: number; onChange: (value: number) => void }> = ({ label, value, onChange }) => (
  <label className="flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
    {label}
    <input
      type="number"
      value={value}
      onChange={(event) => onChange(Number(event.target.value))}
      className="rounded-xl border border-slate-200 p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
    />
  </label>
);

const TextField: React.FC<{ label: string; value: string; onChange: (value: string) => void }> = ({ label, value, onChange }) => (
  <label className="flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
    {label}
    <input
      type="text"
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className="rounded-xl border border-slate-200 p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
    />
  </label>
);

const SelectField: React.FC<{ label: string; value: string; onChange: (value: string) => void }> = ({ label, value, onChange }) => (
  <label className="flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
    {label}
    <select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className="rounded-xl border border-slate-200 bg-white p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
    >
      <option value="text">text</option>
      <option value="json">json</option>
    </select>
  </label>
);
