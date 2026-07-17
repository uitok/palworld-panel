import React from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { Compass, LogOut, PanelLeftClose, PanelLeftOpen, Search, X } from 'lucide-react';
import { useServerStore } from '../../store/useServerStore';
import { navGroups } from '../../routes';
import { appConfig } from '../../config/defaults';

interface SidebarProps {
  mobile?: boolean;
  onNavigate?: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({ mobile = false, onNavigate }) => {
  const { isSidebarCollapsed, setIsSidebarCollapsed, session, logout } = useServerStore();
  const location = useLocation();
  const [query, setQuery] = React.useState('');
  const collapsed = !mobile && isSidebarCollapsed;
  const currentPath = location.pathname.replace(/\/+$/, '') || '/';
  const normalizedQuery = query.trim().toLocaleLowerCase('zh-CN');
  const visibleGroups = React.useMemo(
    () =>
      navGroups
        .map((group) => ({
          ...group,
          items: group.items.filter((item) => {
            if (!normalizedQuery) return true;
            return `${item.navLabel} ${item.title} ${item.id}`.toLocaleLowerCase('zh-CN').includes(normalizedQuery);
          }),
        }))
        .filter((group) => group.items.length > 0),
    [normalizedQuery],
  );

  return (
    <aside
      className={`relative flex h-full shrink-0 flex-col overflow-hidden bg-slate-950 text-white transition-[width] duration-300 ${
        mobile ? 'w-full max-w-[300px]' : collapsed ? 'w-[72px]' : 'w-56'
      }`}
    >
      <div className="pointer-events-none absolute inset-x-0 top-0 h-48 bg-[radial-gradient(circle_at_top_left,rgba(57,200,184,0.16),transparent_68%)]" />

      <div className={`relative flex items-center border-b border-white/[0.08] ${collapsed ? 'flex-col gap-3 px-3 py-4' : 'justify-between px-4 py-4'}`}>
        <NavLink
          to="/dashboard"
          onClick={onNavigate}
          className={`flex min-w-0 items-center ${collapsed ? 'justify-center' : 'gap-3'}`}
          aria-label={`${appConfig.brand} 总览`}
        >
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-sky-400 to-blue-500 text-white shadow-lg shadow-sky-950/25 ring-1 ring-white/15">
            <Compass size={20} strokeWidth={2.2} />
          </span>
          {!collapsed && (
            <span className="min-w-0">
              <strong className="block truncate text-[15px] font-bold tracking-tight text-white">{appConfig.brand}</strong>
              <span className="block truncate text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-400">
                Server control
              </span>
            </span>
          )}
        </NavLink>

        {!mobile && (
          <button
            type="button"
            onClick={() => setIsSidebarCollapsed(!collapsed)}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-white/[0.08] hover:text-white"
            title={collapsed ? '展开侧边栏' : '收起侧边栏'}
            aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}
          >
            {collapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
          </button>
        )}
      </div>

      <div className="relative flex min-h-0 flex-1 flex-col px-3 py-4">
        {!collapsed && (
          <div className="relative mb-4">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={15} />
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索功能"
              aria-label="搜索功能"
              className="h-10 w-full rounded-xl border border-white/[0.08] bg-white/[0.055] pl-9 pr-9 text-xs font-medium text-white outline-none placeholder:text-slate-500 hover:bg-white/[0.075] focus:border-sky-400/60 focus:bg-white/[0.08] focus:ring-2 focus:ring-sky-400/10"
            />
            {query && (
              <button
                type="button"
                onClick={() => setQuery('')}
                className="absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-lg text-slate-500 hover:bg-white/[0.08] hover:text-white"
                aria-label="清除搜索"
              >
                <X size={14} />
              </button>
            )}
          </div>
        )}

        <nav className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto pr-1" aria-label="主导航">
          {visibleGroups.map((group) => (
            <div key={group.id} className="flex flex-col gap-1.5">
              {!collapsed ? (
                <span className="px-3 text-[10px] font-bold uppercase tracking-[0.16em] text-slate-500">{group.title}</span>
              ) : (
                <div className="mx-2 my-1 h-px bg-white/[0.08]" />
              )}

              <div className="flex flex-col gap-1">
                {group.items.map((item) => {
                  const routeActive = currentPath === item.path || item.activePaths?.includes(currentPath);
                  return (
                    <NavLink
                      key={item.id}
                      to={item.path}
                      title={collapsed ? item.navLabel : undefined}
                      onClick={onNavigate}
                      className={`group relative flex min-h-10 items-center overflow-hidden rounded-xl text-left text-[13px] font-semibold transition-colors ${
                        collapsed ? 'justify-center px-2.5 py-2' : 'gap-3 px-3 py-2.5'
                      } ${
                        routeActive
                          ? 'bg-white text-slate-950 shadow-[0_10px_26px_-16px_rgba(0,0,0,0.85)]'
                          : 'text-slate-300 hover:bg-white/[0.065] hover:text-white'
                      }`}
                    >
                      {routeActive && <span className="absolute inset-y-2 left-0 w-0.5 rounded-r-full bg-sky-500" />}
                      <span className={`shrink-0 transition-colors ${routeActive ? 'text-sky-600' : 'text-slate-500 group-hover:text-slate-200'}`}>
                        {item.icon}
                      </span>
                      {!collapsed && <span className="truncate">{item.navLabel}</span>}
                    </NavLink>
                  );
                })}
              </div>
            </div>
          ))}

          {!collapsed && visibleGroups.length === 0 && (
            <div className="rounded-xl border border-dashed border-white/10 px-4 py-8 text-center text-xs font-medium leading-5 text-slate-500">
              没有找到相关功能
            </div>
          )}
        </nav>
      </div>

      <div className={`relative border-t border-white/[0.08] p-3 ${collapsed ? 'flex flex-col items-center gap-2' : 'flex items-center gap-3'}`}>
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white/[0.08] text-xs font-bold text-sky-300 ring-1 ring-white/[0.08]">
          {(session?.name || appConfig.brand).slice(0, 2).toUpperCase()}
        </div>
        {!collapsed && (
          <div className="min-w-0 flex-1">
            <h4 className="truncate text-xs font-semibold text-white">{session?.name || 'Admin'}</h4>
            <p className="truncate text-[10px] font-medium text-slate-500">管理员会话</p>
          </div>
        )}
        <button
          type="button"
          onClick={() => void logout()}
          title="退出登录"
          aria-label="退出登录"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-slate-500 transition-colors hover:bg-rose-500/10 hover:text-rose-300"
        >
          <LogOut size={16} />
        </button>
      </div>
    </aside>
  );
};
