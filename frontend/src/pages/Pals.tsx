import React, { useEffect, useMemo, useState } from 'react';
import { AlertCircle, Hammer, HeartPulse, RefreshCw, Trash2 } from 'lucide-react';
import { palsApi } from '../api/pals';
import { useServerStore } from '../store/useServerStore';
import type { Pal } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

const suitabilityText: Record<string, string> = {
  Handiwork: '手工',
  Transport: '搬运',
  Watering: '浇水',
  Planting: '播种',
  Generating: '发电',
  Gathering: '采集',
  Lumbering: '伐木',
  Mining: '采矿',
  Cooling: '冷却',
  Farming: '牧场',
  Medicine: '制药',
};

export const Pals: React.FC = () => {
  const { refreshKey } = useServerStore();
  const [pals, setPals] = useState<Pal[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchText, setSearchText] = useState('');
  const [activeFilterTab, setActiveFilterTab] = useState('all');
  const [notice, setNotice] = useState<string | null>(null);

  const fetchPals = async () => {
    setLoading(true);
    const data = await palsApi.getPals();
    setPals(Array.isArray(data) ? data : []);
    setLoading(false);
  };

  useEffect(() => {
    fetchPals();
  }, [refreshKey]);

  const filteredPals = useMemo(() => {
    const keyword = searchText.toLowerCase();
    return pals.filter((pal) => {
      const matches =
        pal.name.toLowerCase().includes(keyword) || pal.owner_nickname.toLowerCase().includes(keyword);
      if (activeFilterTab === 'working') return matches && pal.status === 'Working';
      if (activeFilterTab === 'battling') return matches && pal.status === 'Battling';
      if (activeFilterTab === 'injured') return matches && pal.status === 'Injured';
      if (activeFilterTab === 'dead') return matches && pal.status === 'Dead';
      return matches;
    });
  }, [pals, searchText, activeFilterTab]);

  const unsupported = async (promise: Promise<{ message: string }>) => {
    const result = await promise;
    setNotice(result.message);
  };

  const headers = [
    { key: 'name', label: '帕鲁 / 稀有度' },
    { key: 'level', label: '等级' },
    { key: 'health', label: '生命值' },
    { key: 'suitability', label: '工作适应性' },
    { key: 'owner', label: '所属玩家' },
    { key: 'status', label: '状态' },
    { key: 'actions', label: '操作', align: 'center' as const },
  ];

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {notice && (
        <div className="rounded-2xl border border-amber-100 bg-amber-50 px-5 py-3 text-xs font-semibold text-amber-800">
          <AlertCircle className="mr-2 inline" size={14} />
          {notice}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Summary label="全部帕鲁" value={pals.length} tone="emerald" />
        <Summary label="工作中" value={pals.filter((pal) => pal.status === 'Working').length} tone="sky" />
        <Summary label="受伤" value={pals.filter((pal) => pal.status === 'Injured').length} tone="amber" />
        <Summary label="死亡" value={pals.filter((pal) => pal.status === 'Dead').length} tone="rose" />
      </div>

      <div className="rounded-2xl border border-slate-100 bg-slate-50 px-5 py-3 text-xs font-semibold text-slate-500">
        帕鲁列表当前使用后端 `/pals` 作为可选接口；后端未提供时显示演示数据，写操作不会模拟成功。
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading && pals.length === 0 ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在获取帕鲁数据...
          </div>
        ) : (
          <DataTable
            headers={headers}
            data={filteredPals}
            searchText={searchText}
            onSearchChange={setSearchText}
            searchPlaceholder="搜索帕鲁名称或所属玩家"
            tabs={[
              { id: 'all', label: '全部' },
              { id: 'working', label: '工作中' },
              { id: 'battling', label: '战斗中' },
              { id: 'injured', label: '受伤' },
              { id: 'dead', label: '死亡' },
            ]}
            activeTab={activeFilterTab}
            onTabChange={setActiveFilterTab}
            emptyText="暂无帕鲁"
            renderCard={(pal) => (
              <PalCard
                key={pal.id}
                pal={pal}
                onHeal={() => unsupported(palsApi.heal(pal.id))}
                onDelete={() => unsupported(palsApi.delete(pal.id))}
              />
            )}
            renderRow={(pal) => {
              const hpPercent = Math.min(100, Math.max(0, (pal.health / Math.max(1, pal.max_health)) * 100));
              return (
                <tr key={pal.id} className="hover:bg-slate-50/50">
                  <td className="px-6 py-4">
                    <PalIdentity pal={pal} />
                  </td>
                  <td className="px-6 py-4 text-xs font-bold text-slate-600">Lv.{pal.level}</td>
                  <td className="px-6 py-4">
                    <HealthBar pal={pal} hpPercent={hpPercent} />
                  </td>
                  <td className="px-6 py-4">
                    <Suitability pal={pal} />
                  </td>
                  <td className="px-6 py-4">
                    <div className="min-w-0">
                      <p className="truncate text-xs font-bold text-slate-600">{pal.owner_nickname}</p>
                      <p className="truncate font-mono text-[9px] text-slate-400">{pal.owner_steam_id}</p>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <StatusBadge status={pal.status} />
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex justify-center gap-2">
                      <button
                        type="button"
                        onClick={() => unsupported(palsApi.heal(pal.id))}
                        className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50"
                        aria-label="治疗帕鲁"
                      >
                        <HeartPulse size={14} />
                      </button>
                      <button
                        type="button"
                        onClick={() => unsupported(palsApi.delete(pal.id))}
                        className="rounded-lg border border-rose-200 p-2 text-rose-500 hover:bg-rose-50"
                        aria-label="释放帕鲁"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              );
            }}
          />
        )}
      </section>
    </div>
  );
};

const Summary: React.FC<{ label: string; value: number; tone: 'emerald' | 'sky' | 'amber' | 'rose' }> = ({
  label,
  value,
  tone,
}) => {
  const dot = {
    emerald: 'bg-emerald-500',
    sky: 'bg-sky-500',
    amber: 'bg-amber-500',
    rose: 'bg-rose-500',
  }[tone];
  return (
    <div className="flex items-center gap-3 rounded-2xl border border-slate-100 bg-white px-4 py-3 shadow-sm">
      <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      <span className="text-xs font-semibold text-slate-500">
        {label}: {value}
      </span>
    </div>
  );
};

const PalIdentity: React.FC<{ pal: Pal }> = ({ pal }) => (
  <div className="flex min-w-0 items-center gap-3">
    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl border border-slate-200/50 bg-slate-100 text-xs font-semibold text-slate-600">
      {pal.name.charAt(0)}
    </div>
    <div className="min-w-0">
      <p className="truncate text-xs font-bold text-slate-700">{pal.name}</p>
      <span className="mt-1 inline-flex rounded-md border border-slate-200 bg-slate-50 px-1.5 py-0.5 text-[9px] font-bold text-slate-500">
        {pal.rarity}
      </span>
    </div>
  </div>
);

const HealthBar: React.FC<{ pal: Pal; hpPercent: number }> = ({ pal, hpPercent }) => (
  <div className="flex w-32 max-w-full flex-col gap-1.5">
    <div className="flex justify-between text-[10px] font-bold text-slate-500">
      <span>
        {pal.health} / {pal.max_health}
      </span>
      <span>{hpPercent.toFixed(0)}%</span>
    </div>
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
      <div
        style={{ width: `${hpPercent}%` }}
        className={`h-full rounded-full ${pal.status === 'Dead' ? 'bg-slate-300' : hpPercent < 30 ? 'bg-rose-500' : hpPercent < 60 ? 'bg-amber-500' : 'bg-emerald-500'}`}
      />
    </div>
  </div>
);

const Suitability: React.FC<{ pal: Pal }> = ({ pal }) => (
  <div className="flex max-w-[220px] flex-wrap gap-1">
    {pal.work_suitability.map((work) => (
      <span
        key={`${work.type}-${work.level}`}
        className="inline-flex items-center gap-0.5 rounded-lg border border-slate-200/50 bg-slate-50 px-1.5 py-0.5 text-[9px] font-semibold text-slate-500"
      >
        <Hammer size={8} />
        {suitabilityText[work.type] || work.type} Lv.{work.level}
      </span>
    ))}
  </div>
);

const PalCard: React.FC<{ pal: Pal; onHeal: () => void; onDelete: () => void }> = ({ pal, onHeal, onDelete }) => {
  const hpPercent = Math.min(100, Math.max(0, (pal.health / Math.max(1, pal.max_health)) * 100));
  return (
    <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <PalIdentity pal={pal} />
        <StatusBadge status={pal.status} />
      </div>
      <div className="mt-4">
        <HealthBar pal={pal} hpPercent={hpPercent} />
      </div>
      <div className="mt-4">
        <Suitability pal={pal} />
      </div>
      <p className="mt-3 truncate text-[11px] font-semibold text-slate-500">所属玩家: {pal.owner_nickname}</p>
      <div className="mt-4 grid grid-cols-2 gap-2">
        <button type="button" onClick={onHeal} className="rounded-xl border border-slate-200 py-2 text-xs font-bold text-slate-600">
          治疗
        </button>
        <button type="button" onClick={onDelete} className="rounded-xl border border-rose-200 py-2 text-xs font-bold text-rose-600">
          释放
        </button>
      </div>
    </div>
  );
};
