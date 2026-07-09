import React from 'react';
import { NavLink } from 'react-router-dom';
import { Compass, PanelLeftClose, PanelLeftOpen, Search } from 'lucide-react';
import { useServerStore } from '../../store/useServerStore';
import { navGroups } from '../../routes';
import { appConfig } from '../../config/defaults';

interface SidebarProps {
  mobile?: boolean;
  onNavigate?: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({ mobile = false, onNavigate }) => {
  const { isSidebarCollapsed, setIsSidebarCollapsed } = useServerStore();
  const collapsed = !mobile && isSidebarCollapsed;

  return (
    <aside
      className={`flex h-full shrink-0 flex-col justify-between border-r border-slate-100 bg-[#fbfcfd] p-4 transition-all duration-300 ${
        mobile ? 'w-full max-w-[320px]' : collapsed ? 'w-20' : 'w-72'
      }`}
    >
      <div className="flex min-h-0 flex-col gap-5">
        <div className="flex items-center justify-between px-2 py-1">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-sky-500 text-white shadow-md shadow-sky-200">
              <Compass size={20} />
            </div>
            {!collapsed && (
              <div className="min-w-0">
                <h1 className="truncate text-[14px] font-bold leading-tight text-slate-800">{appConfig.brand}</h1>
                <p className="truncate text-[10px] font-medium text-slate-400">Palworld Control Panel</p>
              </div>
            )}
          </div>

          {!mobile && (
            <button
              type="button"
              onClick={() => setIsSidebarCollapsed(!collapsed)}
              className="ml-1 shrink-0 rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
              title={collapsed ? '展开侧边栏' : '收起侧边栏'}
              aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}
            >
              {collapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
            </button>
          )}
        </div>

        <div className="relative px-1">
          <Search
            className={`absolute top-1/2 -translate-y-1/2 text-slate-400 ${
              collapsed ? 'left-1/2 -translate-x-1/2' : 'left-4'
            }`}
            size={15}
          />
          {!collapsed ? (
            <input
              type="text"
              placeholder="搜索功能..."
              disabled
              className="w-full cursor-not-allowed rounded-xl border-none bg-slate-100/60 py-2 pl-9 pr-4 text-xs font-medium text-slate-500 placeholder:text-slate-400 focus:outline-none"
            />
          ) : (
            <div className="h-9 w-full cursor-not-allowed rounded-xl border border-dashed border-slate-200/20 bg-slate-100/30" />
          )}
        </div>

        <nav className="flex max-h-[65vh] flex-col gap-5 overflow-y-auto pr-1" aria-label="主导航">
          {navGroups.map((group) => (
            <div key={group.id} className="flex flex-col gap-1.5">
              {!collapsed ? (
                <span className="px-3 text-[9px] font-extrabold uppercase text-slate-400/80">{group.title}</span>
              ) : (
                <div className="mx-1.5 my-1 h-px bg-slate-100" />
              )}

              <div className="flex flex-col gap-0.5">
                {group.items.map((item) => (
                  <NavLink
                    key={item.id}
                    to={item.path}
                    title={collapsed ? item.navLabel : undefined}
                    onClick={onNavigate}
                    className={({ isActive }) =>
                      `flex items-center rounded-xl text-left text-[13px] font-medium transition-all duration-200 ${
                        collapsed ? 'justify-center p-2.5' : 'gap-3 px-3.5 py-2.5'
                      } ${
                        isActive
                          ? 'border border-slate-100/70 bg-white text-slate-900 shadow-[0_4px_16px_-4px_rgba(14,165,233,0.12)]'
                          : 'text-slate-500 hover:bg-slate-100/50 hover:text-slate-800'
                      }`
                    }
                  >
                    {({ isActive }) => (
                      <>
                        <span className={`shrink-0 ${isActive ? 'text-sky-500' : 'text-slate-400'}`}>{item.icon}</span>
                        {!collapsed && <span className="truncate">{item.navLabel}</span>}
                      </>
                    )}
                  </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>
      </div>

      <div className={`flex shrink-0 items-center border-t border-slate-100/80 pt-4 ${collapsed ? 'justify-center px-0' : 'gap-3 px-2'}`}>
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-slate-100 bg-slate-900 text-xs font-bold text-white">
          {appConfig.brand.slice(0, 2).toUpperCase()}
        </div>
        {!collapsed && (
          <div className="min-w-0 flex-1">
            <h4 className="truncate text-xs font-semibold text-slate-700">Admin</h4>
            <p className="truncate text-[9px] text-slate-400">Bearer token auth</p>
          </div>
        )}
      </div>
    </aside>
  );
};
