import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
	AlertTriangle,
  CheckCircle2,
  DownloadCloud,
  ExternalLink,
  FileArchive,
  Info,
  Languages,
  PackageCheck,
  Power,
  RefreshCw,
  Search,
  SlidersHorizontal,
  Trash2,
  UploadCloud,
  X,
} from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { modsApi } from '../api/mods';
import { serverApi } from '../api/server';
import { tasksApi } from '../api/tasks';
import type { ImportInspection, Job, ModItem, WorkshopItem } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';
import { useServerStore } from '../store/useServerStore';

type ModsTab = 'store' | 'installed';

const tabs: Array<{ id: ModsTab; label: string }> = [
  { id: 'store', label: 'Mod 商店' },
  { id: 'installed', label: '已安装' },
];

const sortOptions = [
  { id: 'popular', label: '热门' },
  { id: 'trend', label: '趋势' },
  { id: 'new', label: '最新' },
  { id: 'updated', label: '最近更新' },
];

export const Mods: React.FC = () => {
  const { session } = useServerStore();
  const [activeTab, setActiveTab] = useState<ModsTab>('store');
  const [mods, setMods] = useState<ModItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [installedSearch, setInstalledSearch] = useState('');
	const [importOpen, setImportOpen] = useState(false);
  const [activeJob, setActiveJob] = useState<Job | null>(null);
  const [pendingRestart, setPendingRestart] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  const [storeQuery, setStoreQuery] = useState('');
  const [storeSort, setStoreSort] = useState('popular');
  const [tagText, setTagText] = useState('');
  const [storeItems, setStoreItems] = useState<WorkshopItem[]>([]);
  const [storeTotal, setStoreTotal] = useState(0);
  const [storeNextCursor, setStoreNextCursor] = useState<string | undefined>();
  const [storeLoading, setStoreLoading] = useState(false);
  const [storeError, setStoreError] = useState<string | null>(null);
  const [statusLoading, setStatusLoading] = useState(true);
  const [selectedWorkshop, setSelectedWorkshop] = useState<WorkshopItem | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [translationLoading, setTranslationLoading] = useState(false);
  const [translationError, setTranslationError] = useState<string | null>(null);
  const initialLoadRef = useRef(false);

  const loadInstalled = useCallback(async () => {
    setLoading(true);
    const [list, status] = await Promise.all([modsApi.list(), serverApi.getStatus()]);
    setMods(Array.isArray(list) ? list : []);
    setPendingRestart(status.pending_restart);
    setLoading(false);
  }, []);

  const loadWorkshopStatus = useCallback(async () => {
    setStatusLoading(true);
    try {
      return await modsApi.workshopStatus();
    } catch (error) {
      setStoreError(getErrorMessage(error));
      return { configured: false, key_source: '' as const, app_id: '1623730' };
    } finally {
      setStatusLoading(false);
    }
  }, []);

  const loadStore = useCallback(async (reset = true, overrides: { sort?: string } = {}) => {
    setStoreLoading(true);
    setStoreError(null);
    try {
      const response = await modsApi.searchWorkshop({
        q: storeQuery.trim(),
        sort: overrides.sort || storeSort,
        cursor: reset ? undefined : storeNextCursor,
        page_size: 24,
        tags: tagText
          .split(',')
          .map((tag) => tag.trim())
          .filter(Boolean),
      });
      setStoreItems((current) => (reset ? response.items : [...current, ...response.items]));
      setStoreNextCursor(response.next_cursor);
      setStoreTotal(response.total);
    } catch (error) {
      setStoreError(getErrorMessage(error));
      if (reset) {
        setStoreItems([]);
        setStoreNextCursor(undefined);
        setStoreTotal(0);
      }
    } finally {
      setStoreLoading(false);
    }
  }, [storeNextCursor, storeQuery, storeSort, tagText]);

  useEffect(() => {
    if (initialLoadRef.current) return;
    initialLoadRef.current = true;
    void loadInstalled();
    void loadWorkshopStatus().then(() => {
      void loadStore();
    });
  }, [loadInstalled, loadStore, loadWorkshopStatus]);

  const storeStatusByWorkshopID = useMemo(() => {
    const map = new Map<string, WorkshopItem>();
    for (const item of storeItems) {
      map.set(item.id, item);
    }
    return map;
  }, [storeItems]);

  const filteredInstalled = useMemo(() => {
    const keyword = installedSearch.trim().toLowerCase();
    if (!keyword) return mods;
    return mods.filter(
      (mod) =>
        mod.name.toLowerCase().includes(keyword) ||
        mod.package_name.toLowerCase().includes(keyword) ||
        mod.id.toLowerCase().includes(keyword) ||
        String(mod.workshop_id || '').toLowerCase().includes(keyword),
    );
  }, [mods, installedSearch]);

  const trackJob = async (job: Job) => {
    setActiveJob(job);
    const done = await tasksApi.waitForJob(job.id, setActiveJob);
    setMessage(done.status === 'success' ? '任务已完成，重启后生效' : done.error || '任务失败');
    await loadInstalled();
    return done;
  };

  const installWorkshop = async (itemID: string, enable: boolean) => {
    if (!itemID.trim()) {
      setMessage('请输入 Steam Workshop Item ID');
      return;
    }
    try {
      const job = await modsApi.downloadWorkshop(itemID.trim(), enable);
      const done = await trackJob(job);
      if (done.status === 'success' && !storeError) {
        await loadStore(true);
      }
    } catch (error) {
      setMessage(getErrorMessage(error));
    }
  };

  const toggleMod = async (mod: ModItem) => {
    try {
      await modsApi.setEnabled(mod.id, !mod.enabled);
      setMessage(`${mod.name} 已${mod.enabled ? '禁用' : '启用'}，重启后生效`);
      await loadInstalled();
    } catch (error) {
      setMessage(getErrorMessage(error));
    }
  };

  const deleteMod = async (mod: ModItem) => {
    if (!window.confirm(`删除 Mod "${mod.name}"？`)) return;
    try {
      await modsApi.delete(mod.id);
      setMessage('Mod 已删除，重启后生效');
      await loadInstalled();
      if (!storeError) {
        await loadStore(true);
      }
    } catch (error) {
      setMessage(getErrorMessage(error));
    }
  };

  const openWorkshopDetail = async (item: WorkshopItem) => {
    setSelectedWorkshop(item);
    setTranslationError(null);
    setDetailLoading(true);
    try {
      setSelectedWorkshop(await modsApi.getWorkshopItem(item.id));
    } catch (error) {
      setMessage(getErrorMessage(error));
    } finally {
      setDetailLoading(false);
    }
  };

  const translateSelectedWorkshop = async (force: boolean) => {
    if (!selectedWorkshop) return;
    const workshopID = selectedWorkshop.id;
    setTranslationLoading(true);
    setTranslationError(null);
    try {
      const translation = await modsApi.translateWorkshop(workshopID, force);
      setSelectedWorkshop((current) => current?.id === workshopID ? { ...current, translation } : current);
    } catch (error) {
      const errorMessage = getErrorMessage(error);
      setTranslationError(errorMessage);
      setMessage(errorMessage);
    } finally {
      setTranslationLoading(false);
    }
  };

  const headers = [
    { key: 'name', label: 'Mod' },
    { key: 'package', label: 'PackageName' },
    { key: 'source', label: '来源' },
    { key: 'status', label: '状态' },
    { key: 'updated', label: '更新' },
    { key: 'actions', label: '操作', align: 'center' as const },
  ];
  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-5 p-4 sm:p-6 lg:p-8">
	  <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
		<div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-slate-200 bg-white p-1">
		  {tabs.map((tab) => {
			const active = activeTab === tab.id;
			return (
			  <button
				key={tab.id}
				type="button"
				onClick={() => setActiveTab(tab.id)}
				className={`shrink-0 rounded-md px-4 py-2 text-xs font-bold transition-all ${
				  active ? 'bg-slate-900 text-white' : 'text-slate-500 hover:bg-slate-50 hover:text-slate-800'
				}`}
			  >
				{tab.label}
			  </button>
			);
		  })}
		</div>
		<button type="button" onClick={() => setImportOpen(true)} className="inline-flex items-center justify-center gap-2 rounded-lg bg-sky-500 px-4 py-2.5 text-xs font-bold text-white hover:bg-sky-600">
		  <DownloadCloud size={15} />导入 Mod
		</button>
      </div>

      {pendingRestart && (
        <div className="rounded-lg border border-amber-100 bg-amber-50 px-5 py-4">
          <p className="text-xs font-bold text-amber-800">Mod 列表已变更，服务器需要重启后生效。</p>
        </div>
      )}
      {message && <div className="rounded-lg border border-sky-100 bg-sky-50 px-5 py-3 text-xs font-semibold text-sky-700">{message}</div>}
      {activeJob && <JobProgress job={activeJob} />}

      {activeTab === 'store' && (
        <section className="flex flex-col gap-4">
          <div className="rounded-lg border border-slate-100 bg-white p-4">
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_220px_auto]">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
                <input
                  type="search"
                  value={storeQuery}
                  onChange={(event) => setStoreQuery(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') void loadStore(true);
                  }}
                  disabled={statusLoading}
                  placeholder="搜索 Workshop Mod"
                  className="w-full rounded-lg border border-slate-200 py-2.5 pl-9 pr-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none disabled:bg-slate-50 disabled:text-slate-400"
                />
              </div>
              <div className="relative">
                <SlidersHorizontal className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
                <input
                  type="text"
                  value={tagText}
                  onChange={(event) => setTagText(event.target.value)}
                  disabled={statusLoading}
                  placeholder="标签，逗号分隔"
                  className="w-full rounded-lg border border-slate-200 py-2.5 pl-9 pr-3 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none disabled:bg-slate-50 disabled:text-slate-400"
                />
              </div>
              <button
                type="button"
                onClick={() => loadStore(true)}
                disabled={storeLoading || statusLoading}
                className="inline-flex items-center justify-center gap-2 rounded-lg bg-slate-900 px-4 py-2.5 text-xs font-bold text-white hover:bg-slate-800 disabled:opacity-50"
              >
                <Search size={14} />
                搜索
              </button>
            </div>
            <div className="mt-3 flex max-w-full items-center gap-2 overflow-x-auto">
              {sortOptions.map((option) => {
                const active = storeSort === option.id;
                return (
                  <button
                    key={option.id}
                    type="button"
                    disabled={statusLoading}
                    onClick={() => {
                      setStoreSort(option.id);
                      void loadStore(true, { sort: option.id });
                    }}
                    className={`shrink-0 rounded-md border px-3 py-1.5 text-[11px] font-bold disabled:opacity-50 ${
                      active ? 'border-sky-200 bg-sky-50 text-sky-700' : 'border-slate-200 text-slate-500 hover:bg-slate-50'
                    }`}
                  >
                    {option.label}
                  </button>
                );
              })}
            </div>
          </div>

          {storeError && <div className="rounded-lg border border-rose-100 bg-rose-50 px-5 py-4 text-xs font-bold text-rose-700">{storeError}</div>}

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {storeItems.map((item) => (
              <WorkshopCard
                key={item.id}
                item={item}
                onDetail={() => openWorkshopDetail(item)}
                onInstall={() => installWorkshop(item.id, false)}
                onInstallEnabled={() => installWorkshop(item.id, true)}
              />
            ))}
          </div>

          {statusLoading && (
            <div className="py-10 text-center text-xs font-semibold text-slate-400">
              <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
              正在检查 Workshop 配置...
            </div>
          )}
          {storeLoading && !statusLoading && (
            <div className="py-10 text-center text-xs font-semibold text-slate-400">
              <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
              正在读取 Workshop...
            </div>
          )}
          {!statusLoading && !storeLoading && !storeError && storeItems.length === 0 && (
            <div className="rounded-lg border border-dashed border-slate-200 bg-white px-4 py-12 text-center text-xs font-semibold text-slate-400">
              暂无匹配 Mod
            </div>
          )}
          {storeNextCursor && (
            <div className="flex justify-center">
              <button
                type="button"
                onClick={() => loadStore(false)}
                disabled={storeLoading}
                className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50 disabled:opacity-50"
              >
                加载更多{storeTotal > 0 ? `（${storeItems.length}/${storeTotal}）` : ''}
              </button>
            </div>
          )}
        </section>
      )}

      {activeTab === 'installed' && (
        <section className="rounded-lg border border-slate-100 bg-white p-4 sm:p-6">
          {loading ? (
            <div className="py-12 text-center text-xs font-semibold text-slate-400">
              <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
              正在读取 Mod 列表...
            </div>
          ) : (
            <DataTable
              title={`已安装 Mod（${mods.length}）`}
              headers={headers}
              data={filteredInstalled}
              searchText={installedSearch}
              onSearchChange={setInstalledSearch}
              searchPlaceholder="搜索 Mod 名称、PackageName 或 ID"
              emptyText="暂无 Mod"
              renderCard={(mod) => {
                const status = storeStatusByWorkshopID.get(mod.workshop_id || mod.id);
                return (
                  <ModCard
                    key={mod.id}
                    mod={mod}
                    updateAvailable={Boolean(status?.update_available)}
                    onToggle={() => toggleMod(mod)}
                    onDelete={() => deleteMod(mod)}
                    onUpdate={() => installWorkshop(mod.workshop_id || mod.id, mod.enabled)}
                  />
                );
              }}
              renderRow={(mod) => {
                const status = storeStatusByWorkshopID.get(mod.workshop_id || mod.id);
                const updateAvailable = Boolean(status?.update_available);
                return (
                  <tr key={mod.id} className="hover:bg-slate-50/50">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <ModPreview mod={mod} />
                        <div className="min-w-0">
                          <p className="truncate text-xs font-bold text-slate-700">{mod.name}</p>
                          <p className="truncate font-mono text-[10px] text-slate-400">{mod.workshop_id || mod.id}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-xs font-semibold text-slate-600">{mod.package_name || '-'}</td>
                    <td className="px-6 py-4 text-xs font-semibold text-slate-500">{mod.source === 'workshop' ? 'Workshop' : 'Upload'}</td>
                    <td className="px-6 py-4">
                      <StatusBadge status={mod.enabled ? 'enabled' : 'disabled'} />
                    </td>
                    <td className="px-6 py-4">
                      {updateAvailable ? <StatusBadge status="updating" customText="可更新" /> : <span className="text-xs font-medium text-slate-400">-</span>}
                    </td>
                    <td className="px-6 py-4 text-center">
                      <div className="flex justify-center gap-2">
                        {mod.source === 'workshop' && (
                          <button
                            type="button"
                            onClick={() => installWorkshop(mod.workshop_id || mod.id, mod.enabled)}
                            className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50"
                            aria-label="重新安装或更新 Mod"
                          >
                            <RefreshCw size={14} />
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => toggleMod(mod)}
                          className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50"
                          aria-label="启用或禁用 Mod"
                        >
                          <Power size={14} />
                        </button>
                        <button
                          type="button"
                          onClick={() => deleteMod(mod)}
                          className="rounded-lg border border-rose-200 p-2 text-rose-500 hover:bg-rose-50"
                          aria-label="删除 Mod"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              }}
            />
          )}
        </section>
      )}

	  {importOpen && (
		<ImportDialog
		  onClose={() => setImportOpen(false)}
		  onImport={async (job) => {
			setImportOpen(false);
			const done = await trackJob(job);
			if (done.status === 'success' && !storeError) await loadStore(true);
		  }}
		/>
	  )}
      {selectedWorkshop && (
        <WorkshopDrawer
          item={selectedWorkshop}
          loading={detailLoading}
          translationLoading={translationLoading}
          translationError={translationError}
          canTranslate={Boolean(session?.permissions.includes('mods:write'))}
          onClose={() => setSelectedWorkshop(null)}
          onTranslate={translateSelectedWorkshop}
          onInstall={() => installWorkshop(selectedWorkshop.id, false)}
          onInstallEnabled={() => installWorkshop(selectedWorkshop.id, true)}
        />
      )}
    </div>
  );
};

const ImportDialog: React.FC<{
	onClose: () => void;
	onImport: (job: Job) => Promise<void>;
}> = ({ onClose, onImport }) => {
	const [source, setSource] = useState('');
	const [file, setFile] = useState<File | null>(null);
	const [inspection, setInspection] = useState<ImportInspection | null>(null);
	const [candidateID, setCandidateID] = useState('');
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState('');
	const selectedCandidate = inspection?.candidates.find((candidate) => candidate.id === candidateID);

	const inspect = async () => {
		if (!file && !source.trim()) {
			setError('请选择本地 ZIP 或输入导入来源');
			return;
		}
		setBusy(true);
		setError('');
		try {
			const next = await modsApi.inspectImport({ source: source.trim(), file: file || undefined });
			setInspection(next);
			setCandidateID(next.selected_candidate_id || (next.candidates.length === 1 ? next.candidates[0].id : ''));
		} catch (inspectError) {
			setError(getErrorMessage(inspectError));
		} finally {
			setBusy(false);
		}
	};

	const chooseCandidate = async (nextID: string) => {
		if (!inspection) return;
		setCandidateID(nextID);
		const candidate = inspection.candidates.find((item) => item.id === nextID);
		if (!candidate || candidate.ready) return;
		setBusy(true);
		setError('');
		try {
			const next = await modsApi.selectImportCandidate(inspection.id, nextID);
			setInspection(next);
			setCandidateID(next.selected_candidate_id || nextID);
		} catch (selectionError) {
			setError(getErrorMessage(selectionError));
		} finally {
			setBusy(false);
		}
	};

	const startImport = async () => {
		if (!inspection || !selectedCandidate?.ready) return;
		setBusy(true);
		setError('');
		try {
			const job = await modsApi.importInspected(inspection.id, selectedCandidate.id);
			await onImport(job);
		} catch (importError) {
			setError(getErrorMessage(importError));
			setBusy(false);
		}
	};

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-3 sm:p-6" role="dialog" aria-modal="true" aria-labelledby="mod-import-title">
			<div className="flex max-h-[min(760px,95dvh)] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-white shadow-xl">
				<div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
					<div>
						<h2 id="mod-import-title" className="text-base font-bold text-slate-900">导入 Mod</h2>
						<p className="mt-0.5 text-[11px] font-semibold text-slate-400">Workshop、GitHub Release、HTTPS ZIP 或本地 ZIP</p>
					</div>
					<button type="button" onClick={onClose} disabled={busy} className="rounded-lg p-2 text-slate-500 hover:bg-slate-100 disabled:opacity-40" aria-label="关闭导入">
						<X size={17} />
					</button>
				</div>

				<div className="flex flex-1 flex-col gap-5 overflow-y-auto p-5">
					{error && <div role="alert" className="rounded-lg border border-rose-100 bg-rose-50 px-4 py-3 text-xs font-semibold text-rose-700">{error}</div>}
					{!inspection && (
						<div className="grid gap-4">
							<label className="grid gap-1.5 text-xs font-bold text-slate-700">
								导入来源
								<input
									type="text"
									value={source}
									disabled={Boolean(file) || busy}
									onChange={(event) => setSource(event.target.value)}
									placeholder="Workshop ID 或 HTTPS 地址"
									className="rounded-lg border border-slate-200 px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none disabled:bg-slate-50"
								/>
							</label>
							<div className="flex items-center gap-3 text-[10px] font-bold text-slate-300"><span className="h-px flex-1 bg-slate-100" />或<span className="h-px flex-1 bg-slate-100" /></div>
							<label className="grid gap-1.5 text-xs font-bold text-slate-700">
								本地 ZIP
								<span className="flex items-center gap-2 rounded-lg border border-slate-200 p-2.5">
									<UploadCloud size={16} className="shrink-0 text-sky-500" />
									<input
										type="file"
										accept=".zip,application/zip"
										disabled={Boolean(source.trim()) || busy}
										onChange={(event) => setFile(event.target.files?.[0] || null)}
										className="min-w-0 flex-1 text-xs font-semibold text-slate-500 file:mr-3 file:rounded-md file:border-0 file:bg-slate-900 file:px-3 file:py-1.5 file:text-[11px] file:font-bold file:text-white"
									/>
								</span>
							</label>
						</div>
					)}

					{inspection && (
						<>
							{inspection.candidates.length > 1 && (
								<label className="grid gap-1.5 text-xs font-bold text-slate-700">
									候选 ZIP
									<select value={candidateID} disabled={busy} onChange={(event) => void chooseCandidate(event.target.value)} className="rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none">
										<option value="">请选择</option>
										{inspection.candidates.map((candidate) => <option key={candidate.id} value={candidate.id}>{candidate.file_name || candidate.id}{candidate.file_size ? ` (${formatBytes(candidate.file_size)})` : ''}</option>)}
									</select>
								</label>
							)}

							{selectedCandidate && (
								<div className="grid gap-4 border-y border-slate-100 py-4 sm:grid-cols-2">
									<div>
										<p className="text-[10px] font-bold text-slate-400">Mod</p>
										<p className="mt-1 text-sm font-bold text-slate-800">{selectedCandidate.name || selectedCandidate.file_name || '等待读取元数据'}</p>
										<p className="mt-1 break-all font-mono text-[10px] font-semibold text-slate-400">{selectedCandidate.package_name || '-'}</p>
									</div>
									<div>
										<p className="text-[10px] font-bold text-slate-400">安装动作</p>
										<p className="mt-1 text-sm font-bold text-slate-800">{selectedCandidate.action === 'update' ? '更新现有 Mod' : selectedCandidate.action === 'new' ? '新增 Mod（默认禁用）' : '安装时确认 PackageName'}</p>
										{selectedCandidate.version && <p className="mt-1 text-[10px] font-semibold text-slate-400">版本 {selectedCandidate.version}</p>}
									</div>
								</div>
							)}

							{selectedCandidate?.warnings && selectedCandidate.warnings.length > 0 && (
								<div className="grid gap-2 rounded-lg border border-amber-100 bg-amber-50 px-4 py-3 text-xs font-semibold text-amber-800">
									{selectedCandidate.warnings.map((warning) => <p key={warning} className="flex items-start gap-2"><AlertTriangle size={14} className="mt-0.5 shrink-0" />{warning}</p>)}
								</div>
							)}
							<p className="text-right text-[10px] font-semibold text-slate-400">检查有效期至 {new Date(inspection.expires_at).toLocaleString()}</p>
						</>
					)}
				</div>

				<div className="flex items-center justify-between gap-3 border-t border-slate-100 px-5 py-4">
					<button type="button" onClick={inspection ? () => { setInspection(null); setCandidateID(''); setError(''); } : onClose} disabled={busy} className="rounded-lg border border-slate-200 px-4 py-2.5 text-xs font-bold text-slate-600 hover:bg-slate-50 disabled:opacity-40">
						{inspection ? '重新选择' : '取消'}
					</button>
					{inspection ? (
						<button type="button" onClick={() => void startImport()} disabled={busy || !selectedCandidate?.ready} className="inline-flex items-center gap-2 rounded-lg bg-sky-500 px-4 py-2.5 text-xs font-bold text-white hover:bg-sky-600 disabled:opacity-40">
							{busy && <RefreshCw size={14} className="animate-spin" />}确认导入
						</button>
					) : (
						<button type="button" onClick={() => void inspect()} disabled={busy || (!file && !source.trim())} className="inline-flex items-center gap-2 rounded-lg bg-slate-900 px-4 py-2.5 text-xs font-bold text-white hover:bg-slate-800 disabled:opacity-40">
							{busy && <RefreshCw size={14} className="animate-spin" />}检查
						</button>
					)}
				</div>
			</div>
		</div>
	);
};

const WorkshopCard: React.FC<{
  item: WorkshopItem;
  onDetail: () => void;
  onInstall: () => void;
  onInstallEnabled: () => void;
}> = ({ item, onDetail, onInstall, onInstallEnabled }) => (
  <article className="flex min-h-[360px] flex-col overflow-hidden rounded-lg border border-slate-100 bg-white shadow-sm">
    <div className="aspect-[16/9] bg-slate-100">
      {item.preview_url ? (
        <img src={item.preview_url} alt="" className="h-full w-full object-cover" loading="lazy" />
      ) : (
        <div className="flex h-full w-full items-center justify-center text-slate-300">
          <FileArchive size={36} />
        </div>
      )}
    </div>
    <div className="flex flex-1 flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="line-clamp-2 text-sm font-bold text-slate-800">{item.title}</h3>
          <p className="mt-1 font-mono text-[10px] text-slate-400">{item.id}</p>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">
          {item.installed && <StatusBadge status="installed" />}
          {item.enabled && <StatusBadge status="enabled" />}
          {item.update_available && <StatusBadge status="updating" customText="可更新" />}
        </div>
      </div>
      <p className="line-clamp-3 min-h-[48px] text-xs font-medium leading-4 text-slate-500">{item.summary || '-'}</p>
      <TagList tags={item.tags} />
      <div className="mt-auto flex items-center justify-between gap-3 text-[11px] font-semibold text-slate-400">
        <span>{formatBytes(item.file_size)}</span>
        <span>{formatNumber(item.subscriptions)} 订阅</span>
      </div>
      <div className="grid grid-cols-[auto_1fr_1fr] gap-2">
        <button type="button" onClick={onDetail} className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50" aria-label="查看详情">
          <Info size={14} />
        </button>
        <button type="button" onClick={onInstall} className="rounded-lg border border-slate-200 px-3 py-2 text-xs font-bold text-slate-700 hover:bg-slate-50">
          {item.installed ? '重新安装' : '安装'}
        </button>
        <button type="button" onClick={onInstallEnabled} className="rounded-lg bg-slate-900 px-3 py-2 text-xs font-bold text-white hover:bg-slate-800">
          安装并启用
        </button>
      </div>
    </div>
  </article>
);

const ModCard: React.FC<{
  mod: ModItem;
  updateAvailable: boolean;
  onToggle: () => void;
  onDelete: () => void;
  onUpdate: () => void;
}> = ({ mod, updateAvailable, onToggle, onDelete, onUpdate }) => (
  <div className="rounded-lg border border-slate-100 bg-white p-4 shadow-sm">
    <div className="flex items-start justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <ModPreview mod={mod} />
        <div className="min-w-0">
          <p className="truncate text-sm font-bold text-slate-800">{mod.name}</p>
          <p className="mt-1 truncate font-mono text-[11px] text-slate-400">{mod.package_name || mod.workshop_id || mod.id}</p>
        </div>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <StatusBadge status={mod.enabled ? 'enabled' : 'disabled'} />
        {updateAvailable && <StatusBadge status="updating" customText="可更新" />}
      </div>
    </div>
    <div className="mt-4 grid grid-cols-3 gap-2">
      {mod.source === 'workshop' ? (
        <button type="button" onClick={onUpdate} className="rounded-lg border border-slate-200 py-2 text-xs font-bold text-slate-600">
          更新
        </button>
      ) : (
        <span className="rounded-lg border border-slate-100 py-2 text-center text-xs font-bold text-slate-300">-</span>
      )}
      <button type="button" onClick={onToggle} className="rounded-lg border border-slate-200 py-2 text-xs font-bold text-slate-600">
        {mod.enabled ? '禁用' : '启用'}
      </button>
      <button type="button" onClick={onDelete} className="rounded-lg border border-rose-200 py-2 text-xs font-bold text-rose-600">
        删除
      </button>
    </div>
  </div>
);

const ModPreview: React.FC<{ mod: ModItem }> = ({ mod }) => (
  <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-slate-100 text-slate-400">
    {mod.preview_url ? <img src={mod.preview_url} alt="" className="h-full w-full object-cover" loading="lazy" /> : <FileArchive size={16} />}
  </div>
);

const JobProgress: React.FC<{ job: Job }> = ({ job }) => (
  <div className="rounded-lg border border-slate-100 bg-white p-3">
    <div className="flex items-center justify-between gap-3 text-xs font-bold text-slate-700">
      <span className="min-w-0 truncate">{job.message || job.type}</span>
      <StatusBadge status={job.status === 'running' ? 'running_job' : job.status} />
    </div>
    <div className="mt-3 h-2 rounded-full bg-slate-100">
      <div className="h-full rounded-full bg-indigo-500" style={{ width: `${job.progress}%` }} />
    </div>
  </div>
);

const WorkshopDrawer: React.FC<{
  item: WorkshopItem;
  loading: boolean;
  translationLoading: boolean;
  translationError: string | null;
  canTranslate: boolean;
  onClose: () => void;
  onTranslate: (force: boolean) => Promise<void>;
  onInstall: () => void;
  onInstallEnabled: () => void;
}> = ({ item, loading, translationLoading, translationError, canTranslate, onClose, onTranslate, onInstall, onInstallEnabled }) => {
  const [descriptionView, setDescriptionView] = useState<'original' | 'zh-CN'>(item.translation ? 'zh-CN' : 'original');

  useEffect(() => {
    if (item.translation) setDescriptionView('zh-CN');
  }, [item.translation]);

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-slate-950/40">
    <aside className="flex h-full w-full max-w-xl flex-col overflow-y-auto bg-white shadow-xl">
      <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
        <div className="min-w-0">
          <h2 className="truncate text-base font-bold text-slate-900">{item.title}</h2>
          <p className="font-mono text-[11px] font-semibold text-slate-400">{item.id}</p>
        </div>
        <button type="button" onClick={onClose} className="rounded-lg border border-slate-200 p-2 text-slate-500 hover:bg-slate-50" aria-label="关闭">
          <X size={16} />
        </button>
      </div>
      {item.preview_url && <img src={item.preview_url} alt="" className="aspect-[16/9] w-full object-cover" />}
      <div className="flex flex-col gap-5 p-5">
        {loading && (
          <div className="text-xs font-semibold text-slate-400">
            <RefreshCw className="mr-2 inline animate-spin text-sky-500" size={14} />
            正在读取详情...
          </div>
        )}
        <div className="flex flex-wrap gap-2">
          {item.installed && <StatusBadge status="installed" />}
          {item.enabled && <StatusBadge status="enabled" />}
          {item.update_available && <StatusBadge status="updating" customText="可更新" />}
        </div>
        <div>
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            {item.translation ? (
              <div className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-1">
                <button type="button" onClick={() => setDescriptionView('original')} className={`rounded-md px-3 py-1.5 text-[11px] font-bold ${descriptionView === 'original' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-400'}`}>原文</button>
                <button type="button" onClick={() => setDescriptionView('zh-CN')} className={`rounded-md px-3 py-1.5 text-[11px] font-bold ${descriptionView === 'zh-CN' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-400'}`}>中文</button>
              </div>
            ) : <span />}
            {canTranslate && (
              <button
                type="button"
                onClick={() => void onTranslate(Boolean(item.translation))}
                disabled={translationLoading || loading}
                className="inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-bold text-emerald-700 hover:bg-emerald-100 disabled:opacity-40"
              >
                {translationLoading ? <RefreshCw className="animate-spin" size={14} /> : <Languages size={14} />}
                {item.translation ? '重新翻译' : '翻译为中文'}
              </button>
            )}
          </div>
          <p className="whitespace-pre-wrap break-words text-sm leading-6 text-slate-600">
            {descriptionView === 'zh-CN' && item.translation ? item.translation.text : item.summary || '-'}
          </p>
          {item.translation && descriptionView === 'zh-CN' && (
            <p className="mt-3 text-[10px] font-semibold text-slate-400">
              {item.translation.model || '未知模型'} · {item.translation.generated_at ? new Date(item.translation.generated_at).toLocaleString('zh-CN') : '-'}{item.translation.cached ? ' · 缓存' : ''}
            </p>
          )}
          {translationError && <p className="mt-3 rounded-lg border border-rose-100 bg-rose-50 px-3 py-2 text-[11px] font-semibold text-rose-700">{translationError}</p>}
        </div>
        <TagList tags={item.tags} />
        <div className="grid grid-cols-2 gap-3 text-xs font-semibold text-slate-500">
          <InfoTile label="大小" value={formatBytes(item.file_size)} />
          <InfoTile label="订阅" value={formatNumber(item.subscriptions)} />
          <InfoTile label="创建" value={formatSteamTime(item.time_created)} />
          <InfoTile label="更新" value={formatSteamTime(item.time_updated)} />
        </div>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-[auto_1fr_1fr]">
          <a
            href={item.steam_url}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center justify-center gap-2 rounded-lg border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50"
          >
            <ExternalLink size={14} />
            Steam
          </a>
          <button type="button" onClick={onInstall} className="inline-flex items-center justify-center gap-2 rounded-lg border border-slate-200 px-4 py-2 text-xs font-bold text-slate-700 hover:bg-slate-50">
            <PackageCheck size={14} />
            {item.installed ? '重新安装' : '安装'}
          </button>
          <button type="button" onClick={onInstallEnabled} className="inline-flex items-center justify-center gap-2 rounded-lg bg-slate-900 px-4 py-2 text-xs font-bold text-white hover:bg-slate-800">
            <CheckCircle2 size={14} />
            安装并启用
          </button>
        </div>
      </div>
    </aside>
  </div>
  );
};

const TagList: React.FC<{ tags: string[] }> = ({ tags }) => {
  if (tags.length === 0) {
    return <div className="min-h-[24px]" />;
  }
  return (
    <div className="flex min-h-[24px] flex-wrap gap-1.5">
      {tags.slice(0, 4).map((tag) => (
        <span key={tag} className="rounded-md border border-slate-100 bg-slate-50 px-2 py-0.5 text-[10px] font-bold text-slate-500">
          {tag}
        </span>
      ))}
    </div>
  );
};

const InfoTile: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div className="rounded-lg border border-slate-100 bg-slate-50 px-3 py-2">
    <p className="text-[10px] font-bold uppercase text-slate-400">{label}</p>
    <p className="mt-1 truncate text-xs font-bold text-slate-700">{value}</p>
  </div>
);

const formatBytes = (value?: number) => {
  if (!value || value <= 0) return '-';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
};

const formatNumber = (value?: number) => {
  if (!value || value <= 0) return '-';
  return new Intl.NumberFormat('zh-CN', { notation: value >= 10000 ? 'compact' : 'standard' }).format(value);
};

const formatSteamTime = (value?: number) => {
  if (!value || value <= 0) return '-';
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium' }).format(new Date(value * 1000));
};
