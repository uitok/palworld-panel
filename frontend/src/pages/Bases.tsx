import React, { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, Copy, MapPin, ShieldAlert, Sparkles, Users } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { basesApi } from '../api/bases';
import { saveIndexApi } from '../api/saveIndex';
import { useServerStore } from '../store/useServerStore';
import type { Base } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';
import { SaveIndexStatusBar } from '../components/ui/SaveIndexStatusBar';
import { SaveDataTabs } from '../components/ui/SaveDataTabs';
import { useDebouncedValue } from '../hooks/useDebouncedValue';

const pageSize = 50;

export const Bases: React.FC = () => {
  const { refreshKey } = useServerStore();
  const queryClient = useQueryClient();
  const [searchText, setSearchText] = useState('');
  const [page, setPage] = useState(1);
  const [notice, setNotice] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const debouncedSearch = useDebouncedValue(searchText, 250);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch]);

  const basesQuery = useQuery({
    queryKey: ['bases', { page, q: debouncedSearch, refreshKey }],
    queryFn: () =>
      basesApi.getBasesList({
        limit: pageSize,
        offset: (page - 1) * pageSize,
        q: debouncedSearch,
      }),
    placeholderData: (previous) => previous,
  });

  const rebuildMutation = useMutation({
    mutationFn: saveIndexApi.rebuild,
    onSuccess: () => {
      setNotice('已触发存档索引重建');
      setActionError(null);
      void queryClient.invalidateQueries({ queryKey: ['bases'] });
    },
    onError: (rebuildError) => {
      setNotice(null);
      setActionError(getErrorMessage(rebuildError));
    },
  });

  const bases = basesQuery.data?.items ?? [];
  const indexStatus = basesQuery.data?.status ?? null;
  const summary = basesQuery.data?.summary;
  const loading = basesQuery.isLoading;
  const error = actionError || (basesQuery.error ? getErrorMessage(basesQuery.error) : null);
  const totalItems = summary?.total ?? bases.length;
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));

  const unsupported = async (promise: Promise<{ message: string }>) => {
    const result = await promise;
    setNotice(result.message);
  };

  const copyCoords = async (base: Base) => {
    const cmd = `/teleportto ${base.x.toFixed(0)} ${base.y.toFixed(0)} ${base.z.toFixed(0)}`;
    await navigator.clipboard?.writeText(cmd);
    setNotice(`已复制传送指令：${cmd}`);
  };

  const headers = [
    { key: 'name', label: '基地 / 公会' },
    { key: 'coordinates', label: '世界坐标' },
    { key: 'structures', label: '建筑数' },
    { key: 'pals', label: '工作帕鲁' },
    { key: 'members', label: '在线成员' },
    { key: 'status', label: '防御状态' },
    { key: 'actions', label: '操作', align: 'center' as const },
  ];

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <SaveDataTabs />
      {notice && (
        <div className="rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3.5 text-xs font-semibold text-sky-700">
          <Sparkles size={16} className="mr-2 inline" />
          {notice}
        </div>
      )}
      {error && (
        <div className="rounded-2xl border border-rose-100 bg-rose-50 px-5 py-3.5 text-xs font-semibold text-rose-700">
          <AlertCircle size={16} className="mr-2 inline" />
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
        <Summary icon={<MapPin size={18} />} label="活跃基地总数" value={`${bases.length} 个`} />
        <Summary icon={<ShieldAlert size={18} />} label="遭袭基地" value={`${bases.filter((base) => base.status === 'Raid').length} 个`} danger={bases.some((base) => base.status === 'Raid')} />
        <Summary icon={<Users size={18} />} label="总建筑数" value={`${bases.reduce((acc, base) => acc + base.structures_count, 0)} 件`} />
      </div>

      <SaveIndexStatusBar
        status={indexStatus}
        loading={basesQuery.isFetching}
        rebuilding={rebuildMutation.isPending}
        onRefresh={() => void basesQuery.refetch()}
        onRebuild={() => rebuildMutation.mutate()}
      />

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading && bases.length === 0 ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <AlertCircle className="mr-2 inline text-sky-500" size={14} />
            正在获取基地数据...
          </div>
        ) : (
          <DataTable
            headers={headers}
            data={bases}
            searchText={searchText}
            onSearchChange={setSearchText}
            searchPlaceholder="搜索基地名称或所属公会"
            pagination={{
              currentPage: page,
              totalPages,
              totalItems,
              itemsPerPage: pageSize,
              onPageChange: setPage,
            }}
            virtualized
            emptyText={error ? '后端不可用或接口未实现' : '暂无基地'}
            renderCard={(base) => (
              <BaseCard
                key={base.id}
                base={base}
                onCopy={() => copyCoords(base)}
                onClean={() => unsupported(basesApi.cleanStructures(base.id))}
                onBackup={() => unsupported(basesApi.backupBase(base.id))}
              />
            )}
            renderRow={(base) => (
              <tr key={base.id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4">
                  <div className="min-w-0">
                    <p className="truncate text-xs font-bold text-slate-700">{base.name}</p>
                    <p className="mt-0.5 text-[10px] font-semibold text-slate-400">{base.guild_name}</p>
                  </div>
                </td>
                <td className="px-6 py-4 font-mono text-xs text-slate-500">
                  {base.x.toFixed(0)}, {base.y.toFixed(0)}, {base.z.toFixed(0)}
                </td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-600">{base.structures_count} 件</td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-600">{base.pals_count} / 20 只</td>
                <td className="px-6 py-4">
                  {base.online_members.length > 0 ? (
                    <div className="flex flex-wrap gap-1.5">
                      {base.online_members.map((member) => (
                        <span key={member} className="rounded-lg border border-sky-100 bg-sky-50 px-2 py-0.5 text-[10px] font-bold text-sky-700">
                          {member}
                        </span>
                      ))}
                    </div>
                  ) : (
                    <span className="text-[10px] font-semibold text-slate-400">无玩家在线</span>
                  )}
                </td>
                <td className="px-6 py-4">
                  <StatusBadge status={base.status} />
                </td>
                <td className="px-6 py-4">
                  <div className="flex justify-center gap-2">
                    <button type="button" onClick={() => copyCoords(base)} className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50" aria-label="复制坐标">
                      <Copy size={14} />
                    </button>
                    <button type="button" onClick={() => unsupported(basesApi.cleanStructures(base.id))} className="rounded-lg border border-slate-200 px-3 py-2 text-[10px] font-bold text-slate-500 hover:bg-slate-50">
                      清理
                    </button>
                  </div>
                </td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};

const Summary: React.FC<{ icon: React.ReactNode; label: string; value: string; danger?: boolean }> = ({
  icon,
  label,
  value,
  danger = false,
}) => (
  <div className="flex items-center justify-between rounded-2xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
    <div>
      <p className="text-[11px] font-semibold text-slate-400">{label}</p>
      <p className={`mt-1 text-2xl font-bold ${danger ? 'text-rose-500' : 'text-slate-800'}`}>{value}</p>
    </div>
    <div className={`flex h-9 w-9 items-center justify-center rounded-xl border ${danger ? 'border-rose-100 bg-rose-50 text-rose-500' : 'border-slate-100 bg-slate-50 text-slate-500'}`}>
      {icon}
    </div>
  </div>
);

const BaseCard: React.FC<{ base: Base; onCopy: () => void; onClean: () => void; onBackup: () => void }> = ({
  base,
  onCopy,
  onClean,
  onBackup,
}) => (
  <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-bold text-slate-800">{base.name}</p>
        <p className="mt-1 truncate text-[11px] font-semibold text-slate-400">{base.guild_name}</p>
      </div>
      <StatusBadge status={base.status} />
    </div>
    <div className="mt-4 grid grid-cols-2 gap-2 text-[11px] font-semibold text-slate-500">
      <span>建筑: {base.structures_count}</span>
      <span>帕鲁: {base.pals_count} / 20</span>
      <span className="col-span-2 font-mono">
        坐标: {base.x.toFixed(0)}, {base.y.toFixed(0)}, {base.z.toFixed(0)}
      </span>
      <span className="col-span-2 truncate">在线成员: {base.online_members.join(', ') || '无'}</span>
    </div>
    <div className="mt-4 grid grid-cols-3 gap-2">
      <button type="button" onClick={onCopy} className="rounded-xl border border-slate-200 py-2 text-xs font-bold text-slate-600">
        复制
      </button>
      <button type="button" onClick={onClean} className="rounded-xl border border-slate-200 py-2 text-xs font-bold text-slate-600">
        清理
      </button>
      <button type="button" onClick={onBackup} className="rounded-xl border border-slate-200 py-2 text-xs font-bold text-slate-600">
        备份
      </button>
    </div>
  </div>
);
