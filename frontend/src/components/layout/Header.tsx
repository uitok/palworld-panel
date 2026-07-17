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

const groupLabels = { setup: 'GETTING STARTED', workspace: 'OVERVIEW', world: 'WORLD DATA', system: 'OPERATIONS' } as const;

export const Header: React.FC<HeaderProps> = ({ onMenuClick, onAnnounceClick, onSaveClick, onRestartClick }) => {
  const { autoRefresh, setAutoRefresh, triggerRefresh } = useServerStore();
  const location = useLocation();
  const routeMeta = getRouteMetaByPathname(location.pathname);
  const title = routeMeta?.title || '管理面板';
  const groupLabel = routeMeta ? groupLabels[routeMeta.navGroup] : appConfig.brand.toUpperCase();

  return (
    <header className="pp-topbar">
      <div className="pp-topbar__inner">
        <div className="pp-row pp-grow">
          <button type="button" onClick={onMenuClick} className="pp-button lg:hidden" aria-label="打开导航"><Menu size={17} /></button>
          <div className="pp-grow">
            <div className="pp-topbar__eyebrow">{groupLabel}</div>
            <h2 className="pp-topbar__title pp-truncate">{title}</h2>
          </div>
        </div>
        <div className="pp-topbar__actions">
          <button type="button" onClick={() => setAutoRefresh(!autoRefresh)} className="pp-button hidden sm:inline-flex">
            <span className={`h-2 w-2 rounded-full ${autoRefresh ? 'bg-blue-500' : 'bg-slate-300'}`} />
            <span>{autoRefresh ? '自动刷新' : '已暂停'}</span>
          </button>
          <button type="button" onClick={triggerRefresh} title="同步最新数据" className="pp-button"><RefreshCw size={14} /><span>同步</span></button>
          <button type="button" onClick={onSaveClick} title="保存世界" className="pp-button hidden md:inline-flex"><Save size={14} /><span>保存</span></button>
          <button type="button" onClick={onRestartClick} title="重启服务器" className="pp-button hidden md:inline-flex text-rose-600"><ServerCrash size={14} /><span>重启</span></button>
          <button type="button" onClick={onAnnounceClick} title="广播公告" className="pp-button pp-btn--primary hidden sm:inline-flex"><Megaphone size={14} /><span>广播</span></button>
        </div>
      </div>
    </header>
  );
};
