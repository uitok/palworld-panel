import React, { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Download, ExternalLink, FileJson, LoaderCircle, RefreshCw, Save, Search, Send, Sword, Trash2, Upload } from 'lucide-react';
import { palDefenderGMApi } from '../../api/paldefenderGM';
import type { Pal, PalDefenderPalCatalogEntry, PalDefenderPalTemplate } from '../../types';
import { PalIcon } from './PalIcon';

type ActionRunner = (key: string, action: () => Promise<unknown>, success: string) => Promise<boolean>;

export const PalWorkspace: React.FC<{
  identifier: string;
  playerName: string;
  canWrite: boolean;
  available: boolean;
  busy: boolean;
  pending: string;
  savePals: Pal[];
  palCatalog: PalDefenderPalCatalogEntry[];
  onRun: ActionRunner;
  onRelease: (pal: Pal) => Promise<boolean>;
}> = ({ identifier, playerName, canWrite, available, busy, pending, savePals, palCatalog, onRun, onRelease }) => {
  const queryClient = useQueryClient();
  const [palID, setPalID] = useState('');
  const [palLevel, setPalLevel] = useState('1');
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [selectedExport, setSelectedExport] = useState('');
  const [editor, setEditor] = useState<TemplateEditor>(() => emptyTemplateEditor());
  const [palSearch, setPalSearch] = useState('');
  const [releaseTarget, setReleaseTarget] = useState<Pal | null>(null);
  const [releaseConfirmation, setReleaseConfirmation] = useState('');

  const filteredPalCatalog = palCatalog.filter((pal) => {
    const needle = palSearch.trim().toLowerCase();
    return !needle || pal.id.toLowerCase().includes(needle) || pal.name.toLowerCase().includes(needle);
  }).slice(0, 80);

  const palsQuery = useQuery({
    queryKey: ['paldefender-gm', 'pals', identifier],
    queryFn: () => palDefenderGMApi.pals(identifier),
    enabled: Boolean(available && identifier),
  });
  const templatesQuery = useQuery({
    queryKey: ['paldefender-gm', 'templates'],
    queryFn: palDefenderGMApi.templates,
    enabled: available,
  });
  const exportedQuery = useQuery({
    queryKey: ['paldefender-gm', 'exported-templates', identifier],
    queryFn: () => palDefenderGMApi.exportedPalTemplates(identifier),
    enabled: Boolean(identifier),
  });

  const directGrant = async () => {
    const level = Number(palLevel);
    if (!palID.trim() || !Number.isInteger(level) || level < 1 || level > 255) return;
    await onRun('give-pal', async () => {
      await palDefenderGMApi.givePals(identifier, { Pals: [{ PalID: palID.trim(), Level: level }] });
      await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'pals', identifier] });
    }, `已向 ${playerName} 发放 ${palID.trim()} Lv.${level}`);
  };

  const giveTemplate = async () => {
    if (!selectedTemplate) return;
    await onRun('give-template', async () => {
      await palDefenderGMApi.givePalTemplates(identifier, { PalTemplates: [selectedTemplate] });
      await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'pals', identifier] });
    }, `已向 ${playerName} 发放模板 ${selectedTemplate}`);
  };

  const exportPals = async () => {
    await onRun('export-pals', async () => {
      await palDefenderGMApi.exportPals(identifier);
      await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'exported-templates', identifier] });
    }, `已导出 ${playerName} 的帕鲁；请选择导出文件继续编辑`);
  };

  const loadManagedTemplate = async () => {
    if (!selectedTemplate) return;
    const template = await palDefenderGMApi.template(selectedTemplate);
    setEditor(editorFromTemplate(selectedTemplate, template));
  };

  const loadExportedTemplate = async () => {
    if (!selectedExport) return;
    const template = await palDefenderGMApi.exportedPalTemplate(identifier, selectedExport);
    const defaultName = selectedExport.replace(/\.json$/i, '').replace(/[^A-Za-z0-9_-]+/g, '_').slice(0, 64);
    setEditor(editorFromTemplate(defaultName || 'exported_pal', template));
  };

  const saveTemplate = async () => {
    const name = editor.name.trim();
    const template = templateFromEditor(editor);
    if (!name || !template.PalID) return;
    await onRun('save-template', async () => {
      await palDefenderGMApi.putTemplate(name, template);
      setSelectedTemplate(name.endsWith('.json') ? name : `${name}.json`);
      await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'templates'] });
    }, `模板 ${name} 已保存`);
  };

  const disabled = !canWrite || !available || busy;
  return (
    <div className="space-y-5 p-4 sm:p-5">
      <div className="grid gap-5 xl:grid-cols-2">
        <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
          <div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><Sword size={16} className="text-sky-500" />按 ID 发放帕鲁</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">适合普通帕鲁；需要 IV、魂强化、技能和被动时使用右侧模板。</p></div>
          <div className="mt-4 grid gap-3 sm:grid-cols-[minmax(0,1fr)_110px]">
            <label className="text-xs font-bold text-slate-600">PalID<input aria-label="帕鲁 ID" value={palID} onChange={(event) => setPalID(event.target.value)} placeholder="例如 Anubis" className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 font-mono text-xs text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
            <label className="text-xs font-bold text-slate-600">等级<input aria-label="帕鲁等级" type="number" min={1} max={255} value={palLevel} onChange={(event) => setPalLevel(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
          </div>
          <label className="relative mt-3 block"><Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" /><input aria-label="搜索帕鲁目录" value={palSearch} onChange={(event) => setPalSearch(event.target.value)} placeholder="搜索中文名或 PalID" className="w-full rounded-xl border border-slate-200 py-2.5 pl-9 pr-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>
          <div className="mt-3 grid max-h-56 grid-cols-2 gap-2 overflow-y-auto pr-1 sm:grid-cols-3">
            {filteredPalCatalog.map((pal) => <button type="button" key={pal.id} onClick={() => setPalID(pal.id)} aria-pressed={palID.toLowerCase() === pal.id.toLowerCase()} className={`flex min-w-0 items-center gap-2 rounded-xl border p-2 text-left ${palID.toLowerCase() === pal.id.toLowerCase() ? 'border-sky-300 bg-sky-50' : 'border-slate-100 bg-slate-50/70'}`}><PalIcon characterID={pal.id} name={pal.name} className="h-9 w-9 rounded-lg" /><span className="min-w-0 flex-1"><span className="block truncate text-[11px] font-bold text-slate-700">{pal.name}</span><span className="mt-1 block truncate font-mono text-[9px] text-slate-400">{pal.id}</span></span></button>)}
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-2">
            <button type="button" onClick={() => void directGrant()} disabled={disabled || !palID.trim()} className="inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'give-pal' ? <LoaderCircle size={14} className="animate-spin" /> : <Send size={14} />}发放帕鲁</button>
            <a href="https://paldeck.cc/pals" target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-xs font-bold text-sky-600">查询 PalID <ExternalLink size={12} /></a>
          </div>
          <div className="mt-5 grid grid-cols-3 gap-2 border-t border-slate-100 pt-4">
            <SmallCount label="队伍" value={palsQuery.data?.Meta.TeamCount ?? 0} />
            <SmallCount label="帕鲁终端" value={palsQuery.data?.Meta.PalboxCount ?? 0} />
            <SmallCount label="基地" value={palsQuery.data?.Meta.BaseCampCount ?? 0} />
          </div>
        </section>

        <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
          <div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><FileJson size={16} className="text-violet-500" />模板发放与导出</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">导出的现有帕鲁先读取到编辑器，再另存为可发放模板。</p></div>
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            <label className="text-xs font-bold text-slate-600">已保存模板<select aria-label="已保存模板" value={selectedTemplate} onChange={(event) => setSelectedTemplate(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold text-slate-700"><option value="">请选择</option>{(templatesQuery.data?.templates ?? []).map((template) => <option key={template.name} value={template.name}>{template.name}</option>)}</select></label>
            <label className="text-xs font-bold text-slate-600">导出文件<select aria-label="导出帕鲁模板" value={selectedExport} onChange={(event) => setSelectedExport(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold text-slate-700"><option value="">请选择</option>{(exportedQuery.data?.templates ?? []).map((template) => <option key={template.name} value={template.name}>{template.name}</option>)}</select></label>
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            <button type="button" onClick={() => void giveTemplate()} disabled={disabled || !selectedTemplate} className="inline-flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'give-template' ? <LoaderCircle size={14} className="animate-spin" /> : <Upload size={14} />}发放模板</button>
            <button type="button" onClick={() => void loadManagedTemplate()} disabled={!selectedTemplate} className="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-bold text-slate-600 disabled:opacity-40"><FileJson size={14} />编辑模板</button>
            <button type="button" onClick={() => void exportPals()} disabled={!canWrite || busy || !identifier} className="inline-flex items-center gap-2 rounded-xl border border-sky-200 bg-sky-50 px-3 py-2.5 text-xs font-bold text-sky-700 disabled:opacity-40">{pending === 'export-pals' ? <LoaderCircle size={14} className="animate-spin" /> : <Download size={14} />}导出玩家帕鲁</button>
            <button type="button" onClick={() => void loadExportedTemplate()} disabled={!selectedExport} className="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-bold text-slate-600 disabled:opacity-40"><RefreshCw size={14} />载入导出文件</button>
          </div>
        </section>
      </div>

      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between"><div><h3 className="flex items-center gap-2 text-sm font-bold text-slate-800"><Save size={16} className="text-emerald-500" />PalTemplate 属性编辑器</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">保存会写入 PalDefender/Pals/Templates，不会直接覆盖玩家已有帕鲁。</p></div><a href="https://paldeck.cc/creator" target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-xs font-bold text-sky-600">打开模板生成器 <ExternalLink size={12} /></a></div>
        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <TextField label="模板名称" value={editor.name} onChange={(value) => setEditor({ ...editor, name: value })} placeholder="reward_anubis" />
          <TextField label="PalID" value={editor.palID} onChange={(value) => setEditor({ ...editor, palID: value })} placeholder="Anubis" />
          <TextField label="昵称" value={editor.nickname} onChange={(value) => setEditor({ ...editor, nickname: value })} placeholder="可选" />
          <label className="text-xs font-bold text-slate-600">性别<select aria-label="性别" value={editor.gender} onChange={(event) => setEditor({ ...editor, gender: event.target.value })} className="mt-1.5 w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold text-slate-700"><option value="">默认</option><option value="Male">Male</option><option value="Female">Female</option><option value="None">None</option></select></label>
          <NumberEditor label="等级" value={editor.level} onChange={(value) => setEditor({ ...editor, level: value })} />
          <NumberEditor label="伙伴技能等级" value={editor.partnerSkillLevel} onChange={(value) => setEditor({ ...editor, partnerSkillLevel: value })} />
          <NumberEditor label="IV 生命" value={editor.ivHealth} onChange={(value) => setEditor({ ...editor, ivHealth: value })} />
          <NumberEditor label="IV 远程攻击" value={editor.ivAttackShot} onChange={(value) => setEditor({ ...editor, ivAttackShot: value })} />
          <NumberEditor label="IV 防御" value={editor.ivDefense} onChange={(value) => setEditor({ ...editor, ivDefense: value })} />
          <NumberEditor label="魂强化 生命" value={editor.soulHealth} onChange={(value) => setEditor({ ...editor, soulHealth: value })} />
          <NumberEditor label="魂强化 攻击" value={editor.soulAttack} onChange={(value) => setEditor({ ...editor, soulAttack: value })} />
          <NumberEditor label="魂强化 防御" value={editor.soulDefense} onChange={(value) => setEditor({ ...editor, soulDefense: value })} />
        </div>
        <div className="mt-3 grid gap-3 lg:grid-cols-2">
          <TextArea label="被动词条" value={editor.passives} onChange={(value) => setEditor({ ...editor, passives: value })} placeholder="Legend, CraftSpeed_up3" />
          <TextArea label="主动技能（最多 3 个）" value={editor.activeSkills} onChange={(value) => setEditor({ ...editor, activeSkills: value })} placeholder="SandTornado, RockLance" />
        </div>
        <button type="button" onClick={() => void saveTemplate()} disabled={disabled || !editor.name.trim() || !editor.palID.trim()} className="mt-4 inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'save-template' ? <LoaderCircle size={14} className="animate-spin" /> : <Save size={14} />}保存模板</button>
      </section>

      <section className="rounded-2xl border border-slate-100 bg-white">
        <div className="border-b border-slate-100 px-4 py-3"><h3 className="text-sm font-bold text-slate-800">存档解析中的玩家帕鲁</h3><p className="mt-1 text-[11px] font-semibold text-slate-400">图标来自参考项目的公开资源地址；加载失败时自动使用本地图标。</p></div>
        {savePals.length === 0 ? <div className="px-4 py-10 text-center text-xs font-semibold text-slate-400">没有匹配的存档帕鲁</div> : <div className="grid gap-2 p-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{savePals.map((pal) => <div key={pal.id} className="flex min-w-0 items-center gap-3 rounded-xl border border-slate-100 bg-slate-50/70 p-3"><button type="button" onClick={() => setPalID(pal.character_id || '')} className="flex min-w-0 flex-1 items-center gap-3 text-left"><PalIcon characterID={pal.character_id} name={pal.name} className="h-12 w-12 rounded-xl" /><span className="min-w-0 flex-1"><span className="block truncate text-xs font-bold text-slate-700">{pal.nickname || pal.name}</span><span className="block truncate font-mono text-[10px] text-slate-400">{pal.character_id || pal.id}</span><span className="mt-1 block text-[10px] font-bold text-slate-500">Lv.{pal.level} · {pal.gender || '未知性别'} · Rank {pal.rank ?? 0}</span></span></button><button type="button" aria-label={`放生 ${pal.nickname || pal.name}`} title="谨慎放生" onClick={() => { setReleaseTarget(pal); setReleaseConfirmation(''); }} disabled={!canWrite || !available || busy || !pal.character_id} className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-rose-200 bg-rose-50 text-rose-600 disabled:opacity-35"><Trash2 size={13} /></button></div>)}</div>}
      </section>
      {releaseTarget && <div className="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/50 p-4" role="presentation"><section role="dialog" aria-modal="true" aria-labelledby="release-pal-title" className="w-full max-w-md rounded-2xl bg-white p-5 shadow-2xl"><h3 id="release-pal-title" className="flex items-center gap-2 text-base font-black text-rose-700"><AlertTriangle size={17} />确认放生帕鲁</h3><p className="mt-3 text-xs font-semibold leading-5 text-slate-600">将通过 PalDefender 删除最多一只匹配帕鲁：<strong>{releaseTarget.nickname || releaseTarget.name}</strong>，{releaseTarget.character_id}，Lv.{releaseTarget.level}{releaseTarget.gender ? `，${releaseTarget.gender}` : ''}{releaseTarget.rank != null ? `，Rank ${releaseTarget.rank}` : ''}。</p><p className="mt-2 rounded-xl bg-amber-50 px-3 py-2.5 text-[11px] font-semibold leading-5 text-amber-800">PalDefender 按属性筛选而不是实例 ID；存在完全相同帕鲁时，可能删除其中任意一只。存档快照也可能暂时滞后。</p><label className="mt-4 block text-xs font-bold text-slate-600">输入玩家名称“{playerName}”确认<input aria-label="放生确认玩家名称" value={releaseConfirmation} onChange={(event) => setReleaseConfirmation(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold" /></label><div className="mt-5 flex justify-end gap-2"><button type="button" onClick={() => setReleaseTarget(null)} disabled={busy} className="rounded-xl border border-slate-200 px-4 py-2.5 text-xs font-bold text-slate-600 disabled:opacity-40">取消</button><button type="button" onClick={async () => { if (await onRelease(releaseTarget)) setReleaseTarget(null); }} disabled={busy || releaseConfirmation !== playerName} className="inline-flex items-center gap-2 rounded-xl bg-rose-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">{pending === 'release-pal' && <LoaderCircle size={14} className="animate-spin" />}确认放生</button></div></section></div>}
    </div>
  );
};

interface TemplateEditor {
  name: string; palID: string; nickname: string; gender: string; level: string; partnerSkillLevel: string;
  ivHealth: string; ivAttackShot: string; ivDefense: string; soulHealth: string; soulAttack: string; soulDefense: string;
  passives: string; activeSkills: string;
}

const emptyTemplateEditor = (): TemplateEditor => ({ name: '', palID: '', nickname: '', gender: '', level: '1', partnerSkillLevel: '1', ivHealth: '', ivAttackShot: '', ivDefense: '', soulHealth: '', soulAttack: '', soulDefense: '', passives: '', activeSkills: '' });
const textList = (value: string) => [...new Set(value.split(/[\n,，;；]+/).map((item) => item.trim()).filter(Boolean))];
const optionalNumber = (value: string) => value.trim() === '' ? undefined : Number(value);
const compactMap = (values: Record<string, string>) => Object.fromEntries(Object.entries(values).flatMap(([key, value]) => value.trim() === '' ? [] : [[key, Number(value)]]));

const templateFromEditor = (editor: TemplateEditor): PalDefenderPalTemplate => {
  const ivs = compactMap({ Health: editor.ivHealth, AttackShot: editor.ivAttackShot, Defense: editor.ivDefense });
  const souls = compactMap({ Health: editor.soulHealth, Attack: editor.soulAttack, Defense: editor.soulDefense });
  const template: PalDefenderPalTemplate = { PalID: editor.palID.trim() };
  if (editor.nickname.trim()) template.Nickname = editor.nickname.trim();
  if (editor.gender) template.Gender = editor.gender as 'Male' | 'Female' | 'None';
  if (optionalNumber(editor.level) !== undefined) template.Level = optionalNumber(editor.level);
  if (optionalNumber(editor.partnerSkillLevel) !== undefined) template.PartnerSkillLevel = optionalNumber(editor.partnerSkillLevel);
  if (Object.keys(ivs).length > 0) template.IVs = ivs;
  if (Object.keys(souls).length > 0) template.PalSouls = souls;
  if (textList(editor.passives).length > 0) template.Passives = textList(editor.passives);
  if (textList(editor.activeSkills).length > 0) template.ActiveSkills = textList(editor.activeSkills).slice(0, 3);
  return template;
};

const editorFromTemplate = (name: string, template: PalDefenderPalTemplate): TemplateEditor => ({
  name: name.replace(/\.json$/i, ''), palID: template.PalID || '', nickname: template.Nickname || '', gender: template.Gender || '',
  level: template.Level == null ? '' : String(template.Level), partnerSkillLevel: template.PartnerSkillLevel == null ? '' : String(template.PartnerSkillLevel),
  ivHealth: template.IVs?.Health == null ? '' : String(template.IVs.Health), ivAttackShot: template.IVs?.AttackShot == null ? '' : String(template.IVs.AttackShot), ivDefense: template.IVs?.Defense == null ? '' : String(template.IVs.Defense),
  soulHealth: template.PalSouls?.Health == null ? '' : String(template.PalSouls.Health), soulAttack: template.PalSouls?.Attack == null ? '' : String(template.PalSouls.Attack), soulDefense: template.PalSouls?.Defense == null ? '' : String(template.PalSouls.Defense),
  passives: (template.Passives ?? []).join(', '), activeSkills: (template.ActiveSkills ?? []).join(', '),
});

const SmallCount: React.FC<{ label: string; value: number }> = ({ label, value }) => <div className="rounded-xl bg-slate-50 px-3 py-2.5 text-center"><p className="text-[9px] font-bold text-slate-400">{label}</p><p className="mt-1 text-sm font-black text-slate-700">{value}</p></div>;
const TextField: React.FC<{ label: string; value: string; onChange: (value: string) => void; placeholder: string }> = ({ label, value, onChange, placeholder }) => <label className="text-xs font-bold text-slate-600">{label}<input aria-label={label} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>;
const NumberEditor: React.FC<{ label: string; value: string; onChange: (value: string) => void }> = ({ label, value, onChange }) => <label className="text-xs font-bold text-slate-600">{label}<input aria-label={label} type="number" min={0} max={255} value={value} onChange={(event) => onChange(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none" /></label>;
const TextArea: React.FC<{ label: string; value: string; onChange: (value: string) => void; placeholder: string }> = ({ label, value, onChange, placeholder }) => <label className="text-xs font-bold text-slate-600">{label}<textarea aria-label={label} rows={3} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="mt-1.5 w-full resize-y rounded-xl border border-slate-200 p-3 font-mono text-xs text-slate-700 focus:border-sky-500 focus:outline-none" /></label>;
