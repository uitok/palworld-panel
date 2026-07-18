import React, { useEffect, useState } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { ChevronDown, LogOut, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import { useServerStore } from '../../store/useServerStore';
import { appRoutes, type AppRoute } from '../../routes';
import { appConfig } from '../../config/defaults';

interface SidebarProps {
  mobile?: boolean;
  onNavigate?: () => void;
}

const formatUptime = (seconds?: number) => {
  if (!seconds) return '—';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  return days > 0 ? `${days}d ${hours}h` : `${hours}h`;
};

interface SidebarEntry {
  id: string;
  label: string;
  routeIDs: string[];
}

const sidebarGroups: Array<{ id: string; title: string; entries: SidebarEntry[] }> = [
  { id: 'setup', title: '开始', entries: [{ id: 'setup', label: '开服向导', routeIDs: ['setup'] }] },
  { id: 'workspace', title: '工作台', entries: [{ id: 'server-center', label: '服务器中心', routeIDs: ['dashboard', 'monitor', 'community-servers'] }] },
  {
    id: 'world',
    title: '世界管理',
    entries: [
      { id: 'players-world', label: '玩家与世界', routeIDs: ['player-center', 'world-archive'] },
      { id: 'saves-breeding', label: '存档管理工具', routeIDs: ['save-sources', 'pal-inventory', 'breeding', 'live-map'] },
      { id: 'mods', label: 'Mod 管理', routeIDs: ['mods'] },
    ],
  },
  {
    id: 'system',
    title: '运维与安全',
    entries: [
      { id: 'backup-tasks', label: '备份计划', routeIDs: ['backups', 'tasks'] },
      { id: 'security-audit', label: '安全与审计', routeIDs: ['security', 'banlist', 'audit'] },
      { id: 'settings', label: '系统设置', routeIDs: ['settings'] },
    ],
  },
];

const routesByID = new Map(appRoutes.map((route) => [route.id, route]));

const routeIsActive = (route: AppRoute, currentPath: string) =>
  currentPath === route.path || Boolean(route.activePaths?.includes(currentPath));

export const Sidebar: React.FC<SidebarProps> = ({ mobile = false, onNavigate }) => {
  const { isSidebarCollapsed, setIsSidebarCollapsed, session, logout, status, metrics } = useServerStore();
  const location = useLocation();
  const collapsed = !mobile && isSidebarCollapsed;
  const currentPath = location.pathname.replace(/\/+$/, '') || '/';
  const running = status?.status === 'running';
  const online = metrics?.current_players || 0;
  const capacity = metrics?.max_players || 32;
  const [expandedEntries, setExpandedEntries] = useState<Set<string>>(new Set());

  useEffect(() => {
    const activeEntry = sidebarGroups
      .flatMap((group) => group.entries)
      .find((entry) => entry.routeIDs.some((routeID) => {
        const route = routesByID.get(routeID);
        return route ? routeIsActive(route, currentPath) : false;
      }));
    if (activeEntry && activeEntry.routeIDs.length > 1) {
      setExpandedEntries((current) => {
        if (current.has(activeEntry.id)) return current;
        const next = new Set(current);
        next.add(activeEntry.id);
        return next;
      });
    }
  }, [currentPath]);

  const toggleEntry = (entryID: string) => {
    setExpandedEntries((current) => {
      const next = new Set(current);
      if (next.has(entryID)) next.delete(entryID);
      else next.add(entryID);
      return next;
    });
  };

  return (
    <aside className={`pp-rail ${mobile ? 'is-mobile' : ''} ${collapsed ? 'is-collapsed' : ''}`}>
      <div className="pp-brandmark">
        <NavLink to="/dashboard" onClick={onNavigate} className="pp-brandmark__logo" aria-label={`${appConfig.brand} 总览`}>
          <img src="/brand/palpanel-mark.svg" alt="" width="32" height="32" />
        </NavLink>
        <NavLink to="/dashboard" onClick={onNavigate} className="pp-brandmark__copy" aria-hidden={collapsed}>
          <span className="pp-brandmark__name">{appConfig.brand}</span>
          <span className="pp-brandmark__tag">dev · server control</span>
        </NavLink>
        {!mobile && (
          <button
            type="button"
            onClick={() => setIsSidebarCollapsed(!collapsed)}
            className="pp-rail-toggle"
            title={collapsed ? '展开侧边栏' : '收起侧边栏'}
            aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}
          >
            {collapsed ? <PanelLeftOpen size={15} /> : <PanelLeftClose size={15} />}
          </button>
        )}
      </div>

      <nav className="pp-nav" aria-label="主导航">
        {sidebarGroups.map((group) => (
          <React.Fragment key={group.id}>
            <div className="pp-nav__group">{group.title}</div>
            {group.entries.map((entry) => {
              const routes = entry.routeIDs.map((routeID) => routesByID.get(routeID)).filter((route): route is AppRoute => Boolean(route));
              const primaryRoute = routes[0];
              if (!primaryRoute) return null;
              const active = routes.some((route) => routeIsActive(route, currentPath));
              if (routes.length === 1 || collapsed) {
                return (
                  <NavLink
                    key={entry.id}
                    to={primaryRoute.path}
                    onClick={onNavigate}
                    title={collapsed ? entry.label : undefined}
                    className={`pp-nav__item ${active ? 'is-active' : ''}`}
                  >
                    {primaryRoute.icon}
                    <span className="pp-nav__label pp-truncate">{entry.label}</span>
                  </NavLink>
                );
              }

              const expanded = expandedEntries.has(entry.id);
              return (
                <React.Fragment key={entry.id}>
                  <button
                    type="button"
                    onClick={() => toggleEntry(entry.id)}
                    className={`pp-nav__item pp-nav__item--cluster ${active ? 'is-active' : ''}`}
                    aria-expanded={expanded}
                  >
                    {primaryRoute.icon}
                    <span className="pp-nav__label pp-truncate">{entry.label}</span>
                    <ChevronDown size={14} className={`pp-nav__chevron ${expanded ? 'is-open' : ''}`} />
                  </button>
                  {expanded && (
                    <div className="pp-nav__sub">
                      {routes.map((route) => (
                        <NavLink
                          key={route.id}
                          to={route.path}
                          onClick={onNavigate}
                          className={`pp-nav__subitem ${routeIsActive(route, currentPath) ? 'is-active' : ''}`}
                        >
                          {route.navLabel}
                        </NavLink>
                      ))}
                    </div>
                  )}
                </React.Fragment>
              );
            })}
          </React.Fragment>
        ))}
      </nav>

      {!collapsed && (
        <section className="pp-heartbeat" aria-label="服务器心跳">
          <div className="pp-heartbeat__top">
            <span className={`pp-pulse ${running ? '' : 'is-down'}`} />
            <span className="pp-heartbeat__label">服务器心跳</span>
            <span className="pp-heartbeat__state">{running ? '运行中' : '已停止'}</span>
          </div>
          <div className="pp-heartbeat__meta">
            <span><strong>{online}/{capacity}</strong>在线</span>
            <span className="pp-right"><strong>{formatUptime(metrics?.uptime)}</strong>运行时间</span>
          </div>
        </section>
      )}

      <div className="pp-rail-account">
        <span className="pp-rail-account__avatar">{(session?.name || appConfig.brand).slice(0, 2).toUpperCase()}</span>
        <span className="pp-rail-account__copy">
          <strong>{session?.name || 'Admin'}</strong>
          <span>管理员会话</span>
        </span>
        <button type="button" onClick={() => void logout()} className="pp-rail-account__logout" title="退出登录" aria-label="退出登录">
          <LogOut size={15} />
        </button>
      </div>
    </aside>
  );
};
