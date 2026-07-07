import React, { useEffect, useState } from 'react';
import { ShieldAlert } from 'lucide-react';
import { playersApi } from '../api/players';
import type { Player } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

export const BanList: React.FC = () => {
  const [players, setPlayers] = useState<Player[]>([]);
  const [search, setSearch] = useState('');

  useEffect(() => {
    playersApi.getBanList().then(setPlayers);
  }, []);

  const filtered = players.filter(
    (player) => player.nickname.toLowerCase().includes(search.toLowerCase()) || player.steam_id.includes(search),
  );

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <h3 className="flex items-center gap-2 text-[15px] font-bold text-slate-800">
          <ShieldAlert className="text-rose-500" size={18} />
          封禁与黑名单
        </h3>
        <p className="mt-2 text-xs font-medium leading-relaxed text-slate-400">
          当前 Go 后端尚未提供封禁列表读写接口。本页保留入口，后续接入 PalDefender 或官方管理 API 后可扩展为真实列表。
        </p>
      </section>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        <DataTable
          title={`封禁列表（${players.length}）`}
          headers={[
            { key: 'nickname', label: '玩家' },
            { key: 'steam', label: 'SteamID' },
            { key: 'status', label: '状态' },
          ]}
          data={filtered}
          searchText={search}
          onSearchChange={setSearch}
          searchPlaceholder="搜索封禁玩家"
          emptyText="当前后端未返回封禁数据"
          renderCard={(player) => (
            <div key={player.steam_id} className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
              <p className="text-sm font-bold text-slate-800">{player.nickname}</p>
              <p className="mt-1 font-mono text-[11px] text-slate-400">{player.steam_id}</p>
              <div className="mt-3">
                <StatusBadge status="Error" customText="已封禁" />
              </div>
            </div>
          )}
          renderRow={(player) => (
            <tr key={player.steam_id} className="hover:bg-slate-50/50">
              <td className="px-6 py-4 text-xs font-bold text-slate-700">{player.nickname}</td>
              <td className="px-6 py-4 font-mono text-xs text-slate-400">{player.steam_id}</td>
              <td className="px-6 py-4">
                <StatusBadge status="Error" customText="已封禁" />
              </td>
            </tr>
          )}
        />
      </section>
    </div>
  );
};
