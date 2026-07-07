import React, { useEffect, useState } from 'react';
import { DownloadCloud, FolderDown, RefreshCw } from 'lucide-react';
import { tasksApi } from '../api/tasks';
import { useServerStore } from '../store/useServerStore';
import type { Job } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

const jobTypeLabel = (type: string) => {
  switch (type) {
    case 'backup':
      return '全服备份';
    case 'update':
      return '服务端更新';
    case 'install':
      return '服务端安装';
    case 'bootstrap':
      return '开服初始化';
    case 'workshop_download':
      return 'Workshop 下载';
    case 'paldefender_install':
      return 'PalDefender 安装';
    case 'paldefender_update':
      return 'PalDefender 更新';
    case 'restart':
      return '服务器重启';
    case 'broadcast':
      return '系统广播';
    case 'save':
      return '保存世界';
    case 'shutdown':
      return '关闭服务器';
    default:
      return type;
  }
};

export const TaskQueue: React.FC = () => {
  const { refreshKey } = useServerStore();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchJobs = async () => {
    setLoading(true);
    const data = await tasksApi.getJobs();
    setJobs(Array.isArray(data) ? data : []);
    setLoading(false);
  };

  useEffect(() => {
    fetchJobs();
  }, [refreshKey]);

  useEffect(() => {
    const hasRunning = jobs.some((job) => job.status === 'running');
    if (!hasRunning) return;
    const interval = setInterval(fetchJobs, 1000);
    return () => clearInterval(interval);
  }, [jobs]);

  const createBackup = async () => {
    await tasksApi.createBackupJob();
    await fetchJobs();
  };

  const createUpdate = async () => {
    await tasksApi.createUpdateJob();
    await fetchJobs();
  };

  const headers = [
    { key: 'id', label: '任务 ID' },
    { key: 'type', label: '类型' },
    { key: 'status', label: '状态' },
    { key: 'progress', label: '进度' },
    { key: 'message', label: '消息' },
    { key: 'created_at', label: '创建时间' },
  ];

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="flex flex-col gap-3 sm:flex-row">
        <button
          type="button"
          onClick={createBackup}
          className="flex items-center justify-center gap-2 rounded-2xl bg-sky-500 px-5 py-3 text-xs font-semibold text-white hover:bg-sky-600"
        >
          <FolderDown size={14} />
          立即执行备份
        </button>

        <button
          type="button"
          onClick={createUpdate}
          className="flex items-center justify-center gap-2 rounded-2xl border border-slate-200 bg-white px-5 py-3 text-xs font-semibold text-slate-600 hover:bg-slate-50"
        >
          <DownloadCloud size={14} />
          检查并更新服务端
        </button>
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading && jobs.length === 0 ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在获取任务队列...
          </div>
        ) : (
          <DataTable
            headers={headers}
            data={jobs}
            title="最近任务运行记录"
            emptyText="暂无任务"
            renderCard={(job) => <JobCard key={job.id} job={job} />}
            renderRow={(job) => (
              <tr key={job.id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 font-mono text-xs font-bold text-slate-600">#{job.id}</td>
                <td className="px-6 py-4 text-xs font-bold text-slate-700">{jobTypeLabel(job.type)}</td>
                <td className="px-6 py-4">
                  <StatusBadge status={job.status === 'running' ? 'running_job' : job.status} />
                </td>
                <td className="px-6 py-4">
                  <Progress job={job} />
                </td>
                <td className="max-w-[320px] px-6 py-4 text-xs font-medium text-slate-500">
                  {job.error || job.message || '-'}
                </td>
                <td className="px-6 py-4 text-xs font-medium text-slate-400">{job.created_at}</td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};

const progressText = (status: Job['status']) => {
  if (status === 'running') return '执行中';
  if (status === 'success') return '完成';
  if (status === 'failed') return '失败';
  return '等待';
};

const Progress: React.FC<{ job: Job }> = ({ job }) => (
  <div className="flex w-36 max-w-full flex-col gap-1.5">
    <div className="flex justify-between text-[10px] font-bold text-slate-400">
      <span>{progressText(job.status)}</span>
      <span>{job.progress || 0}%</span>
    </div>
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
      <div
        style={{ width: `${job.progress || 0}%` }}
        className={`h-full rounded-full transition-all duration-300 ${
          job.status === 'failed' ? 'bg-rose-500' : job.status === 'success' ? 'bg-emerald-500' : 'animate-pulse bg-sky-500'
        }`}
      />
    </div>
  </div>
);

const JobCard: React.FC<{ job: Job }> = ({ job }) => (
  <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <p className="text-sm font-bold text-slate-800">{jobTypeLabel(job.type)}</p>
        <p className="mt-1 truncate font-mono text-[10px] text-slate-400">#{job.id}</p>
      </div>
      <StatusBadge status={job.status === 'running' ? 'running_job' : job.status} />
    </div>
    <div className="mt-4">
      <Progress job={job} />
    </div>
    <p className="mt-3 text-[11px] font-medium text-slate-500">{job.error || job.message || '无消息'}</p>
    <p className="mt-2 text-[10px] text-slate-400">{job.updated_at || job.created_at}</p>
  </div>
);
