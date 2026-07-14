import { createContext } from 'react';
import type { Job, ServerMetrics, ServerStatus, SessionInfo } from '../types';

export interface ServerStoreContextType {
  authState: 'loading' | 'register' | 'login' | 'authenticated' | 'unavailable';
  session: SessionInfo | null;
  completeAuthentication: (session: SessionInfo) => void;
  refreshAuthentication: () => Promise<void>;
  logout: () => Promise<void>;
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

export const ServerStoreContext = createContext<ServerStoreContextType | undefined>(undefined);
