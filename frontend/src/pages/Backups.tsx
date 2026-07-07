import React, { useEffect, useMemo, useState } from 'react';
import { Archive, FolderDown, RefreshCw } from 'lucide-react';
import { backupsApi } from '../api/backups';
import { tasksApi } from '../api/tasks';
import type { BackupInfo, Job } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

const formatBytes = (bytes: number) => {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
};

export const Backups: React.FC = () => {
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeJob, setActiveJob] = useState<Job | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    const list = await backupsApi.list();
    setBackups(Array.isArray(list) ? list : []);
    setLoading(false);
  };

  useEffect(() => {
    load();
  }, []);

  const totalSize = useMemo(() => backups.reduce((sum, item) => sum + item.size_bytes, 0), [backups]);

  const createBackup = async () => {
    const job = await backupsApi.create();
    setActiveJob(job);
    const done = await tasksApi.waitForJob(job.id, setActiveJob);
    setMessage(done.status === 'success' ? '备份任务已完成' : done.error || '备份任务失败');
    await load();
  };

  const headers = [
    { key: 'name', label: '备份名称' },
    { key: 'size', label: '大小' },
    { key: 'created', label: '创建时间' },
    { key: 'path', label: '路径' },
  ];

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {message && <div className="rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3 text-xs font-semibold text-sky-700">{message}</div>}

      <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
        <SummaryCard label="备份数量" value={`${backups.length} 个`} />
        <SummaryCard label="总容量" value={formatBytes(totalSize)} />
        <SummaryCard label="位置" value="data/backups" />
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 className="flex items-center gap-2 text-[15px] font-bold text-slate-800">
              <Archive size={18} className="text-sky-500" />
              备份管理
            </h3>
            <p className="mt-1 text-xs font-medium text-slate-400">
              后端更新前会自动创建备份，也可以在这里手动创建全服备份。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={load}
              className="flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50"
            >
              <RefreshCw size={14} />
              刷新
            </button>
            <button
              type="button"
              onClick={createBackup}
              className="flex items-center gap-2 rounded-xl bg-sky-500 px-4 py-2 text-xs font-bold text-white hover:bg-sky-600"
            >
              <FolderDown size={14} />
              立即备份
            </button>
          </div>
        </div>

        {activeJob && (
          <div className="mt-4 rounded-2xl border border-slate-100 bg-slate-50 p-3">
            <div className="flex items-center justify-between gap-3 text-xs font-bold text-slate-700">
              <span>{activeJob.message || activeJob.type}</span>
              <StatusBadge status={activeJob.status === 'running' ? 'running_job' : activeJob.status} />
            </div>
            <div className="mt-3 h-2 rounded-full bg-white">
              <div className="h-full rounded-full bg-sky-500" style={{ width: `${activeJob.progress}%` }} />
            </div>
          </div>
        )}
      </section>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">正在读取备份列表...</div>
        ) : (
          <DataTable
            title="备份文件"
            headers={headers}
            data={backups}
            emptyText="暂无备份"
            renderCard={(backup) => (
              <div key={backup.name} className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
                <p className="break-all text-sm font-bold text-slate-800">{backup.name}</p>
                <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] font-semibold text-slate-500">
                  <span>大小: {formatBytes(backup.size_bytes)}</span>
                  <span>时间: {backup.created_at}</span>
                </div>
                <p className="mt-3 break-all rounded-xl bg-slate-50 p-2 font-mono text-[10px] text-slate-400">{backup.path}</p>
              </div>
            )}
            renderRow={(backup) => (
              <tr key={backup.name} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 text-xs font-bold text-slate-700">{backup.name}</td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-600">{formatBytes(backup.size_bytes)}</td>
                <td className="px-6 py-4 text-xs font-medium text-slate-400">{backup.created_at}</td>
                <td className="max-w-[420px] truncate px-6 py-4 font-mono text-[10px] text-slate-400">{backup.path}</td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};

const SummaryCard: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div className="rounded-2xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
    <p className="text-[11px] font-semibold text-slate-400">{label}</p>
    <p className="mt-1 text-2xl font-bold text-slate-800">{value}</p>
  </div>
);
