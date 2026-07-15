import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import {
  AlertTriangle,
  Ban,
  BellRing,
  Boxes,
  CheckCircle2,
  CircleOff,
  LoaderCircle,
  LogOut,
  Megaphone,
  MessageSquareText,
  PackagePlus,
  Plus,
  RefreshCw,
  Search,
  Send,
  ShieldAlert,
  Trash2,
  UserRound,
} from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { palDefenderGMApi } from '../api/paldefenderGM';
import { useServerStore } from '../store/useServerStore';
import type {
  PalDefenderGMPlayer,
  PalDefenderInventory,
  PalDefenderInventoryContainer,
  PalDefenderItemCatalogEntry,
  PalDefenderItemGrant,
  PalDefenderMessageRequest,
} from '../types';
import { StatusBadge } from '../components/ui/StatusBadge';

type WorkspaceTab = 'items' | 'message' | 'moderation';
type MessageMode = 'player' | 'broadcast' | 'alert';
type ContainerKey = keyof PalDefenderInventory;
type GrantRow = PalDefenderItemGrant & { key: number };

const containerOptions: Array<{ key: ContainerKey; label: string }> = [
  { key: 'Items', label: '物品' },
  { key: 'KeyItems', label: '关键物品' },
  { key: 'Weapons', label: '武器' },
  { key: 'Armor', label: '防具' },
  { key: 'Food', label: '食物' },
  { key: 'DropSlot', label: '掉落槽' },
];

const messageTypes: Array<{ value: NonNullable<PalDefenderMessageRequest['SendType']>; label: string }> = [
  { value: 'PlayerChat', label: '玩家聊天' },
  { value: 'PlayerGlobalChat', label: '全局聊天' },
  { value: 'PlayerGuildChat', label: '公会聊天' },
  { value: 'PlayerLogNormal', label: '普通通知' },
  { value: 'PlayerLogImportant', label: '重要通知' },
  { value: 'PlayerLogVeryImportant', label: '紧急通知' },
];

let grantKey = 1;
const emptyPlayers: PalDefenderGMPlayer[] = [];
const newGrant = (): GrantRow => ({ key: grantKey++, ItemID: '', Count: 1 });
const identifierFor = (player: PalDefenderGMPlayer) => player.UserId || player.PlayerUID;
const isOnline = (player: PalDefenderGMPlayer) => player.Status.toLowerCase() === 'online';

export const PalDefenderGM: React.FC = () => {
  const queryClient = useQueryClient();
  const { session } = useServerStore();
  const canWrite = Boolean(session?.permissions.includes('players:write'));
  const [search, setSearch] = useState('');
  const [selectedID, setSelectedID] = useState('');
  const [activeTab, setActiveTab] = useState<WorkspaceTab>('items');
  const [containerKey, setContainerKey] = useState<ContainerKey>('Items');
  const [grantRows, setGrantRows] = useState<GrantRow[]>([newGrant()]);
  const [messageMode, setMessageMode] = useState<MessageMode>('player');
  const [messageType, setMessageType] = useState<NonNullable<PalDefenderMessageRequest['SendType']>>('PlayerLogImportant');
  const [message, setMessage] = useState('');
  const [reason, setReason] = useState('');
  const [banIP, setBanIP] = useState(false);
  const [pending, setPending] = useState('');
  const [notice, setNotice] = useState('');
  const [actionError, setActionError] = useState('');
  const actionInFlight = useRef(false);

  const statusQuery = useQuery({
    queryKey: ['paldefender-gm', 'status'],
    queryFn: palDefenderGMApi.status,
  });
  const status = statusQuery.data;
  const playersQuery = useQuery({
    queryKey: ['paldefender-gm', 'players'],
    queryFn: palDefenderGMApi.players,
    enabled: Boolean(status?.available),
  });
  const catalogQuery = useQuery({
    queryKey: ['paldefender-gm', 'item-catalog'],
    queryFn: () => palDefenderGMApi.items('', 5000),
    enabled: Boolean(status?.available),
    staleTime: 30 * 60 * 1000,
  });
  const players = playersQuery.data?.Players ?? emptyPlayers;
  const listedPlayer = players.find((player) => identifierFor(player) === selectedID) ?? null;
  const playerQuery = useQuery({
    queryKey: ['paldefender-gm', 'player', selectedID],
    queryFn: () => palDefenderGMApi.player(selectedID),
    enabled: Boolean(status?.available && selectedID),
  });
  const selectedPlayer = playerQuery.data ?? listedPlayer;
  const inventoryQuery = useQuery({
    queryKey: ['paldefender-gm', 'inventory', selectedID],
    queryFn: () => palDefenderGMApi.inventory(selectedID),
    enabled: Boolean(status?.available && selectedID),
  });

  useEffect(() => {
    if (players.length === 0) {
      setSelectedID('');
      return;
    }
    if (!players.some((player) => identifierFor(player) === selectedID)) {
      setSelectedID(identifierFor(players[0]));
    }
  }, [players, selectedID]);

  const filteredPlayers = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return players;
    return players.filter((player) =>
      [player.Name, player.UserId, player.PlayerUID, player.GuildName].some((value) => value.toLowerCase().includes(needle)),
    );
  }, [players, search]);

  const selectedContainer = inventoryQuery.data?.Inventory[containerKey] ?? null;
  const inventoryRows = selectedContainer
    ? Object.entries(selectedContainer.Slots).sort(([left], [right]) => Number(left) - Number(right))
    : [];

  const runAction = async (key: string, action: () => Promise<unknown>, success: string) => {
    if (actionInFlight.current) return;
    actionInFlight.current = true;
    setPending(key);
    setNotice('');
    setActionError('');
    try {
      await action();
      if (success) setNotice(success);
    } catch (error) {
      setActionError(getErrorMessage(error));
    } finally {
      actionInFlight.current = false;
      setPending('');
    }
  };

  const submitGrant = async () => {
    if (!selectedPlayer) return;
    if (!isOnline(selectedPlayer)) {
      setActionError('玩家已离线，未发送物品发放请求');
      return;
    }
    const items = grantRows.map(({ ItemID, Count }) => ({ ItemID: ItemID.trim(), Count: Number(Count) }));
    if (items.some((item) => !item.ItemID || !Number.isInteger(item.Count) || item.Count <= 0)) {
      setActionError('物品 ID 与正整数数量均为必填项');
      return;
    }
    if (!window.confirm(`向 ${selectedPlayer.Name} 发放 ${items.length} 种物品？`)) return;
    await runAction(
      'give',
      async () => {
        const result = await palDefenderGMApi.giveItems(selectedID, items);
        await queryClient.invalidateQueries({ queryKey: ['paldefender-gm', 'inventory', selectedID] });
        setNotice(`已发放 ${result.Granted.Items} 件物品`);
      },
      '',
    );
  };

  const submitMessage = async () => {
    const text = message.trim();
    if (!text) {
      setActionError('消息不能为空');
      return;
    }
    if (messageMode === 'player' && !selectedPlayer) return;
    if (messageMode === 'player' && selectedPlayer && !isOnline(selectedPlayer)) {
      setActionError('玩家已离线，未发送私信请求');
      return;
    }
    const label = messageMode === 'player' ? selectedPlayer?.Name || selectedID : messageMode === 'alert' ? '全服警报' : '全服广播';
    await runAction(
      'message',
      () =>
        messageMode === 'player'
          ? palDefenderGMApi.sendMessage(selectedID, { SendType: messageType, Message: text })
          : palDefenderGMApi.broadcast(text, messageMode === 'alert'),
      `消息已发送：${label}`,
    );
  };

  const moderate = async (action: 'kick' | 'ban' | 'unban') => {
    if (!selectedPlayer) return;
    const labels = { kick: '踢出', ban: '封禁', unban: '解除封禁' };
    if (!window.confirm(`${labels[action]} ${selectedPlayer.Name} (${selectedID})？`)) return;
    await runAction(
      action,
      () => {
        const request = { Reason: reason.trim(), IP: action === 'ban' ? banIP : undefined };
        if (action === 'kick') return palDefenderGMApi.kick(selectedID, request);
        if (action === 'ban') return palDefenderGMApi.ban(selectedID, request);
        return palDefenderGMApi.unban(selectedID, request);
      },
      `${labels[action]}操作已完成：${selectedPlayer.Name}`,
    );
  };

  const queryError = statusQuery.error || playersQuery.error || playerQuery.error || inventoryQuery.error || catalogQuery.error;
  const visibleError = actionError || (queryError ? getErrorMessage(queryError) : '') || status?.error || '';
  const busy = Boolean(pending);

  return (
    <div className="mx-auto flex w-full max-w-[1500px] flex-col gap-4 p-4 sm:p-6 lg:p-8">
      <header className="flex flex-col gap-3 border-b border-slate-200 pb-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <ShieldAlert size={19} className="text-rose-500" />
            <h1 className="text-xl font-bold text-slate-900">PalDefender GM</h1>
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <StatusBadge status={status?.available ? 'Online' : 'Offline'} customText={status?.available ? 'REST 已连接' : 'REST 未连接'} />
            {status?.version?.Version && <span className="font-mono text-[11px] font-semibold text-slate-500">v{status.version.Version}</span>}
            {playersQuery.data && <span className="text-[11px] font-semibold text-slate-500">{playersQuery.data.Meta.OnlineCount} / {playersQuery.data.Meta.PlayerCount} 在线</span>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => {
              void statusQuery.refetch();
              void playersQuery.refetch();
              if (selectedID) {
                void playerQuery.refetch();
                void inventoryQuery.refetch();
              }
            }}
            title="刷新 GM 数据"
            aria-label="刷新 GM 数据"
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 text-slate-600 hover:bg-slate-50"
          >
            <RefreshCw size={15} className={statusQuery.isFetching || playersQuery.isFetching ? 'animate-spin' : ''} />
          </button>
          <Link to="/security" className="rounded-lg border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">
            安全设置
          </Link>
        </div>
      </header>

      {notice && (
        <div role="status" className="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-xs font-semibold text-emerald-800">
          <CheckCircle2 size={15} /> {notice}
        </div>
      )}
      {visibleError && (
        <div role="alert" className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-xs font-semibold text-rose-800">
          <AlertTriangle className="mt-0.5 shrink-0" size={15} /> <span className="break-words">{visibleError}</span>
        </div>
      )}

      {statusQuery.isLoading ? (
        <div className="flex min-h-72 items-center justify-center text-xs font-semibold text-slate-500">
          <LoaderCircle className="mr-2 animate-spin text-sky-500" size={16} /> 正在检查 PalDefender REST...
        </div>
      ) : status?.state === 'not_installed' ? (
        <UnavailableState icon={<CircleOff size={22} />} title="PalDefender 尚未安装" action="打开安全设置" />
      ) : status?.state === 'not_loaded' ? (
        <UnavailableState icon={<CircleOff size={22} />} title="PalDefender 尚未通过启动日志确认加载" action="检查安全设置" />
      ) : status?.state === 'rest_disabled' ? (
        <UnavailableState icon={<CircleOff size={22} />} title="PalDefender REST API 未启用" action="打开安全设置" />
      ) : !status?.configured || status?.state === 'not_configured' ? (
        <UnavailableState icon={<CircleOff size={22} />} title="REST Token 未配置" action="打开安全设置" />
      ) : status?.state === 'server_not_running' ? (
        <UnavailableState icon={<CircleOff size={22} />} title="游戏服务或 PalDefender REST 未运行" action="检查安全设置" />
      ) : !status.available ? (
        <UnavailableState icon={<CircleOff size={22} />} title="PalDefender REST 不可用" action="检查安全设置" />
      ) : (
        <section className="grid min-h-[650px] overflow-hidden rounded-lg border border-slate-200 bg-white lg:grid-cols-[310px_minmax(0,1fr)]">
          <aside className="flex min-h-0 flex-col border-b border-slate-200 lg:border-b-0 lg:border-r">
            <div className="border-b border-slate-100 p-3">
              <label className="relative block">
                <span className="sr-only">搜索 GM 玩家</span>
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={14} />
                <input
                  type="search"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="玩家、平台 ID、公会"
                  className="w-full rounded-lg border border-slate-200 py-2.5 pl-9 pr-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
                />
              </label>
            </div>
            <div className="max-h-72 min-h-0 flex-1 overflow-y-auto p-2 lg:max-h-none">
              {playersQuery.isLoading ? (
                <p className="p-4 text-center text-xs font-semibold text-slate-400">正在读取玩家...</p>
              ) : filteredPlayers.length === 0 ? (
                <p className="p-4 text-center text-xs font-semibold text-slate-400">暂无匹配玩家</p>
              ) : (
                filteredPlayers.map((player) => {
                  const id = identifierFor(player);
                  const selected = selectedID === id;
                  return (
                    <button
                      type="button"
                      key={`${player.PlayerUID}:${player.UserId}`}
                      onClick={() => setSelectedID(id)}
                      className={`mb-1 flex w-full items-center gap-3 rounded-lg border px-3 py-3 text-left ${selected ? 'border-sky-200 bg-sky-50' : 'border-transparent hover:bg-slate-50'}`}
                    >
                      <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${isOnline(player) ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-100 text-slate-500'}`}>
                        <UserRound size={15} />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-xs font-bold text-slate-800">{player.Name}</span>
                        <span className="mt-0.5 block truncate font-mono text-[10px] text-slate-400">{id}</span>
                      </span>
                      <span className={`h-2 w-2 shrink-0 rounded-full ${isOnline(player) ? 'bg-emerald-500' : 'bg-slate-300'}`} />
                    </button>
                  );
                })
              )}
            </div>
          </aside>

          <main className="min-w-0">
            {selectedPlayer ? (
              <>
                <div className="flex flex-col gap-3 border-b border-slate-200 px-4 py-4 sm:px-5 lg:flex-row lg:items-center lg:justify-between">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <h2 className="truncate text-base font-bold text-slate-900">{selectedPlayer.Name}</h2>
                      <StatusBadge status={isOnline(selectedPlayer) ? 'Online' : 'Offline'} />
                    </div>
                    <p className="mt-1 truncate font-mono text-[10px] text-slate-400">{selectedID} · {selectedPlayer.PlayerUID}</p>
                    <p className="mt-1 text-[11px] font-semibold text-slate-500">{selectedPlayer.GuildName || '无公会'} · {selectedPlayer.MapLocation.x.toFixed(0)}, {selectedPlayer.MapLocation.y.toFixed(0)}, {selectedPlayer.MapLocation.z.toFixed(0)}</p>
                  </div>
                  <div className="flex rounded-lg border border-slate-200 bg-slate-100 p-0.5" role="tablist" aria-label="GM 工作区">
                    {([
                      ['items', '物品', Boxes],
                      ['message', '消息', MessageSquareText],
                      ['moderation', '管理', ShieldAlert],
                    ] as const).map(([id, label, Icon]) => (
                      <button
                        type="button"
                        role="tab"
                        aria-selected={activeTab === id}
                        key={id}
                        onClick={() => setActiveTab(id)}
                        className={`inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-xs font-bold ${activeTab === id ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'}`}
                      >
                        <Icon size={13} /> {label}
                      </button>
                    ))}
                  </div>
                </div>

                {activeTab === 'items' && (
                  <ItemsWorkspace
                    rows={grantRows}
                    setRows={setGrantRows}
                    canWrite={canWrite}
                    online={isOnline(selectedPlayer)}
                    busy={busy}
                    pending={pending}
                    onSubmit={() => void submitGrant()}
                    containerKey={containerKey}
                    onContainerChange={setContainerKey}
                    container={selectedContainer}
                    inventoryRows={inventoryRows}
                    loading={inventoryQuery.isLoading || inventoryQuery.isFetching}
                    onRefresh={() => void inventoryQuery.refetch()}
                    catalog={catalogQuery.data?.items ?? []}
                  />
                )}
                {activeTab === 'message' && (
                  <MessageWorkspace
                    mode={messageMode}
                    onModeChange={setMessageMode}
                    messageType={messageType}
                    onMessageTypeChange={setMessageType}
                    message={message}
                    onMessageChange={setMessage}
                    canWrite={canWrite}
                    online={isOnline(selectedPlayer)}
                    busy={busy}
                    onSubmit={() => void submitMessage()}
                  />
                )}
                {activeTab === 'moderation' && (
                  <ModerationWorkspace
                    reason={reason}
                    onReasonChange={setReason}
                    banIP={banIP}
                    onBanIPChange={setBanIP}
                    canWrite={canWrite}
                    busy={busy}
                    online={isOnline(selectedPlayer)}
                    pending={pending}
                    onAction={(action) => void moderate(action)}
                  />
                )}
              </>
            ) : (
              <div className="flex min-h-[650px] items-center justify-center text-xs font-semibold text-slate-400">请选择玩家</div>
            )}
          </main>
        </section>
      )}
    </div>
  );
};

const UnavailableState: React.FC<{ icon: React.ReactNode; title: string; action: string }> = ({ icon, title, action }) => (
  <div className="flex min-h-72 flex-col items-center justify-center rounded-lg border border-slate-200 bg-white px-6 text-center">
    <span className="text-slate-400">{icon}</span>
    <p className="mt-3 text-sm font-bold text-slate-800">{title}</p>
    <Link to="/security" className="mt-4 rounded-lg bg-slate-900 px-4 py-2.5 text-xs font-bold text-white">{action}</Link>
  </div>
);

const ItemsWorkspace: React.FC<{
  rows: GrantRow[];
  setRows: React.Dispatch<React.SetStateAction<GrantRow[]>>;
  canWrite: boolean;
  online: boolean;
  busy: boolean;
  pending: string;
  onSubmit: () => void;
  containerKey: ContainerKey;
  onContainerChange: (key: ContainerKey) => void;
  container: PalDefenderInventoryContainer | null;
  inventoryRows: Array<[string, { ItemID: string; Count: number }]>;
  loading: boolean;
  onRefresh: () => void;
  catalog: PalDefenderItemCatalogEntry[];
}> = ({ rows, setRows, canWrite, online, busy, pending, onSubmit, containerKey, onContainerChange, container, inventoryRows, loading, onRefresh, catalog }) => (
  <div>
    <form
      className="border-b border-slate-200 p-4 sm:p-5"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="text-sm font-bold text-slate-800">发放物品</h3>
        <button
          type="button"
          onClick={() => setRows((current) => [...current, newGrant()])}
          disabled={rows.length >= 100 || busy || !canWrite || !online}
          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 disabled:opacity-40"
        >
          <Plus size={13} /> 添加
        </button>
      </div>
      <div className="flex flex-col gap-2">
        {rows.map((row, index) => (
          <div key={row.key} className="grid grid-cols-[minmax(0,1fr)_110px_36px] gap-2">
            <ItemGrantField
              label={`物品 ID ${index + 1}`}
              value={row.ItemID}
              onChange={(value) => setRows((current) => current.map((item) => item.key === row.key ? { ...item, ItemID: value } : item))}
              catalog={catalog}
            />
            <input
              aria-label={`数量 ${index + 1}`}
              type="number"
              value={row.Count}
              min={1}
              max={2147483647}
              step={1}
              onChange={(event) => setRows((current) => current.map((item) => item.key === row.key ? { ...item, Count: Number(event.target.value) } : item))}
              className="rounded-lg border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
            />
            <button
              type="button"
              title="删除物品行"
              aria-label={`删除物品行 ${index + 1}`}
              onClick={() => setRows((current) => current.length === 1 ? [newGrant()] : current.filter((item) => item.key !== row.key))}
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 text-slate-400 hover:border-rose-200 hover:text-rose-600"
            >
              <Trash2 size={14} />
            </button>
          </div>
        ))}
      </div>
      <button
        type="submit"
        disabled={!canWrite || !online || busy}
        className="mt-3 inline-flex min-w-32 items-center justify-center gap-2 rounded-lg bg-sky-600 px-4 py-2.5 text-xs font-bold text-white hover:bg-sky-700 disabled:opacity-40"
      >
        {pending === 'give' ? <LoaderCircle size={14} className="animate-spin" /> : <PackagePlus size={14} />}
        确认发放
      </button>
    </form>

    <div className="p-4 sm:p-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <select
            aria-label="背包容器"
            value={containerKey}
            onChange={(event) => onContainerChange(event.target.value as ContainerKey)}
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-bold text-slate-700"
          >
            {containerOptions.map((option) => <option key={option.key} value={option.key}>{option.label}</option>)}
          </select>
          {container && <span className="text-[11px] font-semibold text-slate-500">{container.UsedSlots}/{container.MaxSlots} 槽 · {container.FreeSlots} 空闲</span>}
        </div>
        <button type="button" onClick={onRefresh} title="刷新背包" aria-label="刷新背包" className="flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 text-slate-500">
          <RefreshCw size={13} className={loading ? 'animate-spin' : ''} />
        </button>
      </div>
      <div className="mt-3 overflow-x-auto rounded-lg border border-slate-200">
        <table className="w-full min-w-[460px] text-left">
          <thead className="bg-slate-50 text-[10px] font-bold text-slate-400"><tr><th className="px-4 py-2.5">槽位</th><th className="px-4 py-2.5">物品</th><th className="px-4 py-2.5 text-right">数量</th></tr></thead>
          <tbody className="divide-y divide-slate-100">
            {loading && inventoryRows.length === 0 ? (
              <tr><td colSpan={3} className="px-4 py-10 text-center text-xs font-semibold text-slate-400">正在读取背包...</td></tr>
            ) : inventoryRows.length === 0 ? (
              <tr><td colSpan={3} className="px-4 py-10 text-center text-xs font-semibold text-slate-400">该容器为空</td></tr>
            ) : inventoryRows.map(([slot, item]) => (
              <tr key={slot}>
                <td className="px-4 py-3 font-mono text-[11px] text-slate-400">{slot}</td>
                <td className="px-4 py-3"><InventoryItemIdentity itemID={item.ItemID} catalog={catalog} /></td>
                <td className="px-4 py-3 text-right text-xs font-bold text-slate-700">{item.Count.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  </div>
);

const InventoryItemIdentity: React.FC<{
  itemID: string;
  catalog: PalDefenderItemCatalogEntry[];
}> = ({ itemID, catalog }) => {
  const entry = catalog.find((item) => item.id.toLowerCase() === itemID.toLowerCase());
  return (
    <div className="flex min-w-0 items-center gap-2">
      {entry?.icon ? (
        <img
          src={`/assets/items/${encodeURIComponent(entry.icon)}.webp`}
          alt={`${entry.name}图标`}
          className="h-9 w-9 shrink-0 object-contain"
          onError={(event) => { event.currentTarget.style.visibility = 'hidden'; }}
        />
      ) : <span className="h-9 w-9 shrink-0" />}
      <span className="min-w-0">
        {entry?.name && <span className="block truncate text-xs font-bold text-slate-700">{entry.name}</span>}
        <span className="block truncate font-mono text-[10px] font-semibold text-slate-400">{itemID}</span>
      </span>
    </div>
  );
};

const ItemGrantField: React.FC<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  catalog: PalDefenderItemCatalogEntry[];
}> = ({ label, value, onChange, catalog }) => {
  const [focused, setFocused] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const normalized = value.trim().toLowerCase();
  const exact = catalog.find((item) => item.id.toLowerCase() === normalized);
  const suggestions = useMemo(() => {
    if (!focused || !normalized) return [];
    return catalog
      .filter((item) => item.id.toLowerCase().includes(normalized) || item.name.toLowerCase().includes(normalized))
      .sort((left, right) => {
        const leftExact = left.id.toLowerCase() === normalized ? 0 : 1;
        const rightExact = right.id.toLowerCase() === normalized ? 0 : 1;
        return leftExact - rightExact || left.id.localeCompare(right.id);
      })
      .slice(0, 8);
  }, [catalog, focused, normalized]);
  const listID = `item-options-${label.replace(/\s+/g, '-').toLowerCase()}`;
  const choose = (item: PalDefenderItemCatalogEntry) => {
    onChange(item.id);
    setFocused(false);
    setActiveIndex(-1);
  };

  return (
    <div className="relative min-w-0">
      {exact?.icon && (
        <img
          src={`/assets/items/${encodeURIComponent(exact.icon)}.webp`}
          alt=""
          className="pointer-events-none absolute left-2 top-1/2 z-10 h-6 w-6 -translate-y-1/2 object-contain"
          onError={(event) => { event.currentTarget.style.visibility = 'hidden'; }}
        />
      )}
      <input
        role="combobox"
        aria-label={label}
        aria-autocomplete="list"
        aria-expanded={suggestions.length > 0}
        aria-controls={listID}
        aria-activedescendant={activeIndex >= 0 ? `${listID}-${activeIndex}` : undefined}
        value={value}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onChange={(event) => {
          onChange(event.target.value);
          setActiveIndex(-1);
        }}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown' && suggestions.length > 0) {
            event.preventDefault();
            setActiveIndex((current) => Math.min(suggestions.length - 1, current + 1));
          } else if (event.key === 'ArrowUp' && suggestions.length > 0) {
            event.preventDefault();
            setActiveIndex((current) => Math.max(0, current - 1));
          } else if (event.key === 'Enter' && activeIndex >= 0 && suggestions[activeIndex]) {
            event.preventDefault();
            choose(suggestions[activeIndex]);
          } else if (event.key === 'Escape') {
            setFocused(false);
            setActiveIndex(-1);
          }
        }}
        placeholder="ItemID 或中文名"
        maxLength={128}
        className={`w-full min-w-0 rounded-lg border border-slate-200 py-2.5 pr-3 font-mono text-xs text-slate-700 focus:border-sky-500 focus:outline-none ${exact?.icon ? 'pl-10' : 'pl-3'}`}
      />
      {suggestions.length > 0 && (
        <div id={listID} role="listbox" className="absolute left-0 right-0 top-[calc(100%+4px)] z-30 max-h-72 overflow-y-auto rounded-lg border border-slate-200 bg-white p-1 shadow-xl">
          {suggestions.map((item, index) => (
            <button
              type="button"
              role="option"
              aria-selected={activeIndex === index}
              id={`${listID}-${index}`}
              key={item.id}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => choose(item)}
              className={`flex w-full items-center gap-2 rounded-md px-2 py-2 text-left ${activeIndex === index ? 'bg-sky-50' : 'hover:bg-slate-50'}`}
            >
              {item.icon ? (
                <img
                  src={`/assets/items/${encodeURIComponent(item.icon)}.webp`}
                  alt=""
                  className="h-8 w-8 shrink-0 object-contain"
                  onError={(event) => { event.currentTarget.style.visibility = 'hidden'; }}
                />
              ) : <span className="h-8 w-8 shrink-0" />}
              <span className="min-w-0">
                <span className="block truncate text-xs font-bold text-slate-700">{item.name}</span>
                <span className="block truncate font-mono text-[10px] text-slate-400">{item.id}</span>
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
};

const MessageWorkspace: React.FC<{
  mode: MessageMode;
  onModeChange: (mode: MessageMode) => void;
  messageType: NonNullable<PalDefenderMessageRequest['SendType']>;
  onMessageTypeChange: (type: NonNullable<PalDefenderMessageRequest['SendType']>) => void;
  message: string;
  onMessageChange: (message: string) => void;
  canWrite: boolean;
  online: boolean;
  busy: boolean;
  onSubmit: () => void;
}> = ({ mode, onModeChange, messageType, onMessageTypeChange, message, onMessageChange, canWrite, online, busy, onSubmit }) => (
  <form className="max-w-3xl p-4 sm:p-5" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
    <div className="flex w-fit rounded-lg border border-slate-200 bg-slate-100 p-0.5" aria-label="消息目标">
      {([
        ['player', '当前玩家', MessageSquareText],
        ['broadcast', '全服广播', Megaphone],
        ['alert', '全服警报', BellRing],
      ] as const).map(([id, label, Icon]) => (
        <button type="button" key={id} onClick={() => onModeChange(id)} className={`inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-xs font-bold ${mode === id ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500'}`}>
          <Icon size={13} /> {label}
        </button>
      ))}
    </div>
    {mode === 'player' && (
      <label className="mt-4 flex max-w-sm flex-col gap-1.5 text-xs font-bold text-slate-600">
        消息类型
        <select value={messageType} onChange={(event) => onMessageTypeChange(event.target.value as NonNullable<PalDefenderMessageRequest['SendType']>)} className="rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold">
          {messageTypes.map((type) => <option key={type.value} value={type.value}>{type.label}</option>)}
        </select>
      </label>
    )}
    <label className="mt-4 flex flex-col gap-1.5 text-xs font-bold text-slate-600">
      消息内容
      <textarea value={message} onChange={(event) => onMessageChange(event.target.value)} maxLength={4096} rows={7} className="resize-y rounded-lg border border-slate-200 p-3 text-sm font-medium text-slate-700 focus:border-sky-500 focus:outline-none" />
    </label>
    <div className="mt-2 text-right text-[10px] font-semibold text-slate-400">{message.length}/4096</div>
    <button type="submit" disabled={!canWrite || busy || !message.trim() || (mode === 'player' && !online)} className="mt-3 inline-flex items-center gap-2 rounded-lg bg-slate-900 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">
      {busy ? <LoaderCircle size={14} className="animate-spin" /> : <Send size={14} />} 发送
    </button>
  </form>
);

const ModerationWorkspace: React.FC<{
  reason: string;
  onReasonChange: (reason: string) => void;
  banIP: boolean;
  onBanIPChange: (enabled: boolean) => void;
  canWrite: boolean;
  busy: boolean;
  online: boolean;
  pending: string;
  onAction: (action: 'kick' | 'ban' | 'unban') => void;
}> = ({ reason, onReasonChange, banIP, onBanIPChange, canWrite, busy, online, pending, onAction }) => (
  <div className="max-w-3xl p-4 sm:p-5">
    <label className="flex flex-col gap-1.5 text-xs font-bold text-slate-600">
      操作原因
      <textarea value={reason} onChange={(event) => onReasonChange(event.target.value)} maxLength={1024} rows={5} className="resize-y rounded-lg border border-slate-200 p-3 text-sm font-medium text-slate-700 focus:border-sky-500 focus:outline-none" />
    </label>
    <label className="mt-3 flex w-fit items-center gap-2 text-xs font-bold text-slate-600">
      <input type="checkbox" checked={banIP} onChange={(event) => onBanIPChange(event.target.checked)} className="h-4 w-4 accent-rose-600" />
      同时封禁 IP
    </label>
    <div className="mt-5 flex flex-wrap gap-2 border-t border-slate-200 pt-5">
      <button type="button" onClick={() => onAction('kick')} disabled={!canWrite || busy || !online} className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs font-bold text-amber-800 disabled:opacity-40">
        {pending === 'kick' ? <LoaderCircle size={14} className="animate-spin" /> : <LogOut size={14} />} 踢出
      </button>
      <button type="button" onClick={() => onAction('ban')} disabled={!canWrite || busy} className="inline-flex items-center gap-2 rounded-lg bg-rose-600 px-4 py-2.5 text-xs font-bold text-white disabled:opacity-40">
        {pending === 'ban' ? <LoaderCircle size={14} className="animate-spin" /> : <Ban size={14} />} 封禁
      </button>
      <button type="button" onClick={() => onAction('unban')} disabled={!canWrite || busy} className="inline-flex items-center gap-2 rounded-lg border border-slate-200 px-4 py-2.5 text-xs font-bold text-slate-600 disabled:opacity-40">
        {pending === 'unban' ? <LoaderCircle size={14} className="animate-spin" /> : <CircleOff size={14} />} 解除封禁
      </button>
    </div>
  </div>
);
