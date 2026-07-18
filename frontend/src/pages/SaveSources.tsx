import React, { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArchiveRestore, CheckCircle2, Database, FileArchive, Pencil, RefreshCw, Server, Trash2, Upload } from 'lucide-react';
import { getErrorMessage } from '../api/client';
import { saveSourcesApi } from '../api/saveSources';

export const SaveSources: React.FC = () => {
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState('');
  const [notice, setNotice] = useState('');
  const sources = useQuery({ queryKey: ['save-sources'], queryFn: saveSourcesApi.list });
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['save-sources'] });
  const importMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error('请选择包含 Level.sav 的 ZIP 或 TAR 文件');
      return saveSourcesApi.importArchive(file, name);
    },
    onSuccess: () => { setFile(null); setName(''); setNotice('存档已导入，可激活后建立索引。'); void refresh(); },
    onError: (error) => setNotice(getErrorMessage(error)),
  });
  const action = useMutation({
    mutationFn: async ({ type, id, value }: { type: 'activate' | 'rebuild' | 'remove' | 'rename'; id: string; value?: string }) => {
      if (type === 'activate') return saveSourcesApi.activate(id);
      if (type === 'rebuild') return saveSourcesApi.rebuild(id);
      if (type === 'rename') return saveSourcesApi.rename(id, value || '');
      return saveSourcesApi.remove(id);
    },
    onSuccess: () => void refresh(),
    onError: (error) => setNotice(getErrorMessage(error)),
  });

  const active = sources.data?.items.find((item) => item.active);
  const status = sources.data?.active_status;
  return (
    <div className="page-shell">
      <div className="page-titlebar">
        <div><p className="eyebrow">Save workspace</p><h1>存档中心</h1><p>管理服务器存档和导入的本地存档，当前激活源会用于世界数据与配种计算。</p></div>
        <button type="button" className="pp-button" onClick={() => void sources.refetch()}><RefreshCw size={15} />刷新</button>
      </div>

      <section className="status-strip compact-status">
        <div className="server-hero"><div><span className="eyebrow">当前数据源</span><h2>{active?.name || '尚未选择'}</h2><p>{status?.updated_at ? `索引更新于 ${new Date(status.updated_at).toLocaleString('zh-CN')}` : '等待首次索引'}</p></div><span className={`state-pill ${status?.state === 'ready' ? 'ok' : 'warn'}`}>{status?.state || 'unknown'}</span></div>
        <Metric label="玩家" value={status?.counts.players || 0} />
        <Metric label="帕鲁" value={status?.counts.pals || 0} />
        <Metric label="基地" value={status?.counts.bases || 0} />
      </section>

      {notice && <div className="pp-notice">{notice}</div>}

      <div className="content-grid two-column">
        <section className="pp-card">
          <div className="pp-card-head"><div><h2>可用存档</h2><p>服务器源始终跟踪 PalPanel 当前接管的世界。</p></div><Database size={18} /></div>
          <div className="source-list">
            {(sources.data?.items || []).map((source) => (
              <article key={source.id} className={`source-row ${source.active ? 'active' : ''}`}>
                <span className="source-icon">{source.kind === 'server' ? <Server size={18} /> : <FileArchive size={18} />}</span>
                <div className="source-copy"><strong>{source.name}</strong><span>{source.kind === 'server' ? '当前服务器' : '本地归档导入'} · {source.active ? '正在使用' : '未激活'}</span></div>
                <div className="source-actions">
                  <button type="button" className="pp-button" onClick={() => { const next = window.prompt('新的存档名称', source.name); if (next?.trim() && next.trim() !== source.name) action.mutate({ type: 'rename', id: source.id, value: next.trim() }); }}><Pencil size={14} />重命名</button>
                  {!source.active && <button type="button" className="pp-button accent" onClick={() => action.mutate({ type: 'activate', id: source.id })}><CheckCircle2 size={14} />激活</button>}
                  {source.active && <button type="button" className="pp-button" onClick={() => action.mutate({ type: 'rebuild', id: source.id })}><RefreshCw size={14} />重建</button>}
                  {source.kind !== 'server' && !source.active && <button type="button" className="icon-danger" aria-label="删除存档" onClick={() => window.confirm('删除这个导入存档？') && action.mutate({ type: 'remove', id: source.id })}><Trash2 size={15} /></button>}
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="pp-card upload-card">
          <div className="pp-card-head"><div><h2>导入本地存档</h2><p>上传标准 Steam 或服务端世界 ZIP/TAR。</p></div><ArchiveRestore size={18} /></div>
          <label className="field-label">显示名称<input value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：单人世界 2026-07-16" /></label>
          <label className="file-drop"><Upload size={24} /><strong>{file?.name || '选择 ZIP 或 TAR 存档'}</strong><span>支持 .zip、.tar、.tar.gz、.tgz，归档中必须包含 Level.sav</span><input type="file" accept=".zip,.tar,.tar.gz,.tgz,application/zip,application/x-tar,application/gzip" onChange={(event) => setFile(event.target.files?.[0] || null)} /></label>
          <button type="button" disabled={!file || importMutation.isPending} className="pp-button accent wide" onClick={() => importMutation.mutate()}>{importMutation.isPending ? <RefreshCw className="animate-spin" size={15} /> : <Upload size={15} />}导入存档</button>
        </section>
      </div>
    </div>
  );
};

const Metric: React.FC<{ label: string; value: number }> = ({ label, value }) => <div className="metric"><span className="eyebrow">{label}</span><strong>{value}</strong><span>当前索引</span></div>;
