import React, { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, RefreshCw, Users } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { guildsApi } from '../api/guilds';
import { saveIndexApi } from '../api/saveIndex';
import { useServerStore } from '../store/useServerStore';
import type { Guild } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { SaveIndexStatusBar } from '../components/ui/SaveIndexStatusBar';
import { useDebouncedValue } from '../hooks/useDebouncedValue';

const pageSize = 50;

export const Guilds: React.FC = () => {
  const { refreshKey } = useServerStore();
  const queryClient = useQueryClient();
  const [searchText, setSearchText] = useState('');
  const [page, setPage] = useState(1);
  const [actionError, setActionError] = useState<string | null>(null);
  const debouncedSearch = useDebouncedValue(searchText, 250);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch]);

  const guildsQuery = useQuery({
    queryKey: ['guilds', { page, q: debouncedSearch, refreshKey }],
    queryFn: () =>
      guildsApi.getGuildsList({
        limit: pageSize,
        offset: (page - 1) * pageSize,
        q: debouncedSearch,
      }),
    placeholderData: (previous) => previous,
  });

  const rebuildMutation = useMutation({
    mutationFn: saveIndexApi.rebuild,
    onSuccess: () => {
      setActionError(null);
      void queryClient.invalidateQueries({ queryKey: ['guilds'] });
    },
    onError: (rebuildError) => {
      setActionError(getErrorMessage(rebuildError));
    },
  });

  const guilds = guildsQuery.data?.items ?? [];
  const indexStatus = guildsQuery.data?.status ?? null;
  const summary = guildsQuery.data?.summary;
  const loading = guildsQuery.isLoading;
  const error = actionError || (guildsQuery.error ? getErrorMessage(guildsQuery.error) : null);
  const totalItems = summary?.total ?? guilds.length;
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {error && (
        <div className="rounded-2xl border border-rose-100 bg-rose-50 px-5 py-3 text-xs font-semibold text-rose-700">
          <AlertCircle className="mr-2 inline" size={14} />
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Summary label="匹配公会" value={totalItems} />
        <Summary label="成员总数" value={guilds.reduce((sum, guild) => sum + guild.members.length, 0)} />
        <Summary label="在线成员" value={guilds.reduce((sum, guild) => sum + guild.online_member_count, 0)} />
      </div>

      <SaveIndexStatusBar
        status={indexStatus}
        loading={guildsQuery.isFetching}
        rebuilding={rebuildMutation.isPending}
        onRefresh={() => void guildsQuery.refetch()}
        onRebuild={() => rebuildMutation.mutate()}
      />

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading && guilds.length === 0 ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在获取公会数据...
          </div>
        ) : (
          <DataTable
            headers={[
              { key: 'name', label: '公会' },
              { key: 'members', label: '成员' },
              { key: 'online', label: '在线' },
              { key: 'bases', label: '基地' },
              { key: 'owner', label: '会长 UID' },
            ]}
            data={guilds}
            searchText={searchText}
            onSearchChange={setSearchText}
            searchPlaceholder="搜索公会或会长 UID"
            pagination={{
              currentPage: page,
              totalPages,
              totalItems,
              itemsPerPage: pageSize,
              onPageChange: setPage,
            }}
            virtualized
            emptyText={error ? '存档索引不可用' : '暂无公会'}
            renderCard={(guild) => <GuildCard key={guild.id} guild={guild} />}
            renderRow={(guild) => (
              <tr key={guild.id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 text-xs font-bold text-slate-700">{guild.name}</td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-600">{guild.members.length} 人</td>
                <td className="px-6 py-4 text-xs font-semibold text-emerald-600">{guild.online_member_count} 人</td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-600">{guild.base_ids.length} 个</td>
                <td className="px-6 py-4 font-mono text-[10px] text-slate-400">{guild.owner_player_uid || '-'}</td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};

const Summary: React.FC<{ label: string; value: number }> = ({ label, value }) => (
  <div className="flex items-center gap-3 rounded-2xl border border-slate-100 bg-white px-5 py-3 shadow-sm">
    <Users size={15} className="text-sky-500" />
    <span className="text-xs font-semibold text-slate-500">
      {label}: {value}
    </span>
  </div>
);

const GuildCard: React.FC<{ guild: Guild }> = ({ guild }) => (
  <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
    <p className="truncate text-sm font-bold text-slate-800">{guild.name}</p>
    <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] font-semibold text-slate-500">
      <span>成员: {guild.members.length}</span>
      <span>在线: {guild.online_member_count}</span>
      <span>基地: {guild.base_ids.length}</span>
      <span className="truncate font-mono">会长: {guild.owner_player_uid || '-'}</span>
    </div>
  </div>
);
