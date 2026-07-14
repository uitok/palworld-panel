import React, { useCallback, useEffect, useState } from 'react';
import type { Job, ServerMetrics, ServerStatus, SessionInfo } from '../types';
import { appEvents, readAppStorage, writeAppStorage } from '../config/defaults';
import { authApi } from '../api/auth';
import { ServerStoreContext } from './serverStoreContext';

type AuthState = 'loading' | 'register' | 'login' | 'authenticated' | 'unavailable';

export const ServerStoreProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [authState, setAuthState] = useState<AuthState>('loading');
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [autoRefresh, setAutoRefreshState] = useState<boolean>(true);
  const [refreshKey, setRefreshKey] = useState<number>(0);
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [metrics, setMetrics] = useState<ServerMetrics | null>(null);
  const [jobs, setJobsState] = useState<Job[]>([]);
  const [isSidebarCollapsed, setIsSidebarCollapsedState] = useState<boolean>(() => {
    return readAppStorage('sidebarCollapsed') === 'true';
  });

  const refreshAuthentication = useCallback(async () => {
    try {
      const next = await authApi.status();
      if (next.authenticated && next.user) {
        setSession(next.user);
        setAuthState('authenticated');
      } else {
        setSession(null);
        setAuthState(next.initialized ? 'login' : 'register');
      }
    } catch {
      setSession(null);
      setAuthState('unavailable');
    }
  }, []);

  useEffect(() => {
    void refreshAuthentication();
  }, [refreshAuthentication]);

  useEffect(() => {
    const onAuthError = () => {
      setSession(null);
      setAuthState('login');
    };
    window.addEventListener(appEvents.authError, onAuthError);
    window.addEventListener(appEvents.legacyAuthError, onAuthError);
    return () => {
      window.removeEventListener(appEvents.authError, onAuthError);
      window.removeEventListener(appEvents.legacyAuthError, onAuthError);
    };
  }, []);

  const completeAuthentication = (nextSession: SessionInfo) => {
    setSession(nextSession);
    setAuthState('authenticated');
  };

  const logout = async () => {
    try {
      await authApi.logout();
    } finally {
      setSession(null);
      setAuthState('login');
    }
  };

  const setAutoRefresh = (auto: boolean) => setAutoRefreshState(auto);
  const triggerRefresh = () => setRefreshKey((previous) => previous + 1);
  const setJobs = (nextJobs: Job[]) => setJobsState(Array.isArray(nextJobs) ? nextJobs : []);
  const setIsSidebarCollapsed = (collapsed: boolean) => {
    setIsSidebarCollapsedState(collapsed);
    writeAppStorage('sidebarCollapsed', String(collapsed));
  };

  return (
    <ServerStoreContext.Provider
      value={{
        authState,
        session,
        completeAuthentication,
        refreshAuthentication,
        logout,
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
