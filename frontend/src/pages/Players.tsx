import React, { useEffect, useMemo, useState } from 'react';
import { AlertCircle, RefreshCw } from 'lucide-react';
import { playersApi } from '../api/players';
import { useServerStore } from '../store/useServerStore';
import type { Player } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

export const Players: React.FC = () => {
  const { refreshKey } = useServerStore();
  const [players, setPlayers] = useState<Player[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchText, setSearchText] = useState('');
  const [activeTab, setActiveTab] = useState('all');

  const load = async () => {
    setLoading(true);
    const data = await playersApi.getPlayers();
    setPlayers(Array.isArray(data) ? data : []);
    setLoading(false);
  };

  useEffect(() => {
    load();
  }, [refreshKey]);

  const filtered = useMemo(() => {
    const keyword = searchText.toLowerCase();
    return players.filter((player) => {
      const matches = player.nickname.toLowerCase().includes(keyword) || player.steam_id.includes(keyword);
      if (activeTab === 'online') return matches && player.is_online;
      if (activeTab === 'offline') return matches && !player.is_online;
      return matches;
    });
  }, [players, searchText, activeTab]);

  const headers = [
    { key: 'nickname', label: '玩家 / SteamID' },
    { key: 'level', label: '等级' },
    { key: 'guild_name', label: '公会' },
    { key: 'coordinates', label: '坐标' },
    { key: 'status', label: '状态' },
    { key: 'last_seen', label: '最后在线' },
    { key: 'actions', label: '操作', align: 'center' as const },
  ];

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Summary label="在线玩家" value={players.filter((player) => player.is_online).length} color="emerald" />
        <Summary label="离线玩家" value={players.filter((player) => !player.is_online).length} color="slate" />
        <Summary label="总玩家" value={players.length} color="sky" />
      </div>

      <div className="rounded-2xl border border-amber-100 bg-amber-50 px-5 py-3 text-xs font-semibold text-amber-800">
        <AlertCircle className="mr-2 inline" size={14} />
        当前后端只代理官方玩家列表查询，踢出、封禁、传送、发物品等写操作尚未提供接口。
      </div>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading && players.length === 0 ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在获取玩家数据...
          </div>
        ) : (
          <DataTable
            headers={headers}
            data={filtered}
            searchText={searchText}
            onSearchChange={setSearchText}
            searchPlaceholder="搜索玩家昵称或 SteamID"
            tabs={[
              { id: 'all', label: '全部' },
              { id: 'online', label: '在线' },
              { id: 'offline', label: '离线' },
            ]}
            activeTab={activeTab}
            onTabChange={setActiveTab}
            emptyText="暂无玩家"
            renderCard={(player) => <PlayerCard key={player.steam_id} player={player} />}
            renderRow={(player) => (
              <tr key={player.steam_id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4">
                  <PlayerIdentity player={player} />
                </td>
                <td className="px-6 py-4 text-xs font-bold text-slate-600">Lv.{player.level}</td>
                <td className="px-6 py-4 text-xs font-semibold text-slate-500">{player.guild_name}</td>
                <td className="px-6 py-4 font-mono text-xs text-slate-500">
                  {player.x.toFixed(0)}, {player.y.toFixed(0)}, {player.z.toFixed(0)}
                </td>
                <td className="px-6 py-4">
                  <StatusBadge status={player.is_online ? 'Online' : 'Offline'} />
                </td>
                <td className="px-6 py-4 text-xs font-medium text-slate-400">{player.last_online_time}</td>
                <td className="px-6 py-4 text-center">
                  <span className="rounded-lg bg-slate-50 px-2 py-1 text-[10px] font-bold text-slate-400">未支持</span>
                </td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};

const Summary: React.FC<{ label: string; value: number; color: 'emerald' | 'slate' | 'sky' }> = ({ label, value, color }) => {
  const dot = color === 'emerald' ? 'bg-emerald-500' : color === 'sky' ? 'bg-sky-500' : 'bg-slate-400';
  return (
    <div className="flex items-center gap-3 rounded-2xl border border-slate-100 bg-white px-5 py-3 shadow-sm">
      <span className={`h-2.5 w-2.5 rounded-full ${dot}`} />
      <span className="text-xs font-semibold text-slate-500">
        {label}: {value}
      </span>
    </div>
  );
};

const PlayerIdentity: React.FC<{ player: Player }> = ({ player }) => (
  <div className="flex min-w-0 items-center gap-3">
    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-sky-50 text-xs font-bold text-sky-600">
      {player.nickname.substring(0, 2).toUpperCase()}
    </div>
    <div className="min-w-0">
      <p className="truncate text-xs font-bold text-slate-700">{player.nickname}</p>
      <p className="truncate font-mono text-[10px] text-slate-400">{player.steam_id}</p>
    </div>
  </div>
);

const PlayerCard: React.FC<{ player: Player }> = ({ player }) => (
  <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
    <div className="flex items-start justify-between gap-3">
      <PlayerIdentity player={player} />
      <StatusBadge status={player.is_online ? 'Online' : 'Offline'} />
    </div>
    <div className="mt-4 grid grid-cols-2 gap-2 text-[11px] font-semibold text-slate-500">
      <span>等级: Lv.{player.level}</span>
      <span className="truncate">公会: {player.guild_name}</span>
      <span className="col-span-2 font-mono">
        坐标: {player.x.toFixed(0)}, {player.y.toFixed(0)}, {player.z.toFixed(0)}
      </span>
      <span className="col-span-2 text-slate-400">最后在线: {player.last_online_time}</span>
    </div>
  </div>
);
