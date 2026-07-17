import React, { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Dna, Egg, FlaskConical, GitBranch, LoaderCircle, Pause, Play, RotateCcw, Search, Sparkles, Square, Timer, Trees } from 'lucide-react';
import { breedingApi, type BreedSessionPrincipal, type BreedingAccess, type BreedingSubmitInput } from '../api/breeding';
import { getErrorMessage } from '../api/client';
import { saveIndexApi } from '../api/saveIndex';
import type { BreedingSolveResult, BreedingTreeNode, Job } from '../types';

const defaultSettings: BreedingSubmitInput['settings'] = {
  max_breeding_steps: 6, max_solver_iterations: 20, max_wild_pals: 1,
  max_input_irrelevant_passives: 2, max_bred_irrelevant_passives: 1,
  max_threads: 0, max_gold_cost: 0, use_gender_reversers: false,
};

const defaultGameSettings: BreedingSubmitInput['game_settings'] = {
  breeding_time_seconds: 300, massive_egg_incubation_minutes: 120,
  multiple_breeding_farms: true, multiple_incubators: true,
};

interface BreedingLabProps {
  access?: BreedingAccess;
  principal?: BreedSessionPrincipal | null;
  quickQuery?: string;
  onBalanceChange?: (balance: number) => void;
}

interface QueuedTarget {
  id: string;
  palId: string;
  gender: string;
  required: string[];
  optional: string[];
  ivs: { hp: number; attack: number; defense: number };
}

export const BreedingLab: React.FC<BreedingLabProps> = ({ access = 'admin', principal, quickQuery = '', onBalanceChange }) => {
  const restricted = access === 'session';
  const catalog = useQuery({ queryKey: ['breeding-catalog', access], queryFn: () => breedingApi.catalog(access), retry: false });
  const customContainers = useQuery({ queryKey: ['breeding-custom-containers', access], queryFn: () => breedingApi.customContainers(access), retry: false });
  const presets = useQuery({ queryKey: ['breeding-presets', access], queryFn: () => breedingApi.presets(access), retry: false });
  const saveStatus = useQuery({ queryKey: ['save-index-status'], queryFn: saveIndexApi.getStatus, enabled: !restricted });
  const [target, setTarget] = useState('');
  const [targetQueue, setTargetQueue] = useState<QueuedTarget[]>([]);
  const [gender, setGender] = useState('wildcard');
  const [owner, setOwner] = useState('');
  const [selectedContainers, setSelectedContainers] = useState<string[]>([]);
  const [required, setRequired] = useState<string[]>([]);
  const [optional, setOptional] = useState<string[]>([]);
  const [ivs, setIvs] = useState({ hp: 0, attack: 0, defense: 0 });
  const [settings, setSettings] = useState(defaultSettings);
  const [allowSurgery, setAllowSurgery] = useState(false);
  const [gameSettings, setGameSettings] = useState(defaultGameSettings);
  const [job, setJob] = useState<Job | null>(null);
  const [result, setResult] = useState<BreedingSolveResult[]>([]);
  const [resultStale, setResultStale] = useState(false);
  const [selected, setSelected] = useState(0);
  const [error, setError] = useState('');
  const [passiveQuery, setPassiveQuery] = useState('');

  useEffect(() => {
    if (!quickQuery.trim() || target || !catalog.data) return;
    const parts = quickQuery.trim().split(/\s+/);
    const requestedTarget = parts.shift()?.toLowerCase() || '';
    const pal = catalog.data.pals.find((item) => item.id.toLowerCase() === requestedTarget || item.name.toLowerCase() === requestedTarget);
    if (!pal) return;
    setTarget(pal.id);
    const requestedPassives = parts
      .map((value) => catalog.data?.passives.find((item) => item.id.toLowerCase() === value.toLowerCase() || item.name.toLowerCase() === value.toLowerCase())?.id)
      .filter((value): value is string => Boolean(value))
      .slice(0, 4);
    setRequired(requestedPassives);
  }, [catalog.data, quickQuery, target]);

  const passives = useMemo(() => (catalog.data?.passives || []).filter((passive) => `${passive.name} ${passive.id}`.toLowerCase().includes(passiveQuery.toLowerCase())).slice(0, 80), [catalog.data?.passives, passiveQuery]);
  const mutation = useMutation({
    mutationFn: async () => {
      const targets = targetQueue.length ? targetQueue : [{ id: 'current', palId: target, gender, required, optional, ivs }];
      const combined: BreedingSolveResult[] = [];
      setResult([]); setResultStale(false); setError('');
      for (const queued of targets) {
        const submitted = await breedingApi.submit({
          owner_player_uid: restricted ? undefined : owner.trim() || undefined,
          custom_container_ids: selectedContainers,
          target: { pal_id: queued.palId, gender: queued.gender, required_passives: queued.required, optional_passives: queued.optional, iv_hp: queued.ivs.hp, iv_attack: queued.ivs.attack, iv_defense: queued.ivs.defense },
          settings: { ...settings, allowed_surgery_passives: allowSurgery ? queued.required : [] },
          game_settings: gameSettings,
          result_limit: 20,
        }, access);
        const created = submitted.job;
        if (restricted) onBalanceChange?.(submitted.balance);
        setJob(created);
        const completed = await breedingApi.waitForJob(created.id, access, setJob);
        if (completed.status !== 'success') throw new Error(completed.error || '配种求解失败');
        const response = await breedingApi.result(created.id, access);
        setResultStale((current) => current || Boolean(response.stale));
        combined.push(...(response.result?.results || []));
        setResult([...combined]);
      }
      return combined;
    },
    onSuccess: (items) => { setResult(items); setSelected(0); },
    onError: (submitError) => setError(getErrorMessage(submitError)),
  });

  const togglePassive = (id: string, mode: 'required' | 'optional') => {
    const setter = mode === 'required' ? setRequired : setOptional;
    const current = mode === 'required' ? required : optional;
    setter(current.includes(id) ? current.filter((value) => value !== id) : current.length < 4 ? [...current, id] : current);
  };
  const queueTarget = () => {
    if (!target) return;
    setTargetQueue((current) => [...current, { id: `${target}-${Date.now()}`, palId: target, gender, required: [...required], optional: [...optional], ivs: { ...ivs } }]);
    setTarget(''); setRequired([]); setOptional([]); setIvs({ hp: 0, attack: 0, defense: 0 });
  };
  const applyPreset = (presetId: string) => {
    const config = presets.data?.find((item) => item.id === presetId)?.config;
    if (!config) return;
    if (config.settings && typeof config.settings === 'object') setSettings({ ...defaultSettings, ...(config.settings as Partial<BreedingSubmitInput['settings']>) });
    if (config.game_settings && typeof config.game_settings === 'object') setGameSettings({ ...defaultGameSettings, ...(config.game_settings as Partial<BreedingSubmitInput['game_settings']>) });
    if (Array.isArray(config.custom_container_ids)) setSelectedContainers(config.custom_container_ids.map(String));
    if (typeof config.allow_surgery === 'boolean') setAllowSurgery(config.allow_surgery);
  };
  const savePreset = async () => {
    const name = window.prompt('预设名称');
    if (!name?.trim()) return;
    try {
      await breedingApi.savePreset(access, name.trim(), { settings, game_settings: gameSettings, custom_container_ids: selectedContainers, allow_surgery: allowSurgery });
      await presets.refetch();
    } catch (reason) {
      setError(getErrorMessage(reason));
    }
  };
  const activeResult = result[selected];
  const running = Boolean(job && !['success', 'failed'].includes(job.status));

  return (
    <div className="page-shell breeding-page">
      <div className="page-titlebar"><div><p className="eyebrow">PalCalc v1.17.6</p><h1>配种实验室</h1><p>{restricted ? '仅使用你绑定角色名下的帕鲁进行计算，无法查看其他玩家或服务器路径。' : '直接使用当前存档中的帕鲁，寻找满足目标词条和 IV 的最优配种路线。'}</p></div><span className={`state-pill ${restricted || saveStatus.data?.state === 'ready' ? 'ok' : 'warn'}`}>{restricted ? `已绑定 ${principal?.nickname || principal?.player_uid || '-'}` : `存档 ${saveStatus.data?.state || 'unknown'}`}</span></div>
      <section className="status-strip breeding-status"><Metric icon={<DatabaseIcon />} label={restricted ? '绑定角色' : '可用帕鲁'} value={restricted ? (principal?.nickname || '-') : (saveStatus.data?.counts.pals || 0)} /><Metric icon={<Dna size={16} />} label="数据库" value={catalog.data?.version || '-'} /><Metric icon={<Timer size={16} />} label="任务" value={job?.status || 'idle'} /><Metric icon={<Sparkles size={16} />} label={restricted ? '积分' : '结果'} value={restricted ? (principal?.balance ?? '-') : result.length} /></section>
      {error && <div className="pp-notice danger">{error}</div>}

      <div className="breeding-workspace">
        <section className="pp-card target-panel">
          <div className="pp-card-head"><div><h2>目标与来源</h2><p>创建一个求解目标。</p></div><FlaskConical size={18} /></div>
          <div className="target-queue"><div><strong>目标队列</strong><span>{targetQueue.length || 1} 个计算目标</span></div>{targetQueue.length > 0 ? targetQueue.map((queued, index) => <article key={queued.id}><span>{index + 1}</span><div><strong>{catalog.data?.pals.find((pal) => pal.id === queued.palId)?.name || queued.palId}</strong><small>{queued.required.length ? queued.required.map((id) => catalog.data?.passives.find((passive) => passive.id === id)?.name || id).join(' · ') : '无必需词条'}</small></div><button type="button" aria-label={`移除目标 ${index + 1}`} onClick={() => setTargetQueue((current) => current.filter((item) => item.id !== queued.id))}>×</button></article>) : <p>当前编辑内容将作为单个目标；加入队列后可连续求解多个目标。</p>}</div>
          <label className="field-label">目标帕鲁<select value={target} onChange={(event) => setTarget(event.target.value)}><option value="">请选择</option>{catalog.data?.pals.map((pal) => <option key={pal.id} value={pal.id}>{pal.name} · {pal.id}</option>)}</select></label>
          <label className="field-label">目标性别<select value={gender} onChange={(event) => setGender(event.target.value)}><option value="wildcard">不限</option><option value="male">雄性</option><option value="female">雌性</option></select></label>
          {!restricted && <label className="field-label">限定玩家 UID<input value={owner} onChange={(event) => setOwner(event.target.value)} placeholder="留空表示管理员使用全部帕鲁" /></label>}
          {(customContainers.data?.length || 0) > 0 && <fieldset className="custom-container-picker"><legend>自定义帕鲁容器</legend>{customContainers.data?.map((container) => <label key={container.id}><input type="checkbox" checked={selectedContainers.includes(container.id)} onChange={() => setSelectedContainers((current) => current.includes(container.id) ? current.filter((id) => id !== container.id) : [...current, container.id])} /><span>{container.name}</span><small>{container.pals.length} 只</small></label>)}</fieldset>}
          <div className="iv-grid">{(['hp', 'attack', 'defense'] as const).map((key) => <label key={key}><span>{key === 'hp' ? '生命 IV' : key === 'attack' ? '攻击 IV' : '防御 IV'}</span><input type="number" min={0} max={100} value={ivs[key]} onChange={(event) => setIvs({ ...ivs, [key]: Number(event.target.value) })} /></label>)}</div>
          <button type="button" className="pp-button wide" disabled={!target} onClick={queueTarget}>加入目标队列</button>
        </section>

        <section className="pp-card passives-panel">
          <div className="pp-card-head"><div><h2>目标词条</h2><p>必需词条必须全部满足，可选词条用于比较候选路线。</p></div><Search size={18} /></div>
          <input className="panel-search" value={passiveQuery} onChange={(event) => setPassiveQuery(event.target.value)} placeholder="搜索被动词条" />
          <div className="selected-passives"><PassiveGroup title="必需" ids={required} catalog={catalog.data?.passives || []} onRemove={(id) => togglePassive(id, 'required')} /><PassiveGroup title="可选" ids={optional} catalog={catalog.data?.passives || []} onRemove={(id) => togglePassive(id, 'optional')} /></div>
          <div className="passive-list">{passives.map((passive) => <div key={passive.id} className="passive-row"><span><strong>{passive.name}</strong><small>{passive.id}</small></span><button type="button" className={required.includes(passive.id) ? 'on' : ''} onClick={() => togglePassive(passive.id, 'required')}>必需</button><button type="button" className={optional.includes(passive.id) ? 'on optional' : ''} onClick={() => togglePassive(passive.id, 'optional')}>可选</button></div>)}</div>
        </section>

        <section className="pp-card settings-panel">
          <div className="pp-card-head"><div><h2>高级设置</h2><p>控制搜索规模和游戏机制。</p></div><Trees size={18} /></div>
          <div className="preset-controls"><select aria-label="配种设置预设" defaultValue="" onChange={(event) => applyPreset(event.target.value)}><option value="">选择设置预设</option>{presets.data?.map((preset) => <option key={preset.id} value={preset.id}>{preset.name}</option>)}</select><button type="button" className="pp-button" onClick={() => void savePreset()}>保存预设</button></div>
          <NumberSetting label="最大配种步骤" value={settings.max_breeding_steps} onChange={(value) => setSettings({ ...settings, max_breeding_steps: value })} />
          <NumberSetting label="最大迭代" value={settings.max_solver_iterations} onChange={(value) => setSettings({ ...settings, max_solver_iterations: value })} />
          <NumberSetting label="允许野生帕鲁" value={settings.max_wild_pals} onChange={(value) => setSettings({ ...settings, max_wild_pals: value })} />
          <NumberSetting label="输入无关词条" value={settings.max_input_irrelevant_passives} onChange={(value) => setSettings({ ...settings, max_input_irrelevant_passives: value })} />
          <NumberSetting label="后代无关词条" value={settings.max_bred_irrelevant_passives} onChange={(value) => setSettings({ ...settings, max_bred_irrelevant_passives: value })} />
          <NumberSetting label="求解线程数（0 为自动）" value={settings.max_threads} onChange={(value) => setSettings({ ...settings, max_threads: value })} />
          <NumberSetting label="手术金币上限" value={settings.max_gold_cost} onChange={(value) => setSettings({ ...settings, max_gold_cost: value })} />
          <label className="toggle-setting"><span><strong>允许必需词条手术</strong><small>仅允许对当前目标的必需词条使用手术</small></span><input type="checkbox" checked={allowSurgery} onChange={(event) => setAllowSurgery(event.target.checked)} /></label>
          <label className="toggle-setting"><span><strong>允许性别反转</strong><small>使用 PalCalc 性别反转机制</small></span><input type="checkbox" checked={settings.use_gender_reversers} onChange={(event) => setSettings({ ...settings, use_gender_reversers: event.target.checked })} /></label>
          <NumberSetting label="配种时间（秒）" value={gameSettings.breeding_time_seconds} onChange={(value) => setGameSettings({ ...gameSettings, breeding_time_seconds: value })} />
          <NumberSetting label="巨大蛋孵化（分钟）" value={gameSettings.massive_egg_incubation_minutes} onChange={(value) => setGameSettings({ ...gameSettings, massive_egg_incubation_minutes: value })} />
          <label className="toggle-setting"><span><strong>多个配种牧场</strong><small>允许路线阶段并行配种</small></span><input type="checkbox" checked={gameSettings.multiple_breeding_farms} onChange={(event) => setGameSettings({ ...gameSettings, multiple_breeding_farms: event.target.checked })} /></label>
          <label className="toggle-setting"><span><strong>多个孵化器</strong><small>允许多枚蛋并行孵化</small></span><input type="checkbox" checked={gameSettings.multiple_incubators} onChange={(event) => setGameSettings({ ...gameSettings, multiple_incubators: event.target.checked })} /></label>
          <button type="button" className="pp-button accent wide solve-button" disabled={(!target && !targetQueue.length) || mutation.isPending || (!restricted && saveStatus.data?.state !== 'ready')} onClick={() => mutation.mutate()}>{mutation.isPending ? <LoaderCircle className="animate-spin" size={16} /> : <Play size={16} />}开始计算{targetQueue.length > 1 ? `（${targetQueue.length} 个目标${restricted ? `，${targetQueue.length} 积分` : ''}）` : restricted ? '（1 积分）' : ''}</button>
          {job && <div className="job-progress"><span><strong>{job.message || job.status}</strong><small>{job.progress}%</small></span><div><i style={{ width: `${job.progress}%` }} /></div>{running && <div className="job-actions"><button type="button" onClick={() => void breedingApi.pause(job.id, access)}><Pause size={14} />暂停</button><button type="button" onClick={() => void breedingApi.resume(job.id, access)}><RotateCcw size={14} />恢复</button><button type="button" onClick={() => void breedingApi.cancel(job.id, access)}><Square size={14} />取消</button></div>}</div>}
        </section>
      </div>

      <section className="pp-card result-card">
        <div className="pp-card-head"><div><h2>候选路线</h2><p>{result.length ? `找到 ${result.length} 条路线，默认按预计时间排序。` : '完成求解后在这里查看配种树。'}</p></div><GitBranch size={18} /></div>
        {resultStale && <div className="pp-notice warn">当前存档已发生变化，这条路线可能包含已移动或消失的帕鲁，建议重新计算。</div>}
        {result.length > 0 ? <div className="result-layout"><div className="result-list">{result.map((item, index) => <button type="button" key={`${item.pal_id}-${index}`} className={selected === index ? 'active' : ''} onClick={() => setSelected(index)}><strong>{item.pal_name}</strong><span><Egg size={13} />{item.eggs} 枚 · {item.breeding_steps} 步</span><small>{formatDuration(item.effort_seconds)}</small></button>)}</div><div className="tree-scroll">{activeResult && <TreeNode node={activeResult.tree} depth={0} />}</div></div> : <div className="empty-panel"><Dna size={30} /><strong>尚无配种结果</strong><span>选择目标帕鲁和词条后开始计算。</span></div>}
      </section>
    </div>
  );
};

const DatabaseIcon = () => <Dna size={16} />;
const Metric: React.FC<{ icon: React.ReactNode; label: string; value: React.ReactNode }> = ({ icon, label, value }) => <div className="metric"><span className="metric-label">{icon}{label}</span><strong>{value}</strong></div>;
const NumberSetting: React.FC<{ label: string; value: number; onChange: (value: number) => void }> = ({ label, value, onChange }) => <label className="number-setting"><span>{label}</span><input type="number" min={0} max={999999} value={value} onChange={(event) => onChange(Number(event.target.value))} /></label>;
const PassiveGroup: React.FC<{ title: string; ids: string[]; catalog: Array<{ id: string; name: string }>; onRemove: (id: string) => void }> = ({ title, ids, catalog, onRemove }) => <div><span>{title}</span><div>{ids.length ? ids.map((id) => <button type="button" key={id} onClick={() => onRemove(id)}>{catalog.find((item) => item.id === id)?.name || id} ×</button>) : <small>未选择</small>}</div></div>;
const formatDuration = (seconds: number) => { const hours = Math.floor(seconds / 3600); const minutes = Math.ceil((seconds % 3600) / 60); return hours ? `${hours} 小时 ${minutes} 分` : `${minutes} 分钟`; };

const TreeNode: React.FC<{ node: BreedingTreeNode; depth: number }> = ({ node, depth }) => <div className={`tree-branch depth-${Math.min(depth, 4)}`}><article className={`tree-node ${node.type}`}><div><span className="node-type">{node.type === 'owned' ? '已有' : node.type === 'wild' ? '野生' : node.type === 'surgery' ? '手术' : node.type === 'bred' ? '配种' : '组合'}</span><strong>{node.pal_name}</strong><small>{node.gender} · {node.location_type || '路线节点'}</small></div><div className="node-meta"><span>{node.passives.length ? node.passives.join(' · ') : '无目标词条'}</span>{node.eggs != null && <span>{node.eggs} 枚蛋</span>}{node.probability != null && <span>{(node.probability * 100).toFixed(1)}%</span>}</div></article>{node.children && <div className="tree-children">{node.children.map((child, index) => <TreeNode key={`${child.type}-${child.pal_id}-${index}`} node={child} depth={depth + 1} />)}</div>}</div>;
