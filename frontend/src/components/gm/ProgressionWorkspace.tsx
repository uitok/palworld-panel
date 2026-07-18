import React, { useMemo, useState } from 'react';
import { BookOpen, Coins, LoaderCircle, RefreshCw, Sparkles, Undo2 } from 'lucide-react';
import { palDefenderGMApi } from '../../api/paldefenderGM';
import type { PalDefenderProgression, PalDefenderTechnologyCatalogEntry, PalDefenderTechs } from '../../types';

type ActionRunner = (key: string, action: () => Promise<unknown>, success: string) => Promise<boolean>;

export const ProgressionWorkspace: React.FC<{
  identifier: string;
  canWrite: boolean;
  available: boolean;
  busy: boolean;
  pending: string;
  progression?: PalDefenderProgression;
  techs?: PalDefenderTechs;
  catalog: PalDefenderTechnologyCatalogEntry[];
  runtimeTechnologyIDs: string[];
  loading: boolean;
  onRun: ActionRunner;
  onRefresh: () => Promise<void>;
}> = ({ identifier, canWrite, available, busy, pending, progression, techs, catalog, runtimeTechnologyIDs, loading, onRun, onRefresh }) => {
  const [exp, setExp] = useState('');
  const [technologyPoints, setTechnologyPoints] = useState('');
  const [ancientTechnologyPoints, setAncientTechnologyPoints] = useState('');
  const [technologyIDs, setTechnologyIDs] = useState('');
  const [technologySearch, setTechnologySearch] = useState('');

  const parsedTechnology = useMemo(() => {
    const values = technologyIDs.split(/[\s,，;；]+/).map((value) => value.trim()).filter(Boolean);
    return [...new Set(values)];
  }, [technologyIDs]);
  const runtimeSet = useMemo(() => new Set(runtimeTechnologyIDs.map((id) => id.toLowerCase())), [runtimeTechnologyIDs]);
  const unlockedSet = useMemo(() => new Set((techs?.Techs.Unlocked ?? []).map((id) => id.toLowerCase())), [techs?.Techs.Unlocked]);
  const filteredCatalog = useMemo(() => {
    const needle = technologySearch.trim().toLowerCase();
    return catalog.filter((entry) => !needle || [entry.id, entry.name, entry.category, String(entry.level)].some((value) => value.toLowerCase().includes(needle))).slice(0, 120);
  }, [catalog, technologySearch]);

  const toggleTechnology = (id: string) => {
    const current = parsedTechnology;
    setTechnologyIDs((current.some((value) => value.toLowerCase() === id.toLowerCase()) ? current.filter((value) => value.toLowerCase() !== id.toLowerCase()) : [...current, id]).join('\n'));
  };

  const grant = async () => {
    const request: { EXP?: number; TechnologyPoints?: number; AncientTechnologyPoints?: number } = {};
    const values = [
      ['EXP', exp],
      ['TechnologyPoints', technologyPoints],
      ['AncientTechnologyPoints', ancientTechnologyPoints],
    ] as const;
    for (const [key, raw] of values) {
      if (!raw.trim()) continue;
      const amount = Number(raw);
      if (!Number.isInteger(amount) || amount <= 0) return;
      request[key] = amount;
    }
    if (Object.keys(request).length === 0) return;
    await onRun('progression', async () => {
      await palDefenderGMApi.giveProgression(identifier, request);
      await onRefresh();
      setExp('');
      setTechnologyPoints('');
      setAncientTechnologyPoints('');
    }, '成长数值已发放');
  };

  const changeTechnology = async (mode: 'learn' | 'forget') => {
    if (parsedTechnology.length === 0) return;
    const selection = parsedTechnology.length === 1 ? parsedTechnology[0] : parsedTechnology;
    await onRun(`tech-${mode}`, async () => {
      if (mode === 'learn') await palDefenderGMApi.learnTech(identifier, { Technology: selection });
      else await palDefenderGMApi.forgetTech(identifier, { Technology: selection });
      await onRefresh();
    }, mode === 'learn' ? '科技已解锁' : '科技已遗忘');
  };

  const current = progression?.Progression;
  const disabled = !canWrite || !available || busy;
  return (
    <div className="grid gap-5 p-4 sm:p-5 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><Coins size={16} className="text-amber-500" />发放成长数值</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">填写的是增加量，不会把当前值覆盖成目标值。</p></div>
          <button type="button" onClick={() => void onRefresh()} disabled={loading} title="刷新成长数据" className="flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 text-slate-500 disabled:opacity-40"><RefreshCw size={13} className={loading ? 'animate-spin' : ''} /></button>
        </div>
        <div className="mt-4 grid gap-3 sm:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
          <NumberField label="经验值" value={exp} onChange={setExp} placeholder="例如 10000" />
          <NumberField label="普通科技点" value={technologyPoints} onChange={setTechnologyPoints} placeholder="例如 10" />
          <NumberField label="古代科技点" value={ancientTechnologyPoints} onChange={setAncientTechnologyPoints} placeholder="例如 5" />
        </div>
        <button type="button" onClick={() => void grant()} disabled={disabled || ![exp, technologyPoints, ancientTechnologyPoints].some((value) => Number(value) > 0)} className="mt-4 inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">
          {pending === 'progression' ? <LoaderCircle size={14} className="animate-spin" /> : <Sparkles size={14} />}确认发放
        </button>
        <div className="mt-5 grid gap-2 border-t border-slate-100 pt-4 sm:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
          <CurrentValue label="等级" value={current?.Player.level ?? 0} />
          <CurrentValue label="普通科技点" value={current?.Currencies.technologyPoints ?? 0} />
          <CurrentValue label="古代科技点" value={current?.Currencies.ancientTechnologyPoints ?? 0} />
        </div>
      </section>

      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        <div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><BookOpen size={16} className="text-violet-500" />科技解锁与遗忘</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">可用逗号、空格或换行分隔 TechID；输入 All 可处理全部科技。</p></div>
        <label className="relative mt-4 block"><span className="sr-only">搜索科技目录</span><input aria-label="搜索科技目录" value={technologySearch} onChange={(event) => setTechnologySearch(event.target.value)} placeholder="搜索中文名、TechID、等级或类别" className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-violet-400 focus:outline-none" /></label>
        <div className="mt-3 grid max-h-64 gap-2 overflow-y-auto pr-1 sm:grid-cols-2">
          {filteredCatalog.map((entry) => { const selected = parsedTechnology.some((id) => id.toLowerCase() === entry.id.toLowerCase()); const supported = runtimeSet.size === 0 || runtimeSet.has(entry.id.toLowerCase()); const unlocked = unlockedSet.has(entry.id.toLowerCase()); return <button type="button" key={entry.id} onClick={() => toggleTechnology(entry.id)} disabled={!supported} aria-pressed={selected} className={`flex items-start gap-3 rounded-xl border p-3 text-left ${selected ? 'border-violet-300 bg-violet-50' : 'border-slate-100 bg-slate-50/70'} disabled:opacity-45`}><span className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white ring-1 ring-slate-100">{entry.icon_url ? <img src={entry.icon_url} alt={`${entry.name}图标`} loading="lazy" referrerPolicy="no-referrer" onError={(event) => { event.currentTarget.style.display = 'none'; }} className="h-full w-full object-contain" /> : <BookOpen size={17} className="text-slate-300" />}</span><span className="min-w-0 flex-1"><span className="flex items-center justify-between gap-2"><span className="truncate text-xs font-bold text-slate-700">{entry.name}</span><span className="shrink-0 rounded-md bg-white px-1.5 py-0.5 text-[9px] font-bold text-slate-500">Lv.{entry.level}</span></span><span className="mt-1 block truncate font-mono text-[9px] text-slate-400">{entry.id}</span><span className="mt-2 flex flex-wrap gap-1.5"><span className="rounded-md bg-white px-1.5 py-0.5 text-[9px] font-bold text-slate-500">{entry.category || '科技'}</span>{entry.boss && <span className="rounded-md bg-amber-50 px-1.5 py-0.5 text-[9px] font-bold text-amber-700">古代</span>}{unlocked && <span className="rounded-md bg-emerald-50 px-1.5 py-0.5 text-[9px] font-bold text-emerald-700">已解锁</span>}{!supported && <span className="rounded-md bg-rose-50 px-1.5 py-0.5 text-[9px] font-bold text-rose-700">当前服务端不可用</span>}</span></span></button>; })}
          {filteredCatalog.length === 0 && <div className="col-span-full rounded-xl border border-dashed border-slate-200 px-4 py-8 text-center text-xs font-semibold text-slate-400">没有匹配的科技</div>}
        </div>
        <label className="mt-4 block text-xs font-bold text-slate-600">
          TechID
          <textarea aria-label="科技 ID" value={technologyIDs} onChange={(event) => setTechnologyIDs(event.target.value)} rows={5} placeholder={'Technology_1\nTechnology_2'} className="mt-1.5 w-full resize-y rounded-xl border border-slate-200 p-3 font-mono text-xs text-slate-700 focus:border-sky-500 focus:outline-none" />
        </label>
        <div className="mt-3 flex flex-wrap gap-2">
          <button type="button" onClick={() => void changeTechnology('learn')} disabled={disabled || parsedTechnology.length === 0} className="inline-flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'tech-learn' ? <LoaderCircle size={14} className="animate-spin" /> : <BookOpen size={14} />}解锁科技</button>
          <button type="button" onClick={() => void changeTechnology('forget')} disabled={disabled || parsedTechnology.length === 0} className="inline-flex items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-4 py-2.5 text-xs font-bold text-rose-700 disabled:opacity-40">{pending === 'tech-forget' ? <LoaderCircle size={14} className="animate-spin" /> : <Undo2 size={14} />}遗忘科技</button>
          <button type="button" onClick={() => setTechnologyIDs('All')} className="rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-bold text-slate-500">填入 All</button>
        </div>
        <div className="mt-5 border-t border-slate-100 pt-4">
          <div className="flex items-center justify-between gap-3"><p className="text-xs font-bold text-slate-600">已解锁科技</p><span className="text-[10px] font-bold text-slate-400">{techs?.Meta.UnlockedCount ?? 0} / {techs?.Meta.TotalCount ?? 0}</span></div>
          <div className="mt-3 flex max-h-44 flex-wrap gap-1.5 overflow-y-auto">
            {(techs?.Techs.Unlocked ?? []).length === 0 ? <span className="text-xs font-semibold text-slate-400">暂无数据</span> : techs?.Techs.Unlocked.map((tech) => <button type="button" key={tech} onClick={() => setTechnologyIDs(tech)} className="rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 font-mono text-[10px] text-slate-500 hover:border-violet-200 hover:text-violet-700">{tech}</button>)}
          </div>
        </div>
      </section>
    </div>
  );
};

const NumberField: React.FC<{ label: string; value: string; onChange: (value: string) => void; placeholder: string }> = ({ label, value, onChange, placeholder }) => (
  <label className="text-xs font-bold text-slate-600">{label}<input aria-label={label} type="number" min={1} step={1} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
);

const CurrentValue: React.FC<{ label: string; value: number }> = ({ label, value }) => (
  <div className="rounded-xl bg-slate-50 px-3 py-2.5"><p className="text-[9px] font-bold text-slate-400">当前{label}</p><p className="mt-1 text-sm font-black text-slate-700">{value.toLocaleString()}</p></div>
);
