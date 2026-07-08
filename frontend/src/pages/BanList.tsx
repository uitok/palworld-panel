import React, { useEffect, useMemo, useState } from 'react';
import { RefreshCw, ShieldAlert, Trash2 } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { playersApi } from '../api/players';
import type { PlayerAccessEntry } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

export const BanList: React.FC = () => {
  const [players, setPlayers] = useState<PlayerAccessEntry[]>([]);
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    try {
      setPlayers(await playersApi.getBanList());
      setError(null);
    } catch (loadError) {
      setPlayers([]);
      setError(getErrorMessage(loadError));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(() => {
    const keyword = search.toLowerCase();
    return players.filter(
      (player) => (player.nickname || '').toLowerCase().includes(keyword) || player.steam_id.includes(keyword),
    );
  }, [players, search]);

  const unban = async (player: PlayerAccessEntry) => {
    if (!window.confirm(`解除封禁 ${player.nickname || player.steam_id}？`)) return;
    setPending(player.steam_id);
    try {
      await playersApi.unbanPlayer(player.steam_id);
      setMessage('已向官方 REST 提交解封请求，并删除本地封禁记录');
      await load();
    } catch (actionError) {
      setMessage(getErrorMessage(actionError));
    } finally {
      setPending(null);
    }
  };

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {error && <div className="rounded-2xl border border-rose-100 bg-rose-50 px-5 py-3 text-xs font-semibold text-rose-700">{error}</div>}
      {message && <div className="rounded-2xl border border-amber-100 bg-amber-50 px-5 py-3 text-xs font-semibold text-amber-800">{message}</div>}

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <h3 className="flex items-center gap-2 text-[15px] font-bold text-slate-800">
          <ShieldAlert className="text-rose-500" size={18} />
          封禁与黑名单
        </h3>
        <p className="mt-2 text-xs font-medium leading-relaxed text-slate-400">
          列表来自面板本地记录；封禁和解封会调用 Palworld 官方 REST API 后再更新本地记录。
        </p>
      </section>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在读取封禁列表...
          </div>
        ) : (
          <DataTable
            title={`封禁列表 (${players.length})`}
            headers={[
              { key: 'nickname', label: '玩家' },
              { key: 'steam', label: 'SteamID' },
              { key: 'reason', label: '原因' },
              { key: 'created_at', label: '记录时间' },
              { key: 'status', label: '状态' },
              { key: 'actions', label: '操作', align: 'center' as const },
            ]}
            data={filtered}
            searchText={search}
            onSearchChange={setSearch}
            searchPlaceholder="搜索封禁玩家"
            emptyText={error ? '后端不可用或接口未实现' : '暂无封禁记录'}
            renderCard={(player) => (
              <div key={player.steam_id} className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-sm font-bold text-slate-800">{player.nickname || '未知玩家'}</p>
                    <p className="mt-1 font-mono text-[11px] text-slate-400">{player.steam_id}</p>
                  </div>
                  <StatusBadge status="Error" customText="已封禁" />
                </div>
                {player.reason && <p className="mt-2 text-[11px] font-semibold text-slate-500">{player.reason}</p>}
                <button
                  type="button"
                  onClick={() => unban(player)}
                  disabled={pending === player.steam_id}
                  className="mt-3 inline-flex items-center gap-1.5 rounded-lg border border-rose-200 px-3 py-2 text-xs font-bold text-rose-600 hover:bg-rose-50 disabled:opacity-40"
                >
                  <Trash2 size={14} />
                  {pending === player.steam_id ? '提交中' : '解除封禁'}
                </button>
              </div>
            )}
            renderRow={(player) => (
              <tr key={player.steam_id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 text-xs font-bold text-slate-700">{player.nickname || '-'}</td>
                <td className="px-6 py-4 font-mono text-xs text-slate-400">{player.steam_id}</td>
                <td className="px-6 py-4 text-xs font-medium text-slate-500">{player.reason || '-'}</td>
                <td className="px-6 py-4 text-xs font-medium text-slate-400">{player.created_at || '-'}</td>
                <td className="px-6 py-4">
                  <StatusBadge status="Error" customText="已封禁" />
                </td>
                <td className="px-6 py-4 text-center">
                  <button
                    type="button"
                    onClick={() => unban(player)}
                    disabled={pending === player.steam_id}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-rose-200 px-3 py-2 text-[10px] font-bold text-rose-600 hover:bg-rose-50 disabled:opacity-40"
                  >
                    <Trash2 size={14} />
                    {pending === player.steam_id ? '提交中' : '解封'}
                  </button>
                </td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};
