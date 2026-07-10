import { createContext } from 'react';
import type { Job, ServerMetrics, ServerStatus, SessionInfo } from '../types';

export interface ServerStoreContextType {
  panelToken: string;
  setPanelToken: (token: string) => void;
  authError: boolean;
  clearAuthError: () => void;
  session: SessionInfo | null;
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
