import React, { useEffect, useMemo, useState } from 'react';
import { DownloadCloud, FileArchive, Power, RefreshCw, Trash2, UploadCloud } from 'lucide-react';
import { modsApi } from '../api/mods';
import { serverApi } from '../api/server';
import { tasksApi } from '../api/tasks';
import type { Job, ModItem } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

export const Mods: React.FC = () => {
  const [mods, setMods] = useState<ModItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [enableOnUpload, setEnableOnUpload] = useState(true);
  const [workshopId, setWorkshopId] = useState('');
  const [activeJob, setActiveJob] = useState<Job | null>(null);
  const [pendingRestart, setPendingRestart] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    const [list, status] = await Promise.all([modsApi.list(), serverApi.getStatus()]);
    setMods(Array.isArray(list) ? list : []);
    setPendingRestart(status.pending_restart);
    setLoading(false);
  };

  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return mods;
    return mods.filter(
      (mod) =>
        mod.name.toLowerCase().includes(keyword) ||
        mod.package_name.toLowerCase().includes(keyword) ||
        mod.id.toLowerCase().includes(keyword),
    );
  }, [mods, search]);

  const trackJob = async (job: Job) => {
    setActiveJob(job);
    const done = await tasksApi.waitForJob(job.id, setActiveJob);
    setMessage(done.status === 'success' ? '任务已完成' : done.error || '任务失败');
    await load();
  };

  const upload = async () => {
    if (!file) {
      setMessage('请选择一个 Mod zip 文件');
      return;
    }
    await modsApi.upload(file, enableOnUpload);
    setFile(null);
    setMessage('Mod 已上传并解析，变更需要重启生效');
    await load();
  };

  const downloadWorkshop = async () => {
    if (!workshopId.trim()) {
      setMessage('请输入 Steam Workshop Item ID');
      return;
    }
    const job = await modsApi.downloadWorkshop(workshopId.trim());
    setWorkshopId('');
    await trackJob(job);
  };

  const toggleMod = async (mod: ModItem) => {
    await modsApi.setEnabled(mod.id, !mod.enabled);
    setMessage(`${mod.name} 已${mod.enabled ? '禁用' : '启用'}，重启后生效`);
    await load();
  };

  const deleteMod = async (mod: ModItem) => {
    if (!window.confirm(`删除 Mod "${mod.name}"？`)) return;
    await modsApi.delete(mod.id);
    setMessage('Mod 已删除，重启后生效');
    await load();
  };

  const headers = [
    { key: 'name', label: 'Mod' },
    { key: 'package', label: 'PackageName' },
    { key: 'source', label: '来源' },
    { key: 'status', label: '状态' },
    { key: 'updated', label: '更新时间' },
    { key: 'actions', label: '操作', align: 'center' as const },
  ];

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {pendingRestart && (
        <div className="rounded-2xl border border-amber-100 bg-amber-50 px-5 py-4">
          <p className="text-xs font-bold text-amber-800">Mod 列表已变更，服务器需要重启后生效。</p>
        </div>
      )}
      {message && <div className="rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3 text-xs font-semibold text-sky-700">{message}</div>}

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <h3 className="mb-4 flex items-center gap-2 text-[15px] font-bold text-slate-800">
            <UploadCloud size={18} className="text-sky-500" />
            上传 Mod Zip
          </h3>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <input
              type="file"
              accept=".zip"
              onChange={(event) => setFile(event.target.files?.[0] || null)}
              className="min-w-0 flex-1 rounded-xl border border-slate-200 p-2 text-xs font-semibold text-slate-600 file:mr-3 file:rounded-lg file:border-0 file:bg-slate-900 file:px-3 file:py-1.5 file:text-xs file:font-bold file:text-white"
            />
            <label className="flex items-center gap-2 text-xs font-semibold text-slate-600">
              <input
                type="checkbox"
                checked={enableOnUpload}
                onChange={(event) => setEnableOnUpload(event.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-sky-500 focus:ring-sky-500"
              />
              上传后启用
            </label>
            <button type="button" onClick={upload} className="rounded-xl bg-sky-500 px-4 py-2 text-xs font-bold text-white hover:bg-sky-600">
              上传
            </button>
          </div>
          <p className="mt-3 text-[11px] font-medium text-slate-400">
            后端会查找 Info.json，读取 PackageName 并写入官方 Mods/PalModSettings.ini。
          </p>
        </section>

        <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
          <h3 className="mb-4 flex items-center gap-2 text-[15px] font-bold text-slate-800">
            <DownloadCloud size={18} className="text-indigo-500" />
            Steam Workshop 下载
          </h3>
          <div className="flex flex-col gap-3 sm:flex-row">
            <input
              type="text"
              value={workshopId}
              onChange={(event) => setWorkshopId(event.target.value)}
              placeholder="Workshop Item ID"
              className="min-w-0 flex-1 rounded-xl border border-slate-200 p-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
            />
            <button
              type="button"
              onClick={downloadWorkshop}
              className="rounded-xl bg-slate-900 px-4 py-2 text-xs font-bold text-white hover:bg-slate-800"
            >
              下载
            </button>
          </div>
          {activeJob && (
            <div className="mt-4 rounded-2xl border border-slate-100 bg-slate-50 p-3">
              <div className="flex items-center justify-between text-xs font-bold text-slate-700">
                <span>{activeJob.message || activeJob.type}</span>
                <StatusBadge status={activeJob.status === 'running' ? 'running_job' : activeJob.status} />
              </div>
              <div className="mt-3 h-2 rounded-full bg-white">
                <div className="h-full rounded-full bg-indigo-500" style={{ width: `${activeJob.progress}%` }} />
              </div>
            </div>
          )}
        </section>
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在读取 Mod 列表...
          </div>
        ) : (
          <DataTable
            title={`已安装 Mod（${mods.length}）`}
            headers={headers}
            data={filtered}
            searchText={search}
            onSearchChange={setSearch}
            searchPlaceholder="搜索 Mod 名称、PackageName 或 ID"
            emptyText="暂无 Mod"
            renderCard={(mod) => (
              <ModCard key={mod.id} mod={mod} onToggle={() => toggleMod(mod)} onDelete={() => deleteMod(mod)} />
            )}
            renderRow={(mod) => (
              <tr key={mod.id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4">
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-sky-50 text-sky-500">
                      <FileArchive size={16} />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-xs font-bold text-slate-700">{mod.name}</p>
                      <p className="truncate font-mono text-[10px] text-slate-400">{mod.id}</p>
                    </div>
                  </div>
                </td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-600">{mod.package_name || '-'}</td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-500">{mod.source}</td>
                <td className="px-6 py-4">
                  <StatusBadge status={mod.enabled ? 'enabled' : 'disabled'} />
                </td>
                <td className="px-6 py-4 text-xs font-medium text-slate-400">{mod.updated_at || '-'}</td>
                <td className="px-6 py-4 text-center">
                  <div className="flex justify-center gap-2">
                    <button type="button" onClick={() => toggleMod(mod)} className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50" aria-label="启用或禁用 Mod">
                      <Power size={14} />
                    </button>
                    <button type="button" onClick={() => deleteMod(mod)} className="rounded-lg border border-rose-200 p-2 text-rose-500 hover:bg-rose-50" aria-label="删除 Mod">
                      <Trash2 size={14} />
                    </button>
                  </div>
                </td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};

const ModCard: React.FC<{ mod: ModItem; onToggle: () => void; onDelete: () => void }> = ({ mod, onToggle, onDelete }) => (
  <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-bold text-slate-800">{mod.name}</p>
        <p className="mt-1 truncate font-mono text-[11px] text-slate-400">{mod.package_name || mod.id}</p>
      </div>
      <StatusBadge status={mod.enabled ? 'enabled' : 'disabled'} />
    </div>
    <div className="mt-4 grid grid-cols-2 gap-2">
      <button type="button" onClick={onToggle} className="rounded-xl border border-slate-200 py-2 text-xs font-bold text-slate-600">
        {mod.enabled ? '禁用' : '启用'}
      </button>
      <button type="button" onClick={onDelete} className="rounded-xl border border-rose-200 py-2 text-xs font-bold text-rose-600">
        删除
      </button>
    </div>
  </div>
);
