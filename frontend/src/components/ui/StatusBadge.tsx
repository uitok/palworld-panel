import React from 'react';

export type BadgeStatus =
  | 'Online'
  | 'Offline'
  | 'Warning'
  | 'Error'
  | 'Syncing'
  | 'Backup'
  | 'running'
  | 'stopped'
  | 'starting'
  | 'stopping'
  | 'updating'
  | 'error'
  | 'waiting'
  | 'running_job'
  | 'success'
  | 'failed'
  | 'Safe'
  | 'Raid'
  | 'Healthy'
  | 'Injured'
  | 'Working'
  | 'Battling'
  | 'Dead'
  | 'installed'
  | 'missing'
  | 'enabled'
  | 'disabled';

interface StatusBadgeProps {
  status: BadgeStatus | string;
  customText?: string;
}

const statusText: Record<string, string> = {
  Online: '在线',
  Offline: '离线',
  Warning: '警告',
  Error: '异常',
  Syncing: '同步中',
  Backup: '备份',
  running: '运行中',
  stopped: '已停止',
  starting: '启动中',
  stopping: '停止中',
  updating: '更新中',
  error: '错误',
  waiting: '等待中',
  running_job: '执行中',
  success: '成功',
  failed: '失败',
  Safe: '安全',
  Raid: '遭袭',
  Healthy: '健康',
  Injured: '受伤',
  Working: '工作中',
  Battling: '战斗中',
  Dead: '死亡',
  installed: '已安装',
  missing: '未就绪',
  enabled: '已启用',
  disabled: '已禁用',
};

const toneForStatus = (status: string) => {
  if (['Online', 'running', 'success', 'Healthy', 'Safe', 'installed', 'enabled'].includes(status)) {
    return { shell: 'border-emerald-100 bg-emerald-50 text-emerald-700', dot: 'bg-emerald-500' };
  }
  if (['Warning', 'starting', 'stopping', 'waiting', 'Injured'].includes(status)) {
    return { shell: 'border-amber-100 bg-amber-50 text-amber-700', dot: 'bg-amber-500' };
  }
  if (['Error', 'error', 'failed', 'Dead', 'Raid'].includes(status)) {
    return { shell: 'border-rose-100 bg-rose-50 text-rose-700', dot: 'bg-rose-500' };
  }
  if (['Syncing', 'running_job', 'Working'].includes(status)) {
    return { shell: 'border-sky-100 bg-sky-50 text-sky-700', dot: 'bg-sky-500' };
  }
  if (['Backup', 'updating', 'Battling'].includes(status)) {
    return { shell: 'border-indigo-100 bg-indigo-50 text-indigo-700', dot: 'bg-indigo-500' };
  }
  return { shell: 'border-slate-200 bg-slate-50 text-slate-600', dot: 'bg-slate-400' };
};

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status, customText }) => {
  const tone = toneForStatus(String(status));
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-bold leading-none ${tone.shell}`}>
      <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${tone.dot}`} />
      {customText || statusText[String(status)] || String(status)}
    </span>
  );
};
