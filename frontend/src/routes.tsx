import type React from 'react';
import {
  Activity,
  Archive,
  ClipboardList,
  Home,
  LayoutDashboard,
  ListTodo,
  Puzzle,
  Settings as SettingsIcon,
  Shield,
  Sparkles,
  Sword,
  UserX,
  Users,
} from 'lucide-react';
import { Backups } from './pages/Backups';
import { AuditLogs } from './pages/AuditLogs';
import { BanList } from './pages/BanList';
import { Bases } from './pages/Bases';
import { Dashboard } from './pages/Dashboard';
import { Mods } from './pages/Mods';
import { Monitor } from './pages/Monitor';
import { Pals } from './pages/Pals';
import { Players } from './pages/Players';
import { Security } from './pages/Security';
import { Settings } from './pages/Settings';
import { Setup } from './pages/Setup';
import { TaskQueue } from './pages/TaskQueue';

export interface AppRoute {
  id: string;
  path: string;
  title: string;
  navLabel: string;
  navGroup: 'setup' | 'server' | 'world' | 'system';
  icon: React.ReactNode;
  element: React.ReactElement;
}

export const appRoutes: AppRoute[] = [
  {
    id: 'setup',
    path: '/setup',
    title: '开服向导',
    navLabel: '开服向导',
    navGroup: 'setup',
    icon: <Sparkles size={18} />,
    element: <Setup />,
  },
  {
    id: 'dashboard',
    path: '/dashboard',
    title: '系统总览',
    navLabel: '总览',
    navGroup: 'server',
    icon: <LayoutDashboard size={18} />,
    element: <Dashboard />,
  },
  {
    id: 'monitor',
    path: '/monitor',
    title: '实时监控',
    navLabel: '实时监控',
    navGroup: 'server',
    icon: <Activity size={18} />,
    element: <Monitor />,
  },
  {
    id: 'players',
    path: '/players',
    title: '玩家管理',
    navLabel: '玩家管理',
    navGroup: 'server',
    icon: <Users size={18} />,
    element: <Players />,
  },
  {
    id: 'banlist',
    path: '/banlist',
    title: '封禁列表',
    navLabel: '封禁列表',
    navGroup: 'server',
    icon: <UserX size={18} />,
    element: <BanList />,
  },
  {
    id: 'pals',
    path: '/pals',
    title: '帕鲁管理',
    navLabel: '帕鲁管理',
    navGroup: 'world',
    icon: <Sword size={18} />,
    element: <Pals />,
  },
  {
    id: 'bases',
    path: '/bases',
    title: '基地列表',
    navLabel: '基地列表',
    navGroup: 'world',
    icon: <Home size={18} />,
    element: <Bases />,
  },
  {
    id: 'mods',
    path: '/mods',
    title: 'Mod 管理',
    navLabel: 'Mod 管理',
    navGroup: 'world',
    icon: <Puzzle size={18} />,
    element: <Mods />,
  },
  {
    id: 'security',
    path: '/security',
    title: 'PalDefender 安全',
    navLabel: '安全防护',
    navGroup: 'system',
    icon: <Shield size={18} />,
    element: <Security />,
  },
  {
    id: 'backups',
    path: '/backups',
    title: '备份管理',
    navLabel: '备份管理',
    navGroup: 'system',
    icon: <Archive size={18} />,
    element: <Backups />,
  },
  {
    id: 'tasks',
    path: '/tasks',
    title: '任务队列',
    navLabel: '任务队列',
    navGroup: 'system',
    icon: <ListTodo size={18} />,
    element: <TaskQueue />,
  },
  {
    id: 'audit',
    path: '/audit',
    title: '操作审计',
    navLabel: '操作审计',
    navGroup: 'system',
    icon: <ClipboardList size={18} />,
    element: <AuditLogs />,
  },
  {
    id: 'settings',
    path: '/settings',
    title: '服务器设置',
    navLabel: '服务器设置',
    navGroup: 'system',
    icon: <SettingsIcon size={18} />,
    element: <Settings />,
  },
];

export const navGroups: Array<{ id: AppRoute['navGroup']; title: string; items: AppRoute[] }> = [
  { id: 'setup', title: '开服 SETUP', items: appRoutes.filter((route) => route.navGroup === 'setup') },
  { id: 'server', title: '服务器 SERVER', items: appRoutes.filter((route) => route.navGroup === 'server') },
  { id: 'world', title: '世界 WORLD', items: appRoutes.filter((route) => route.navGroup === 'world') },
  { id: 'system', title: '系统 SYSTEM', items: appRoutes.filter((route) => route.navGroup === 'system') },
];

export const getRouteMetaByPathname = (pathname: string): AppRoute | undefined => {
  const normalizedPath = pathname.replace(/\/+$/, '') || '/dashboard';
  return appRoutes.find((route) => route.path === normalizedPath);
};
