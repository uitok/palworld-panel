import React from 'react';
import { useLocation } from 'react-router-dom';
import { Menu, Megaphone, RefreshCw, Save, ServerCrash } from 'lucide-react';
import { useServerStore } from '../../store/useServerStore';
import { getRouteMetaByPathname } from '../../routes';
import { appConfig } from '../../config/defaults';

interface HeaderProps {
  onMenuClick: () => void;
  onAnnounceClick: () => void;
  onSaveClick: () => void;
  onRestartClick: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  onMenuClick,
  onAnnounceClick,
  onSaveClick,
  onRestartClick,
}) => {
  const { autoRefresh, setAutoRefresh, triggerRefresh } = useServerStore();
  const location = useLocation();
  const routeMeta = getRouteMetaByPathname(location.pathname);
  const title = routeMeta?.title || '管理面板';

  return (
    <header className="shrink-0 border-b border-slate-100 bg-white px-4 py-4 sm:px-6 lg:min-h-20 lg:px-8">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <button
            type="button"
            onClick={onMenuClick}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-slate-200 text-slate-600 lg:hidden"
            aria-label="打开导航"
          >
            <Menu size={18} />
          </button>

          <div className="min-w-0">
            <span className="flex items-center gap-1.5 text-[11px] font-semibold uppercase text-slate-400">
              <span className="h-1.5 w-1.5 rounded-full bg-sky-500" />
              {appConfig.brand} Admin
            </span>
            <h2 className="truncate text-lg font-bold leading-tight text-slate-800 sm:text-xl">{title}</h2>
          </div>
        </div>

        <div className="flex min-w-0 items-center justify-end gap-2">
          <button
            type="button"
            onClick={() => setAutoRefresh(!autoRefresh)}
            title={autoRefresh ? '自动刷新已开启' : '自动刷新已暂停'}
            className="flex h-9 items-center gap-2 rounded-xl px-2.5 text-left transition-all hover:bg-slate-50"
          >
            <span className={`h-2 w-2 shrink-0 rounded-full ${autoRefresh ? 'animate-pulse bg-emerald-500' : 'bg-slate-300'}`} />
            <span className="hidden whitespace-nowrap text-xs font-semibold text-slate-500 md:inline">
              {autoRefresh ? '自动刷新 5s' : '刷新暂停'}
            </span>
          </button>

          <button
            type="button"
            onClick={triggerRefresh}
            title="同步最新数据"
            className="flex h-9 items-center gap-2 rounded-xl border border-slate-200/80 px-3 text-xs font-semibold text-slate-600 transition-all hover:bg-slate-50 active:scale-95"
          >
            <RefreshCw size={14} />
            <span className="hidden sm:inline">同步</span>
          </button>

          <button
            type="button"
            onClick={onSaveClick}
            title="保存世界"
            className="hidden h-9 items-center gap-2 rounded-xl border border-slate-200/80 px-3 text-xs font-semibold text-slate-600 transition-all hover:bg-slate-50 active:scale-95 sm:flex"
          >
            <Save size={14} />
            <span className="hidden xl:inline">保存世界</span>
          </button>

          <button
            type="button"
            onClick={onRestartClick}
            title="重启服务器"
            className="hidden h-9 items-center gap-2 rounded-xl border border-rose-200 px-3 text-xs font-semibold text-rose-600 transition-all hover:bg-rose-50 active:scale-95 md:flex"
          >
            <ServerCrash size={14} />
            <span className="hidden xl:inline">重启</span>
          </button>

          <button
            type="button"
            onClick={onAnnounceClick}
            title="广播公告"
            className="hidden h-9 items-center gap-2 rounded-xl bg-sky-500 px-3 text-xs font-semibold text-white transition-all hover:bg-sky-600 active:scale-95 sm:flex lg:px-5"
          >
            <Megaphone size={14} />
            <span className="hidden xl:inline">广播公告</span>
          </button>
        </div>
      </div>
    </header>
  );
};
