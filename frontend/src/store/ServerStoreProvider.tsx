import React, { useEffect, useState } from 'react';
import type { Job, ServerMetrics, ServerStatus, SessionInfo } from '../types';
import { appEvents, readAppStorage, writeAppStorage } from '../config/defaults';
import { authApi } from '../api/auth';
import { ServerStoreContext } from './serverStoreContext';

export const ServerStoreProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [panelToken, setPanelTokenState] = useState<string>(() => {
    return readAppStorage('token') || import.meta.env.VITE_PANEL_TOKEN || '';
  });
  const [authError, setAuthError] = useState(false);
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [autoRefresh, setAutoRefreshState] = useState<boolean>(true);
  const [refreshKey, setRefreshKey] = useState<number>(0);
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [metrics, setMetrics] = useState<ServerMetrics | null>(null);
  const [jobs, setJobsState] = useState<Job[]>([]);
  const [isSidebarCollapsed, setIsSidebarCollapsedState] = useState<boolean>(() => {
    return readAppStorage('sidebarCollapsed') === 'true';
  });

  const setPanelToken = (token: string) => {
    setPanelTokenState(token);
    writeAppStorage('token', token);
    setAuthError(false);
  };

  const clearAuthError = () => setAuthError(false);

  useEffect(() => {
    let active = true;
    setSession(null);
    if (!panelToken) {
      return () => {
        active = false;
      };
    }
    void authApi.me()
      .then((nextSession) => {
        if (active) setSession(nextSession);
      })
      .catch(() => {
        if (active) setSession(null);
      });
    return () => {
      active = false;
    };
  }, [panelToken]);

  useEffect(() => {
    const onAuthError = () => setAuthError(true);
    window.addEventListener(appEvents.authError, onAuthError);
    window.addEventListener(appEvents.legacyAuthError, onAuthError);
    return () => {
      window.removeEventListener(appEvents.authError, onAuthError);
      window.removeEventListener(appEvents.legacyAuthError, onAuthError);
    };
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
    writeAppStorage('sidebarCollapsed', String(collapsed));
  };

  return (
    <ServerStoreContext.Provider
      value={{
        panelToken,
        setPanelToken,
        authError,
        clearAuthError,
        session,
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
