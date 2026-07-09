import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Plus, RefreshCw, ShieldAlert, Trash2 } from 'lucide-react';
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
  const [steamId, setSteamId] = useState('');
  const [nickname, setNickname] = useState('');
  const [reason, setReason] = useState('');

  const load = useCallback(async () => {
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
  }, []);

  useEffect(() => {
    load();
  }, [load]);

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
      setError(null);
      await load();
    } catch (actionError) {
      setMessage(null);
      setError(getErrorMessage(actionError));
    } finally {
      setPending(null);
    }
  };

  const deleteLocalRecord = async (player: PlayerAccessEntry) => {
    if (!window.confirm(`只删除本地记录 ${player.nickname || player.steam_id}？不会调用 Palworld REST。`)) return;
    setPending(`local:${player.steam_id}`);
    try {
      await playersApi.deleteLocalBan(player.steam_id);
      setMessage('已删除本地封禁记录');
      setError(null);
      await load();
    } catch (actionError) {
      setMessage(null);
      setError(getErrorMessage(actionError));
    } finally {
      setPending(null);
    }
  };

  const addLocalRecord = async (event: React.FormEvent) => {
    event.preventDefault();
    const normalizedSteamID = steamId.trim();
    if (!normalizedSteamID) {
      setMessage(null);
      setError('SteamID 不能为空');
      return;
    }
    setPending('add');
    try {
      await playersApi.addLocalBan({
        steam_id: normalizedSteamID,
        nickname: nickname.trim() || undefined,
        reason: reason.trim() || undefined,
      });
      setSteamId('');
      setNickname('');
      setReason('');
      setMessage('已新增本地封禁记录');
      setError(null);
      await load();
    } catch (actionError) {
      setMessage(null);
      setError(getErrorMessage(actionError));
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
        <form onSubmit={addLocalRecord} className="mt-5 grid grid-cols-1 gap-3 lg:grid-cols-[1.2fr_1fr_1.4fr_auto]">
          <input
            type="text"
            value={steamId}
            onChange={(event) => setSteamId(event.target.value)}
            placeholder="SteamID"
            className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
          />
          <input
            type="text"
            value={nickname}
            onChange={(event) => setNickname(event.target.value)}
            placeholder="昵称（可选）"
            className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
          />
          <input
            type="text"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="原因（可选，本地备注）"
            className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
          />
          <button
            type="submit"
            disabled={Boolean(pending)}
            className="inline-flex items-center justify-center gap-2 rounded-xl bg-slate-900 px-4 py-2 text-xs font-bold text-white hover:bg-slate-800 disabled:opacity-40"
          >
            <Plus size={14} />
            {pending === 'add' ? '提交中' : '新增本地记录'}
          </button>
        </form>
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
            headerActions={
              <button
                type="button"
                onClick={load}
                disabled={loading}
                className="inline-flex items-center justify-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 disabled:opacity-40"
              >
                <RefreshCw size={13} className={loading ? 'animate-spin' : ''} />
                刷新
              </button>
            }
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
                <div className="mt-3 flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => unban(player)}
                    disabled={pending === player.steam_id || Boolean(pending)}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-rose-200 px-3 py-2 text-xs font-bold text-rose-600 hover:bg-rose-50 disabled:opacity-40"
                  >
                    <Trash2 size={14} />
                    {pending === player.steam_id ? '提交中' : '解除封禁'}
                  </button>
                  <button
                    type="button"
                    onClick={() => deleteLocalRecord(player)}
                    disabled={pending === `local:${player.steam_id}` || Boolean(pending)}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-2 text-xs font-bold text-slate-500 hover:bg-slate-50 disabled:opacity-40"
                  >
                    <Trash2 size={14} />
                    {pending === `local:${player.steam_id}` ? '删除中' : '删除本地记录'}
                  </button>
                </div>
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
                  <div className="flex justify-center gap-2">
                    <button
                      type="button"
                      onClick={() => unban(player)}
                      disabled={pending === player.steam_id || Boolean(pending)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-rose-200 px-3 py-2 text-[10px] font-bold text-rose-600 hover:bg-rose-50 disabled:opacity-40"
                    >
                      <Trash2 size={14} />
                      {pending === player.steam_id ? '提交中' : '解封'}
                    </button>
                    <button
                      type="button"
                      onClick={() => deleteLocalRecord(player)}
                      disabled={pending === `local:${player.steam_id}` || Boolean(pending)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-2 text-[10px] font-bold text-slate-500 hover:bg-slate-50 disabled:opacity-40"
                    >
                      <Trash2 size={14} />
                      {pending === `local:${player.steam_id}` ? '删除中' : '删本地'}
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
