import React, { useEffect, useState } from 'react';
import { ClipboardList, RefreshCw } from 'lucide-react';
import { auditApi } from '../api/audit';
import type { AuditLog } from '../types';
import { DataTable } from '../components/ui/DataTable';
import { StatusBadge } from '../components/ui/StatusBadge';

export const AuditLogs: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const data = await auditApi.list();
    setLogs(data);
    setLoading(false);
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)]">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 className="flex items-center gap-2 text-[15px] font-bold text-slate-800">
              <ClipboardList size={18} className="text-sky-500" />
              操作审计
            </h3>
            <p className="mt-1 text-xs font-medium text-slate-400">记录所有写操作的操作者、角色、来源 IP、结果和时间。</p>
          </div>
          <button type="button" onClick={load} className="flex items-center gap-2 rounded-xl border border-slate-200 px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">
            <RefreshCw size={14} />
            刷新
          </button>
        </div>
      </section>

      <section className="rounded-3xl border border-slate-100 bg-white p-4 shadow-[0_2px_12px_-3px_rgba(15,23,42,0.02)] sm:p-6">
        {loading ? (
          <div className="py-12 text-center text-xs font-semibold text-slate-400">正在读取审计日志...</div>
        ) : (
          <DataTable
            title={`最近操作（${logs.length}）`}
            headers={[
              { key: 'time', label: '时间' },
              { key: 'actor', label: '操作者' },
              { key: 'action', label: '动作' },
              { key: 'target', label: '对象' },
              { key: 'status', label: '结果' },
              { key: 'ip', label: '来源 IP' },
            ]}
            data={logs}
            emptyText="暂无审计记录"
            renderCard={(item) => (
              <div key={item.id} className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="text-xs font-bold text-slate-800">{item.action}</p>
                    <p className="mt-1 text-[11px] text-slate-400">{item.actor} / {item.role}</p>
                  </div>
                  <StatusBadge status={item.status === 'success' ? 'success' : 'failed'} />
                </div>
                <p className="mt-3 font-mono text-[10px] text-slate-400">{item.created_at}</p>
              </div>
            )}
            renderRow={(item) => (
              <tr key={item.id} className="hover:bg-slate-50/50">
                <td className="px-6 py-4 text-xs font-medium text-slate-400">{item.created_at}</td>
                <td className="px-6 py-4 text-xs font-bold text-slate-700">{item.actor} / {item.role}</td>
                <td className="px-6 py-4 font-mono text-[11px] text-slate-600">{item.action}</td>
                <td className="px-6 py-4 text-xs text-slate-500">{item.target || '-'}</td>
                <td className="px-6 py-4">
                  <StatusBadge status={item.status === 'success' ? 'success' : 'failed'} />
                </td>
                <td className="px-6 py-4 font-mono text-[11px] text-slate-400">{item.ip || '-'}</td>
              </tr>
            )}
          />
        )}
      </section>
    </div>
  );
};
