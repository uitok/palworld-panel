import React, { useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronLeft, ChevronRight, Clipboard, Globe2, KeyRound, RefreshCw, Search, Server as ServerIcon, Users } from 'lucide-react';
import { communityServersApi } from '../api/communityServers';
import type { CommunityServer, CommunityServerQuery, CommunityServerResult, CommunityServerSourceStatus } from '../api/communityServers';
import { getErrorMessage } from '../api/client';
import { useI18n, type Locale } from '../i18n';

const initialQuery: Required<Pick<CommunityServerQuery, 'region' | 'search' | 'min_players' | 'max_players' | 'version' | 'status' | 'page' | 'page_size'>> & Pick<CommunityServerQuery, 'password'> = {
  region: 'cn', search: '', min_players: 0, max_players: 0, version: '', status: 'online', page: 1, page_size: 30,
};

const formatTime = (value: string | undefined, locale: Locale, notUpdated: string) => {
  if (!value) return notUpdated;
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString(locale, { hour12: false });
};

const ServerDetails: React.FC<{ server: CommunityServer; onCopy: (server: CommunityServer) => void }> = ({ server, onCopy }) => {
  const { locale, t } = useI18n();
  return (
  <div className="grid gap-3 border-t border-slate-100 bg-slate-50/70 px-4 py-4 text-xs text-slate-600 sm:grid-cols-2 lg:grid-cols-4">
    <div><span className="block font-semibold text-slate-400">{t('community.version')}</span><strong className="mt-1 block text-slate-700">{server.version || t('common.unknown')}</strong></div>
    <div><span className="block font-semibold text-slate-400">{t('community.country')}</span><strong className="mt-1 block text-slate-700">{server.country || t('common.unknown')}</strong></div>
    <div><span className="block font-semibold text-slate-400">{t('community.updatedAt')}</span><strong className="mt-1 block text-slate-700">{formatTime(server.updated_at, locale, t('community.notUpdated'))}</strong></div>
    <button type="button" onClick={() => void onCopy(server)} className="pp-btn pp-btn--ghost pp-btn--sm justify-self-start lg:justify-self-end"><Clipboard size={13} />{t('community.copy', { address: server.connect })}</button>
    {server.description && <p className="sm:col-span-2 lg:col-span-4 whitespace-pre-wrap leading-6 text-slate-600">{server.description}</p>}
  </div>
  );
};

export const CommunityServers: React.FC = () => {
  const { locale, t } = useI18n();
  const [draft, setDraft] = useState(initialQuery);
  const [query, setQuery] = useState<CommunityServerQuery>(initialQuery);
  const [result, setResult] = useState<CommunityServerResult | null>(null);
  const [sourceStatus, setSourceStatus] = useState<CommunityServerSourceStatus | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const requestSequence = useRef(0);

  const load = async (nextQuery: CommunityServerQuery, force = false) => {
	const requestID = ++requestSequence.current;
    if (force) setRefreshing(true);
	else {
		setLoading(true);
		setResult(null);
		setSourceStatus(null);
	}
    setError(null);
    setNotice(null);
    try {
	  const nextResult = await (force ? communityServersApi.refresh(nextQuery) : communityServersApi.list(nextQuery));
	  const nextStatus = await communityServersApi.sourceStatus().catch(() => null);
	  if (requestID !== requestSequence.current) return;
      setResult(nextResult);
      setSourceStatus(nextStatus);
    } catch (loadError) {
	  const failedStatus = await communityServersApi.sourceStatus().catch(() => null);
	  if (requestID !== requestSequence.current) return;
	  setSourceStatus(failedStatus);
      setError(getErrorMessage(loadError));
    } finally {
	  if (requestID === requestSequence.current) {
		setLoading(false);
		setRefreshing(false);
	  }
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
      setNotice(t('community.copied', { address: server.connect }));
    } catch {
      setNotice(t('community.address', { address: server.connect }));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <section className="overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 px-5 py-6 text-white shadow-sm sm:px-7">
        <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div className="max-w-3xl">
            <div className="mb-3 flex items-center gap-2 text-xs font-bold uppercase tracking-[0.18em] text-sky-300"><Globe2 size={15} /> {t('community.eyebrow')}</div>
            <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{t('community.title')}</h1>
            <p className="mt-2 text-sm leading-6 text-slate-300">{t('community.description')}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-300">
			<span className={`rounded-full px-3 py-1.5 font-semibold ${sourceStatus?.reachable ? 'bg-emerald-500/20 text-emerald-200' : 'bg-amber-400/15 text-amber-200'}`}>{sourceStatus?.enabled === false ? t('community.featureDisabled') : sourceStatus?.reachable ? t('community.sourceOnline') : t('community.sourceUnknown')}</span>
            {sourceStatus?.proxy_configured && <span className="rounded-full bg-sky-400/15 px-3 py-1.5 font-semibold text-sky-200">{t('community.proxyConfigured')}</span>}
          </div>
        </div>
      </section>

	  {(error || notice || result?.stale || sourceStatus?.cache_error) && <div role="status" className={`flex items-start gap-3 rounded-xl border px-4 py-3 text-sm ${error ? 'border-rose-200 bg-rose-50 text-rose-800' : notice ? 'border-sky-200 bg-sky-50 text-sky-800' : 'border-amber-200 bg-amber-50 text-amber-800'}`}>
        {error || (!notice && result?.stale) ? <AlertTriangle className="mt-0.5 shrink-0" size={16} /> : <Clipboard className="mt-0.5 shrink-0" size={16} />}
		<span>{error || notice || (result?.stale ? t('community.stale', { seconds: result.cache_age_seconds }) : sourceStatus?.cache_error ? t('community.cacheError') : '')}</span>
      </div>}

      <form onSubmit={submitFilters} className="grid gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:grid-cols-2 lg:grid-cols-6">
        <label className="lg:col-span-2"><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.serverName')}</span><span className="relative block"><Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} /><input aria-label={t('community.serverName')} value={draft.search} onChange={(event) => setDraft({ ...draft, search: event.target.value })} placeholder={t('community.searchPlaceholder')} className="w-full rounded-xl border border-slate-200 py-2.5 pl-9 pr-3 text-sm outline-none focus:border-sky-400" /></span></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.scope')}</span><select aria-label={t('community.scope')} value={draft.region} onChange={(event) => setDraft({ ...draft, region: event.target.value as 'cn' | 'global' })} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm"><option value="cn">{t('community.china')}</option><option value="global">{t('community.global')}</option></select></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.status')}</span><select aria-label={t('community.status')} value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as 'online' | 'offline' | 'all' })} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm"><option value="online">{t('community.online')}</option><option value="all">{t('community.all')}</option><option value="offline">{t('community.offline')}</option></select></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.password')}</span><select aria-label={t('community.password')} value={draft.password === undefined ? 'all' : String(draft.password)} onChange={(event) => setDraft({ ...draft, password: event.target.value === 'all' ? undefined : event.target.value === 'true' })} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm"><option value="all">{t('community.any')}</option><option value="false">{t('community.noPassword')}</option><option value="true">{t('community.hasPassword')}</option></select></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.minPlayers')}</span><input aria-label={t('community.minPlayers')} type="number" min={0} value={draft.min_players} onChange={(event) => setDraft({ ...draft, min_players: Math.max(0, Number(event.target.value) || 0) })} className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" /></label>
        <label><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.maxPlayers')}</span><input aria-label={t('community.maxPlayers')} type="number" min={0} value={draft.max_players} onChange={(event) => setDraft({ ...draft, max_players: Math.max(0, Number(event.target.value) || 0) })} className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" /></label>
        <label className="sm:col-span-2 lg:col-span-2"><span className="mb-1.5 block text-xs font-semibold text-slate-500">{t('community.version')}</span><input aria-label={t('community.version')} value={draft.version} onChange={(event) => setDraft({ ...draft, version: event.target.value })} placeholder={t('community.versionPlaceholder')} className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm" /></label>
        <div className="flex items-end gap-2 sm:col-span-2 lg:col-span-4 lg:justify-end"><button type="button" onClick={() => void load(query, true)} disabled={refreshing} className="pp-btn pp-btn--ghost"><RefreshCw className={refreshing ? 'animate-spin' : ''} size={15} />{t('community.refreshSource')}</button><button type="submit" className="pp-btn pp-btn--primary"><Search size={15} />{t('common.search')}</button></div>
      </form>

      <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-4 py-3 sm:px-5">
          <div><h2 className="font-bold text-slate-900">{t('community.discoverable')}</h2><p className="mt-0.5 text-xs text-slate-500">{result ? t('community.results', { count: result.total, time: formatTime(result.fetched_at, locale, t('community.notUpdated')) }) : t('community.reading')}</p></div>
          <span className="text-xs font-semibold text-slate-500">{t('community.page', { page: result?.page || 1, pages })}</span>
        </div>
        {loading && !result ? <div className="flex items-center justify-center gap-2 p-12 text-sm text-slate-500"><RefreshCw className="animate-spin" size={17} />{t('community.querying')}</div>
          : result?.servers.length ? <div className="divide-y divide-slate-100">{result.servers.map((server) => <article key={server.id}>
            <button type="button" aria-expanded={expanded === server.id} onClick={() => setExpanded(expanded === server.id ? null : server.id)} className="grid w-full grid-cols-[1fr_auto] items-center gap-4 px-4 py-4 text-left transition hover:bg-slate-50 sm:px-5 lg:grid-cols-[minmax(0,2fr)_150px_150px_130px_auto]">
              <span className="flex min-w-0 items-center gap-3"><span className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl ${server.status === 'online' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500'}`}><ServerIcon size={18} /></span><span className="min-w-0"><strong className="block truncate text-sm text-slate-900">{server.name}</strong><span className="mt-0.5 block truncate font-mono text-xs text-slate-500">{server.connect}</span></span></span>
              <span className="hidden items-center gap-2 text-sm font-semibold text-slate-700 lg:flex"><Users size={15} className="text-slate-400" />{server.players} / {server.max_players || '?'}</span>
              <span className="hidden text-xs font-semibold text-slate-600 lg:block">{server.version || t('community.versionUnknown')}</span>
              <span className="hidden items-center gap-1.5 text-xs font-semibold text-slate-600 lg:flex">{server.password ? <><KeyRound size={14} />{t('community.hasPassword')}</> : t('community.publicJoin')}</span>
              <span className="flex items-center gap-2"><span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-bold text-slate-600 lg:hidden">{server.players}/{server.max_players || '?'}</span><ChevronDown className={`text-slate-400 transition ${expanded === server.id ? 'rotate-180' : ''}`} size={17} /></span>
            </button>
            {expanded === server.id && <ServerDetails server={server} onCopy={copyAddress} />}
          </article>)}</div>
          : <div className="p-12 text-center"><ServerIcon className="mx-auto text-slate-300" size={30} /><h3 className="mt-3 font-bold text-slate-700">{t('community.emptyTitle')}</h3><p className="mt-1 text-sm text-slate-500">{t('community.emptyDescription')}</p></div>}
        <div className="flex items-center justify-between border-t border-slate-200 px-4 py-3 sm:px-5"><span className="text-xs text-slate-500">{t('community.source')}</span><div className="flex gap-2"><button type="button" aria-label={t('community.previous')} disabled={(result?.page || 1) <= 1 || loading} onClick={() => changePage(Math.max(1, (result?.page || 1) - 1))} className="pp-btn pp-btn--ghost pp-btn--sm"><ChevronLeft size={14} /></button><button type="button" aria-label={t('community.next')} disabled={(result?.page || 1) >= pages || loading} onClick={() => changePage((result?.page || 1) + 1)} className="pp-btn pp-btn--ghost pp-btn--sm"><ChevronRight size={14} /></button></div></div>
      </section>
    </div>
  );
};
