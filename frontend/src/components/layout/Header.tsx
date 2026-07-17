import React from 'react';
import { useLocation } from 'react-router-dom';
import { ChevronRight, Menu, Megaphone, RefreshCw, Save, ServerCrash } from 'lucide-react';
import { useServerStore } from '../../store/useServerStore';
import { getRouteMetaByPathname } from '../../routes';
import { appConfig } from '../../config/defaults';

interface HeaderProps {
  onMenuClick: () => void;
  onAnnounceClick: () => void;
  onSaveClick: () => void;
  onRestartClick: () => void;
}

const groupLabels = { setup: '开始', workspace: '工作台', world: '世界管理', system: '运维与安全' } as const;

export const Header: React.FC<HeaderProps> = ({ onMenuClick, onAnnounceClick, onSaveClick, onRestartClick }) => {
  const { autoRefresh, setAutoRefresh, triggerRefresh } = useServerStore();
  const location = useLocation();
  const routeMeta = getRouteMetaByPathname(location.pathname);
  const title = routeMeta?.title || '管理面板';
  const groupLabel = routeMeta ? groupLabels[routeMeta.navGroup] : appConfig.brand;

  return (
    <header className="sticky top-0 z-30 flex h-[72px] shrink-0 items-center border-b border-slate-100 bg-[#f7f8f4]/94 px-4 backdrop-blur-xl sm:px-6 lg:px-7">
      <div className="flex w-full items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <button type="button" onClick={onMenuClick} className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-700 shadow-sm lg:hidden" aria-label="打开导航"><Menu size={19} /></button>
          <div className="min-w-0">
            <span className="flex items-center gap-1 text-[11px] font-semibold text-slate-500"><span>{groupLabel}</span><ChevronRight size={11} className="text-slate-300" /><span className="truncate text-sky-700">{title}</span></span>
            <h2 className="truncate text-xl font-bold leading-tight tracking-[-0.02em] text-slate-900">{title}</h2>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => setAutoRefresh(!autoRefresh)} className="hidden h-9 items-center gap-2 rounded-lg px-2.5 text-xs font-semibold text-slate-600 hover:bg-white sm:flex"><span className={`h-2 w-2 rounded-full ${autoRefresh ? 'bg-emerald-500' : 'bg-slate-300'}`} />{autoRefresh ? '自动刷新' : '已暂停'}</button>
          <button type="button" onClick={triggerRefresh} title="同步最新数据" className="top-icon-button"><RefreshCw size={14} /><span className="hidden sm:inline">同步</span></button>
          <button type="button" onClick={onSaveClick} title="保存世界" className="top-icon-button hidden md:flex"><Save size={14} /><span className="hidden xl:inline">保存</span></button>
          <button type="button" onClick={onRestartClick} title="重启服务器" className="top-icon-button hidden text-rose-600 md:flex"><ServerCrash size={14} /><span className="hidden xl:inline">重启</span></button>
          <button type="button" onClick={onAnnounceClick} title="广播公告" className="top-icon-button accent hidden sm:flex"><Megaphone size={14} /><span className="hidden xl:inline">广播</span></button>
        </div>
      </div>
    </header>
  );
};
