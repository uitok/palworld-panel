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

const errorCodeHint: Record<string, string> = {
  save_indexer_unavailable: 'sav-cli 存档解析服务不可用，请确认 launcher 已启动 sidecar 并检查 sav-cli.log',
  save_index_timeout: 'sav-cli 存档解析超时，请稍后重试并检查存档大小与 sav-cli.log',
  parser_incompatible: '存档解析器与当前存档格式不兼容，请查看安全详情并确认 sav-cli 版本',
  level_sav_not_found: '未找到 Level.sav（请确认存档目录路径正确）',
  save_path_not_found: '存档路径不存在（请检查存档源路径）',
  index_failed: '索引失败（请查看 sav-cli 文本日志定位原因）',
};
const oodleUnavailableHint = '当前 sav-cli 缺少 Oodle 解压能力，无法解析 PlM 压缩存档；请使用 CGO + MinGW 构建';

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
  const rawState = status?.state || 'disabled';
  const state = rawState === 'error' && status?.stale ? 'stale' : rawState;
  const isFatalError = rawState === 'error' && !status?.stale;
  const oodleUnavailable = status?.oodle_available === false;
  const showError = isFatalError || Boolean(status?.error_code) || oodleUnavailable;
  const isProblem = !status?.enabled || status.stale || state === 'error' || state === 'missing' || state === 'not_indexed' || oodleUnavailable;
  const tone = isProblem
    ? 'border-amber-200/80 bg-amber-50 text-amber-900'
    : 'border-emerald-200/80 bg-emerald-50 text-emerald-800';
  const counts = status?.counts;
  const warnings = status?.warnings ?? [];
  const errorCodeText = status?.error_code ? errorCodeHint[status.error_code] : '';
  const errorText = oodleUnavailable ? oodleUnavailableHint : errorCodeText || status?.error || status?.error_code || '';
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
            {showError && errorText && (
              <p className="mt-1 max-h-16 overflow-hidden text-[11px] font-semibold leading-5 text-rose-700" title={errorText}>
                错误：{errorText}
                {status?.error_code && <span className="ml-1 rounded bg-rose-100 px-1 py-0.5 font-mono text-[10px] font-bold text-rose-700">{status.error_code}</span>}
              </p>
            )}
            {status?.error_detail && <p className="mt-1 max-h-16 overflow-hidden text-[10px] font-medium leading-4 text-rose-600" title={status.error_detail}>详情：{status.error_detail}</p>}
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
