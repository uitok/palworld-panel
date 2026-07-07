import React, { createContext, useContext, useState } from 'react';
import { ServerStatus, ServerMetrics, Job } from '../types';

interface ServerStoreContextType {
  panelToken: string;
  setPanelToken: (token: string) => void;
  autoRefresh: boolean;
  setAutoRefresh: (auto: boolean) => void;
  refreshKey: number;
  triggerRefresh: () => void;
  status: ServerStatus | null;
  setStatus: (status: ServerStatus) => void;
  metrics: ServerMetrics | null;
  setMetrics: (metrics: ServerMetrics) => void;
  jobs: Job[];
  setJobs: (jobs: Job[]) => void;
  isSidebarCollapsed: boolean;
  setIsSidebarCollapsed: (collapsed: boolean) => void;
}

const ServerStoreContext = createContext<ServerStoreContextType | undefined>(undefined);

const readLocalStorage = (key: string) => {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(key);
};

export const ServerStoreProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [panelToken, setPanelTokenState] = useState<string>(() => {
    return readLocalStorage('palsphere_token') || import.meta.env.VITE_PANEL_TOKEN || 'change-me';
  });
  const [autoRefresh, setAutoRefreshState] = useState<boolean>(true);
  const [refreshKey, setRefreshKey] = useState<number>(0);
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [metrics, setMetrics] = useState<ServerMetrics | null>(null);
  const [jobs, setJobsState] = useState<Job[]>([]);
  const [isSidebarCollapsed, setIsSidebarCollapsedState] = useState<boolean>(() => {
    return readLocalStorage('palsphere_sidebar_collapsed') === 'true';
  });

  const setPanelToken = (token: string) => {
    setPanelTokenState(token);
    localStorage.setItem('palsphere_token', token);
  };

  const setAutoRefresh = (auto: boolean) => {
    setAutoRefreshState(auto);
  };

  const triggerRefresh = () => {
    setRefreshKey((prev) => prev + 1);
  };

  const setJobs = (nextJobs: Job[]) => {
    setJobsState(Array.isArray(nextJobs) ? nextJobs : []);
  };

  const setIsSidebarCollapsed = (collapsed: boolean) => {
    setIsSidebarCollapsedState(collapsed);
    localStorage.setItem('palsphere_sidebar_collapsed', String(collapsed));
  };

  return (
    <ServerStoreContext.Provider
      value={{
        panelToken,
        setPanelToken,
        autoRefresh,
        setAutoRefresh,
        refreshKey,
        triggerRefresh,
        status,
        setStatus,
        metrics,
        setMetrics,
        jobs,
        setJobs,
        isSidebarCollapsed,
        setIsSidebarCollapsed,
      }}
    >
      {children}
    </ServerStoreContext.Provider>
  );
};

export const useServerStore = () => {
  const context = useContext(ServerStoreContext);
  if (context === undefined) {
    throw new Error('useServerStore must be used within a ServerStoreProvider');
  }
  return context;
};
