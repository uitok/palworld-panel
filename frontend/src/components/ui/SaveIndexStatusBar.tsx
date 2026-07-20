import React from 'react';
import { AlertCircle, ChevronDown, Database, RefreshCw, RotateCcw } from 'lucide-react';
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

const formatTimestamp = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false });
};

const warningLabel = (warnings: string[]) => {
  const playerWarnings = warnings.filter((warning) => warning.startsWith('player save warning:')).length;
  const otherWarnings = warnings.length - playerWarnings;
  const parts = [
    playerWarnings > 0 ? `${playerWarnings} 个玩家存档解析失败` : '',
    otherWarnings > 0 ? `${otherWarnings} 条其他警告` : '',
  ].filter(Boolean);
  return parts.join('，') || `${warnings.length} 条解析警告`;
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
  const warnings = status?.warnings ?? [];
  const details = [
    status?.updated_at ? `更新于 ${formatTimestamp(status.updated_at)}` : '',
    status?.parser ? `解析器 ${status.parser}` : '',
    counts ? `玩家 ${counts.players} · 公会 ${counts.guilds} · 基地 ${counts.bases} · 帕鲁 ${counts.pals}` : '',
  ].filter(Boolean).join(' · ');

  return (
    <div className={`rounded-xl border px-3 py-2.5 text-sm font-semibold ${tone}`}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-white/60">
            {isProblem ? <AlertCircle size={15} /> : <Database size={15} />}
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
              <span>{stateText[state] || state}</span>
              {warnings.length > 0 && <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-bold text-amber-800">{warningLabel(warnings)}</span>}
            </div>
            {details && <p className="mt-0.5 truncate text-[11px] font-medium opacity-75" title={details}>{details}</p>}
            {status?.error && <p className="mt-1 max-h-10 overflow-hidden text-[11px] font-semibold leading-5 text-rose-700" title={status.error}>错误：{status.error}</p>}
          </div>
        </div>
        <div className="flex shrink-0 gap-2 pl-9 sm:pl-0">
          <button type="button" onClick={onRefresh} disabled={loading || rebuilding} className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-current/20 bg-white/35 px-2.5 text-[11px] font-bold hover:bg-white/60 disabled:opacity-40">
            <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />刷新
          </button>
          <button type="button" onClick={onRebuild} disabled={!status?.enabled || loading || rebuilding} className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-current/20 bg-white/35 px-2.5 text-[11px] font-bold hover:bg-white/60 disabled:opacity-40">
            <RotateCcw size={12} className={rebuilding ? 'animate-spin' : ''} />重建
          </button>
        </div>
      </div>
      {warnings.length > 0 && (
        <details className="mt-2 border-t border-current/10 pt-2 text-[11px] font-medium">
          <summary className="flex cursor-pointer list-none items-center gap-1.5 font-bold opacity-80">
            <ChevronDown size={12} />查看解析警告详情（{warnings.length}）
          </summary>
          <div className="mt-2 max-h-48 space-y-1 overflow-y-auto rounded-lg bg-white/50 p-2 font-mono text-[10px] font-medium leading-4 opacity-80">
            {warnings.map((warning, index) => <p key={`${index}-${warning}`} className="break-all">{warning}</p>)}
          </div>
        </details>
      )}
    </div>
  );
};
