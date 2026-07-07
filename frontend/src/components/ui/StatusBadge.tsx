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

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status, customText }) => {
  const getStyle = () => {
    switch (status) {
      case 'Online':
      case 'running':
      case 'success':
      case 'Healthy':
      case 'Safe':
      case 'installed':
      case 'enabled':
        return 'bg-emerald-50 text-emerald-700 border-emerald-100';
      case 'Offline':
      case 'stopped':
      case 'missing':
      case 'disabled':
        return 'bg-slate-50 text-slate-600 border-slate-100';
      case 'Warning':
      case 'starting':
      case 'stopping':
      case 'waiting':
      case 'Injured':
        return 'bg-amber-50 text-amber-700 border-amber-100';
      case 'Error':
      case 'error':
      case 'failed':
      case 'Dead':
      case 'Raid':
        return 'bg-rose-50 text-rose-700 border-rose-100';
      case 'Syncing':
      case 'running_job':
      case 'Working':
        return 'bg-sky-50 text-sky-700 border-sky-100';
      case 'Backup':
      case 'updating':
      case 'Battling':
        return 'bg-indigo-50 text-indigo-700 border-indigo-100';
      default:
        return 'bg-slate-50 text-slate-600 border-slate-100';
    }
  };

  return (
    <span className={`inline-flex items-center rounded-lg border px-2.5 py-0.5 text-[11px] font-bold ${getStyle()}`}>
      {customText || statusText[String(status)] || String(status)}
    </span>
  );
};
