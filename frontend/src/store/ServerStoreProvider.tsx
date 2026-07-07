import React, { useEffect, useState } from 'react';
import type { Job, ServerMetrics, ServerStatus } from '../types';
import { ServerStoreContext } from './serverStoreContext';

const readLocalStorage = (key: string) => {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(key);
};

export const ServerStoreProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [panelToken, setPanelTokenState] = useState<string>(() => {
    return readLocalStorage('palsphere_token') || import.meta.env.VITE_PANEL_TOKEN || '';
  });
  const [authError, setAuthError] = useState(false);
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
    setAuthError(false);
  };

  const clearAuthError = () => setAuthError(false);

  useEffect(() => {
    const onAuthError = () => setAuthError(true);
    window.addEventListener('palsphere:auth-error', onAuthError);
    return () => window.removeEventListener('palsphere:auth-error', onAuthError);
  }, []);

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
        authError,
        clearAuthError,
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
