import React, { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronLeft, ChevronRight, Clipboard, Globe2, KeyRound, RefreshCw, Search, Server as ServerIcon, Users } from 'lucide-react';
import { communityServersApi } from '../api/communityServers';
import type { CommunityServer, CommunityServerQuery, CommunityServerResult, CommunityServerSourceStatus } from '../api/communityServers';
import { getErrorMessage } from '../api/client';

const initialQuery: Required<Pick<CommunityServerQuery, 'region' | 'search' | 'min_players' | 'max_players' | 'version' | 'status' | 'page' | 'page_size'>> & Pick<CommunityServerQuery, 'password'> = {
  region: 'cn', search: '', min_players: 0, max_players: 0, version: '', status: 'online', page: 1, page_size: 30,
};

const formatTime = (value?: string) => {
  if (!value) return '尚未更新';
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString('zh-CN', { hour12: false });
};

const ServerDetails: React.FC<{ server: CommunityServer; onCopy: (server: CommunityServer) => void }> = ({ server, onCopy }) => (
  <div className="grid gap-3 border-t border-slate-100 bg-slate-50/70 px-4 py-4 text-xs text-slate-600 sm:grid-cols-2 lg:grid-cols-4">
    <div><span className="block font-semibold text-slate-400">版本</span><strong className="mt-1 block text-slate-700">{server.version || '未知'}</strong></div>
    <div><span className="block font-semibold text-slate-400">国家/地区</span><strong className="mt-1 block text-slate-700">{server.country || '未知'}</strong></div>
    <div><span className="block font-semibold text-slate-400">数据更新时间</span><strong className="mt-1 block text-slate-700">{formatTime(server.updated_at)}</strong></div>
    <button type="button" onClick={() => void onCopy(server)} className="pp-btn pp-btn--ghost pp-btn--sm justify-self-start lg:justify-self-end"><Clipboard size={13} />复制 {server.connect}</button>
    {server.description && <p className="sm:col-span-2 lg:col-span-4 whitespace-pre-wrap leading-6 text-slate-600">{server.description}</p>}
  </div>
);

export const CommunityServers: React.FC = () => {
  const [draft, setDraft] = useState(initialQuery);
  const [query, setQuery] = useState<CommunityServerQuery>(initialQuery);
  const [result, setResult] = useState<CommunityServerResult | null>(null);
  const [sourceStatus, setSourceStatus] = useState<CommunityServerSourceStatus | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = async (nextQuery: CommunityServerQuery, force = false) => {
    if (force) setRefreshing(true);
    else setLoading(true);
    setError(null);
    setNotice(null);
    try {
      const [nextResult, nextStatus] = await Promise.all([
        force ? communityServersApi.refresh(nextQuery) : communityServersApi.list(nextQuery),
        communityServersApi.sourceStatus().catch(() => null),
      ]);
      setResult(nextResult);
      setSourceStatus(nextStatus);
    } catch (loadError) {
      setError(getErrorMessage(loadError));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => { void load(query); }, [query]);

  const pages = useMemo(() => Math.max(1, Math.ceil((result?.total || 0) / (result?.page_size || 30))), [result]);
  const submitFilters = (event: React.FormEvent) => {
    event.preventDefault();
    setExpanded(null);
    setQuery({ ...draft, page: 1 });
  };
  const changePage = (page: number) => {
    setExpanded(null);
    setQuery((current) => ({ ...current, page }));
  };
  const copyAddress = async (server: CommunityServer) => {
    try {
      await navigator.clipboard.writeText(server.connect);
      setNotice(`已复制 ${server.connect}`);
    } catch {
      setNotice(`连接地址：${server.connect}`);
    }
  };

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <section className="overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 px-5 py-6 text-white shadow-sm sm:px-7">
        <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div className="max-w-3xl">
            <div className="mb-3 flex items-center gap-2 text-xs font-bold uppercase tracking-[0.18em] text-sky-300"><Globe2 size={15} /> Community Discovery</div>
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">社区服务器</h1>
            <p className="mt-2 text-sm leading-6 text-slate-300">查询 BattleMetrics 可发现的 Palworld 社区服务器。公开目录可能存在延迟，也不会包含未公开或无法发现的房间。</p>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-300">
            <span className={`rounded-full px-3 py-1.5 font-semibold ${sourceStatus?.reachable ? 'bg-emerald-500/20 text-emerald-200' : 'bg-amber-400/15 text-amber-200'}`}>{sourceStatus?.reachable ? '数据源在线' : '缓存/数据源待确认'}</span>
            {sourceStatus?.proxy_configured && <span className="rounded-full bg-sky-400/15 px-3 py-1.5 font-semibold text-sky-200">已配置国内代理</span>}
          </div>
        </div>
      </section>

      {(error || notice || result?.stale) && <div role="status" className={`flex items-start gap-3 rounded-xl border px-4 py-3 text-sm ${error ? 'border-rose-200 bg-rose-50 text-rose-800' : notice ? 'border-sky-200 bg-sky-50 text-sky-800' : 'border-amber-200 bg-amber-50 text-amber-800'}`}>
        {error || (!notice && result?.stale) ? <AlertTriangle className="mt-0.5 shrink-0" size={16} /> : <Clipboard className="mt-0.5 shrink-0" size={16} />}
        <span>{error || notice || (result?.stale ? `境外数据源暂不可用，正在显示 ${result.cache_age_seconds} 秒前的缓存。` : '')}</span>
      </div>}

      <form onSubmit={submitFilters} className="grid gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:grid-cols-2 lg:grid-cols-6">
        <label className="lg:col-span-2"><span className="mb-1.5 block text-xs font-semibold text-slate-500">服务器名称</span><span className="relative block"><Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} /><input aria-label="服务器名称" value={draft.search} onChange={(event) => setDraft({ ...draft, search: event.target.value })} placeholder="输入名称或关键词" className="w-full rounded-xl border border-slate-200 py-2.5 pl-9 pr-3 text-sm outline-none focus:border-sky-400" /></span></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">范围</span><select aria-label="范围" value={draft.region} onChange={(event) => setDraft({ ...draft, region: event.target.value as 'cn' | 'global' })} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm"><option value="cn">中国区</option><option value="global">全球</option></select></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">在线状态</span><select aria-label="在线状态" value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as 'online' | 'offline' | 'all' })} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm"><option value="online">在线</option><option value="all">全部</option><option value="offline">离线</option></select></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">密码</span><select aria-label="密码" value={draft.password === undefined ? 'all' : String(draft.password)} onChange={(event) => setDraft({ ...draft, password: event.target.value === 'all' ? undefined : event.target.value === 'true' })} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm"><option value="all">不限</option><option value="false">无密码</option><option value="true">有密码</option></select></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">最低人数</span><input aria-label="最低人数" type="number" min={0} value={draft.min_players} onChange={(event) => setDraft({ ...draft, min_players: Math.max(0, Number(event.target.value) || 0) })} className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" /></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">最高人数</span><input aria-label="最高人数" type="number" min={0} value={draft.max_players} onChange={(event) => setDraft({ ...draft, max_players: Math.max(0, Number(event.target.value) || 0) })} className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" /></label>
        <label className="sm:col-span-2 lg:col-span-2"><span className="mb-1.5 block text-xs font-semibold text-slate-500">版本</span><input aria-label="版本" value={draft.version} onChange={(event) => setDraft({ ...draft, version: event.target.value })} placeholder="例如 0.6.4" className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" /></label>
        <div className="flex items-end gap-2 sm:col-span-2 lg:col-span-4 lg:justify-end"><button type="button" onClick={() => void load(query, true)} disabled={refreshing} className="pp-btn pp-btn--ghost"><RefreshCw className={refreshing ? 'animate-spin' : ''} size={15} />刷新数据源</button><button type="submit" className="pp-btn pp-btn--primary"><Search size={15} />查询</button></div>
      </form>

      <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-4 py-3 sm:px-5">
          <div><h2 className="font-bold text-slate-900">可发现服务器</h2><p className="mt-0.5 text-xs text-slate-500">{result ? `${result.total} 个结果 · 更新于 ${formatTime(result.fetched_at)}` : '正在读取公开目录'}</p></div>
          <span className="text-xs font-semibold text-slate-500">第 {result?.page || 1} / {pages} 页</span>
        </div>
        {loading && !result ? <div className="flex items-center justify-center gap-2 p-12 text-sm text-slate-500"><RefreshCw className="animate-spin" size={17} />正在查询社区服务器…</div>
          : result?.servers.length ? <div className="divide-y divide-slate-100">{result.servers.map((server) => <article key={server.id}>
            <button type="button" aria-expanded={expanded === server.id} onClick={() => setExpanded(expanded === server.id ? null : server.id)} className="grid w-full grid-cols-[1fr_auto] items-center gap-4 px-4 py-4 text-left transition hover:bg-slate-50 sm:px-5 lg:grid-cols-[minmax(0,2fr)_150px_150px_130px_auto]">
              <span className="flex min-w-0 items-center gap-3"><span className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl ${server.status === 'online' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500'}`}><ServerIcon size={18} /></span><span className="min-w-0"><strong className="block truncate text-sm text-slate-900">{server.name}</strong><span className="mt-0.5 block truncate font-mono text-xs text-slate-500">{server.connect}</span></span></span>
              <span className="hidden items-center gap-2 text-sm font-semibold text-slate-700 lg:flex"><Users size={15} className="text-slate-400" />{server.players} / {server.max_players || '?'}</span>
              <span className="hidden text-xs font-semibold text-slate-600 lg:block">{server.version || '版本未知'}</span>
              <span className="hidden items-center gap-1.5 text-xs font-semibold text-slate-600 lg:flex">{server.password ? <><KeyRound size={14} />有密码</> : '公开加入'}</span>
              <span className="flex items-center gap-2"><span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-bold text-slate-600 lg:hidden">{server.players}/{server.max_players || '?'}</span><ChevronDown className={`text-slate-400 transition ${expanded === server.id ? 'rotate-180' : ''}`} size={17} /></span>
            </button>
            {expanded === server.id && <ServerDetails server={server} onCopy={copyAddress} />}
          </article>)}</div>
          : <div className="p-12 text-center"><ServerIcon className="mx-auto text-slate-300" size={30} /><h3 className="mt-3 font-bold text-slate-700">没有找到匹配的服务器</h3><p className="mt-1 text-sm text-slate-500">尝试放宽人数、版本或密码筛选。</p></div>}
        <div className="flex items-center justify-between border-t border-slate-200 px-4 py-3 sm:px-5"><span className="text-xs text-slate-500">来源：BattleMetrics · 仅代表可发现社区服务器</span><div className="flex gap-2"><button type="button" aria-label="上一页" disabled={(result?.page || 1) <= 1 || loading} onClick={() => changePage(Math.max(1, (result?.page || 1) - 1))} className="pp-btn pp-btn--ghost pp-btn--sm"><ChevronLeft size={14} /></button><button type="button" aria-label="下一页" disabled={(result?.page || 1) >= pages || loading} onClick={() => changePage((result?.page || 1) + 1)} className="pp-btn pp-btn--ghost pp-btn--sm"><ChevronRight size={14} /></button></div></div>
      </section>
    </div>
  );
};
