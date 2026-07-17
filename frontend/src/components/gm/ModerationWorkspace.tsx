import React from 'react';
import { Ban, CircleOff, LoaderCircle, LogOut } from 'lucide-react';

export const ModerationWorkspace: React.FC<{
  reason: string; onReasonChange: (reason: string) => void; banIP: boolean; onBanIPChange: (enabled: boolean) => void;
  canWrite: boolean; busy: boolean; online: boolean; pending: string; onAction: (action: 'kick' | 'ban' | 'unban') => void;
}> = ({ reason, onReasonChange, banIP, onBanIPChange, canWrite, busy, online, pending, onAction }) => (
  <section className="border-t border-slate-100 p-4 sm:p-5">
    <h3 className="text-sm font-bold text-slate-800">处罚与解除</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">踢出需要玩家在线；封禁和解除封禁可按当前玩家标识提交。</p>
    <label className="mt-4 flex max-w-3xl flex-col gap-1.5 text-xs font-bold text-slate-600">操作原因<textarea aria-label="操作原因" value={reason} onChange={(event) => onReasonChange(event.target.value)} maxLength={1024} rows={4} className="resize-y rounded-xl border border-slate-200 p-3 text-sm font-medium text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
    <label className="mt-3 flex w-fit items-center gap-2 text-xs font-bold text-slate-600"><input aria-label="同时封禁 IP" type="checkbox" checked={banIP} onChange={(event) => onBanIPChange(event.target.checked)} className="h-4 w-4 accent-rose-600" />同时封禁 IP</label>
    <div className="mt-5 flex flex-wrap gap-2">
      <button type="button" onClick={() => onAction('kick')} disabled={!canWrite || busy || !online} className="inline-flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs font-bold text-amber-800 disabled:opacity-40">{pending === 'kick' ? <LoaderCircle size={14} className="animate-spin" /> : <LogOut size={14} />}踢出</button>
      <button type="button" onClick={() => onAction('ban')} disabled={!canWrite || busy} className="inline-flex items-center gap-2 rounded-xl bg-rose-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'ban' ? <LoaderCircle size={14} className="animate-spin" /> : <Ban size={14} />}封禁</button>
      <button type="button" onClick={() => onAction('unban')} disabled={!canWrite || busy} className="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2.5 text-xs font-bold text-slate-600 disabled:opacity-40">{pending === 'unban' ? <LoaderCircle size={14} className="animate-spin" /> : <CircleOff size={14} />}解除封禁</button>
    </div>
  </section>
);
