import React from 'react';
import { AlertCircle, Database, RefreshCw, RotateCcw } from 'lucide-react';
import type { SaveIndexStatus } from '../../types';

interface SaveIndexStatusBarProps {
  status: SaveIndexStatus | null;
  loading?: boolean;
  rebuilding?: boolean;
  onRefresh: () => void;
  onRebuild: () => void;
}

const stateText: Record<string, string> = {
  disabled: '存档索引未启用',
  missing: '未找到存档',
  not_indexed: '尚未索引',
  ready: '存档索引可用',
  stale: '存档索引已过期',
  error: '存档解析失败',
};

export const SaveIndexStatusBar: React.FC<SaveIndexStatusBarProps> = ({
  status,
  loading = false,
  rebuilding = false,
  onRefresh,
  onRebuild,
}) => {
  const state = status?.state || 'disabled';
  const isProblem = !status?.enabled || status.stale || state === 'error' || state === 'missing' || state === 'not_indexed';
  const tone = isProblem
    ? 'border-amber-200/80 bg-amber-50 text-amber-900'
    : 'border-emerald-200/80 bg-emerald-50 text-emerald-800';
  const counts = status?.counts;
  const updatedAt = status?.updated_at ? `上次成功: ${status.updated_at}` : '';
  const details = [
    updatedAt,
    status?.parser ? `解析器: ${status.parser}` : '',
    counts ? `玩家 ${counts.players} / 公会 ${counts.guilds} / 基地 ${counts.bases} / 帕鲁 ${counts.pals}` : '',
    status?.error ? `错误: ${status.error}` : '',
  ]
    .filter(Boolean)
    .join(' · ');

  return (
    <div className={`flex flex-col gap-4 rounded-xl border px-4 py-3.5 text-sm font-semibold ${tone} lg:flex-row lg:items-center lg:justify-between`}>
      <div className="flex min-w-0 items-start gap-2">
        <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-white/60">
          {isProblem ? <AlertCircle size={15} /> : <Database size={15} />}
        </span>
        <div className="min-w-0">
          <p>{stateText[state] || state}</p>
          {details && <p className="mt-1 break-words text-xs font-medium leading-5 opacity-80">{details}</p>}
          {status?.warnings && status.warnings.length > 0 && (
            <p className="mt-1 break-words text-xs font-medium leading-5 opacity-80">{status.warnings.join(' · ')}</p>
          )}
        </div>
      </div>
      <div className="flex shrink-0 gap-2">
        <button
          type="button"
          onClick={onRefresh}
          disabled={loading || rebuilding}
          className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-current/20 bg-white/35 px-3 text-xs font-bold hover:bg-white/60 disabled:opacity-40"
        >
          <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
          刷新
        </button>
        <button
          type="button"
          onClick={onRebuild}
          disabled={!status?.enabled || loading || rebuilding}
          className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-current/20 bg-white/35 px-3 text-xs font-bold hover:bg-white/60 disabled:opacity-40"
        >
          <RotateCcw size={12} className={rebuilding ? 'animate-spin' : ''} />
          重建
        </button>
      </div>
    </div>
  );
};
