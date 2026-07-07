import React, { useEffect, useMemo, useState } from 'react';
import { ShieldAlert } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { playersApi } from '../api/players';
import type { PlayerAccessEntry } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

export const BanList: React.FC = () => {
  const [players, setPlayers] = useState<PlayerAccessEntry[]>([]);
  const [search, setSearch] = useState('');
  const [message, setMessage] = useState<string | null>(null);

  const load = async () => {
    const list = await playersApi.getBanList();
    setPlayers(list);
  };

  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(
    () =>
      players.filter(
        (player) =>
          (player.nickname || '').toLowerCase().includes(search.toLowerCase()) ||
          player.steam_id.includes(search),
      ),
    [players, search],
  );

  const unban = async (player: PlayerAccessEntry) => {
    if (!window.confirm(`解除封禁 ${player.nickname || player.steam_id}？`)) return;
    try {
      await playersApi.unbanPlayer(player.steam_id);
      setMessage('已解除封禁');
      await load();
    } catch (error) {
      setMessage(getErrorMessage(error));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      {message && <div className="rounded-2xl border border-sky-100 bg-sky-50 px-5 py-3 text-xs font-semibold text-sky-700">{message}</div>}

      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <h3 className="flex items-center gap-2 text-[15px] font-bold text-slate-800">
          <ShieldAlert className="text-rose-500" size={18} />
          封禁与黑名单
        </h3>
        <p className="mt-2 text-xs font-medium leading-relaxed text-slate-400">
          当前列表存储在面板数据库中；如需同步到 PalDefender 或游戏原生封禁文件，可在后续接入对应运行时能力。
        </p>
      </section>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        <DataTable
          title={`封禁列表（${players.length}）`}
          headers={[
            { key: 'nickname', label: '玩家' },
            { key: 'steam', label: 'SteamID' },
            { key: 'reason', label: '原因' },
            { key: 'status', label: '状态' },
            { key: 'actions', label: '操作', align: 'center' },
          ]}
          data={filtered}
          searchText={search}
          onSearchChange={setSearch}
          searchPlaceholder="搜索封禁玩家"
          emptyText="暂无封禁数据"
          renderCard={(player) => (
            <div key={player.steam_id} className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
              <p className="text-sm font-bold text-slate-800">{player.nickname || '未知玩家'}</p>
              <p className="mt-1 font-mono text-[11px] text-slate-400">{player.steam_id}</p>
              <p className="mt-2 text-[11px] font-semibold text-slate-500">{player.reason || '-'}</p>
              <button type="button" onClick={() => unban(player)} className="mt-3 rounded-xl border border-rose-200 px-4 py-2 text-xs font-bold text-rose-600">
                解除封禁
              </button>
            </div>
          )}
          renderRow={(player) => (
            <tr key={player.steam_id} className="hover:bg-slate-50/50">
              <td className="px-6 py-4 text-xs font-bold text-slate-700">{player.nickname || '未知玩家'}</td>
              <td className="px-6 py-4 font-mono text-xs text-slate-400">{player.steam_id}</td>
              <td className="px-6 py-4 text-xs font-semibold text-slate-500">{player.reason || '-'}</td>
              <td className="px-6 py-4">
                <StatusBadge status="Error" customText="已封禁" />
              </td>
              <td className="px-6 py-4 text-center">
                <button type="button" onClick={() => unban(player)} className="rounded-lg border border-rose-200 px-3 py-2 text-[10px] font-bold text-rose-600 hover:bg-rose-50">
                  解除
                </button>
              </td>
            </tr>
          )}
        />
      </section>
    </div>
  );
};
