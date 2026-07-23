import React, { useEffect, useState } from 'react';
import { LoaderCircle, MapPin, Users, X } from 'lucide-react';
import type { PalDefenderTeleportRequest } from '../../types';

export interface TeleportPlayerOption {
  id: string;
  name: string;
}

export const TeleportDialog: React.FC<{
  open: boolean;
  playerName: string;
  options: TeleportPlayerOption[];
  pending: boolean;
  onClose: () => void;
  onSubmit: (request: PalDefenderTeleportRequest) => Promise<boolean>;
}> = ({ open, playerName, options, pending, onClose, onSubmit }) => {
  const [mode, setMode] = useState<'coordinates' | 'player'>('coordinates');
  const [x, setX] = useState('');
  const [y, setY] = useState('');
  const [z, setZ] = useState('');
  const [targetPlayer, setTargetPlayer] = useState('');

  useEffect(() => {
    if (!open) return;
    setMode('coordinates');
    setX(''); setY(''); setZ(''); setTargetPlayer('');
  }, [open]);

  if (!open) return null;
  const coordinateValid = x.trim() !== '' && y.trim() !== '' && [x, y, z || '0'].every((value) => Number.isFinite(Number(value)));
  const canSubmit = mode === 'coordinates' ? coordinateValid : Boolean(targetPlayer);

  const submit = async () => {
    if (!canSubmit) return;
    const request: PalDefenderTeleportRequest = mode === 'coordinates'
      ? { Mode: 'coordinates', X: Number(x), Y: Number(y), ...(z.trim() ? { Z: Number(z) } : {}) }
      : { Mode: 'player', TargetPlayer: targetPlayer };
    if (await onSubmit(request)) onClose();
  };

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/45 p-4" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !pending) onClose(); }}>
      <section role="dialog" aria-modal="true" aria-labelledby="teleport-title" className="w-full max-w-lg rounded-2xl border border-slate-200 bg-white shadow-2xl">
        <header className="flex items-start justify-between gap-3 border-b border-slate-100 px-5 py-4">
          <div><h2 id="teleport-title" className="flex items-center gap-2 text-base font-black text-slate-900"><MapPin size={17} className="text-sky-500" />传送 {playerName}</h2><p className="mt-1 text-[11px] font-semibold text-slate-400">传送命令仅对当前在线玩家生效。</p></div>
          <button type="button" aria-label="关闭传送窗口" onClick={onClose} disabled={pending} className="text-slate-400 hover:text-slate-700 disabled:opacity-40"><X size={18} /></button>
        </header>
        <div className="space-y-4 p-5">
          <div className="grid grid-cols-2 gap-2 rounded-xl bg-slate-100 p-1">
            <button type="button" onClick={() => setMode('coordinates')} className={`rounded-lg px-3 py-2 text-xs font-bold ${mode === 'coordinates' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'}`}><MapPin size={13} className="mr-1.5 inline" />传送到坐标</button>
            <button type="button" onClick={() => setMode('player')} className={`rounded-lg px-3 py-2 text-xs font-bold ${mode === 'player' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'}`}><Users size={13} className="mr-1.5 inline" />传送到玩家</button>
          </div>
          {mode === 'coordinates' ? <div className="grid gap-3 sm:grid-cols-3">
            <CoordinateField label="X" value={x} onChange={setX} />
            <CoordinateField label="Y" value={y} onChange={setY} />
            <CoordinateField label="Z（可选）" value={z} onChange={setZ} />
          </div> : <label className="block text-xs font-bold text-slate-600">目标在线玩家<select aria-label="目标在线玩家" value={targetPlayer} onChange={(event) => setTargetPlayer(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold text-slate-700"><option value="">请选择</option>{options.map((option) => <option key={option.id} value={option.id}>{option.name} · {option.id}</option>)}</select>{options.length === 0 && <span className="mt-2 block text-[10px] font-semibold text-amber-700">当前没有其他在线玩家</span>}</label>}
        </div>
        <footer className="flex justify-end gap-2 border-t border-slate-100 px-5 py-4"><button type="button" onClick={onClose} disabled={pending} className="rounded-xl border border-slate-200 px-4 py-2.5 text-xs font-bold text-slate-600 disabled:opacity-40">取消</button><button type="button" onClick={() => void submit()} disabled={!canSubmit || pending} className="inline-flex items-center gap-2 rounded-xl bg-sky-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending && <LoaderCircle size={14} className="animate-spin" />}确认传送</button></footer>
      </section>
    </div>
  );
};

const CoordinateField: React.FC<{ label: string; value: string; onChange: (value: string) => void }> = ({ label, value, onChange }) => <label className="text-xs font-bold text-slate-600">{label}<input aria-label={label} type="number" value={value} onChange={(event) => onChange(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>;
