import React from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { ServerStoreProvider } from './store/ServerStoreProvider';
import { useServerStore } from './store/useServerStore';
import { readBackendUrl, writeBackendUrl } from './api/client';
import { AppLayout } from './components/layout/AppLayout';
import { ErrorBoundary } from './components/layout/ErrorBoundary';
import { appRoutes } from './routes';

export const AppContent: React.FC = () => {
  const location = useLocation();
  const { panelToken, setPanelToken, authError, clearAuthError } = useServerStore();

  if (!panelToken || authError) {
    return (
      <TokenGate
        authError={authError}
        onSubmit={(token, backendUrl) => {
          writeBackendUrl(backendUrl);
          setPanelToken(token);
          clearAuthError();
        }}
      />
    );
  }

  return (
    <AppLayout>
      <ErrorBoundary key={location.pathname}>
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          {appRoutes.map((route) => (
            <Route key={route.id} path={route.path} element={route.element} />
          ))}
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </ErrorBoundary>
    </AppLayout>
  );
};

const TokenGate: React.FC<{ authError: boolean; onSubmit: (token: string, backendUrl: string) => void }> = ({ authError, onSubmit }) => {
  const [token, setToken] = React.useState('');
  const [backendUrl, setBackendUrl] = React.useState(() => readBackendUrl());
  return (
    <div className="flex min-h-dvh items-center justify-center bg-slate-100 p-4">
      <form
        className="w-full max-w-md rounded-3xl border border-slate-100 bg-white p-6 shadow-[0_24px_70px_rgba(15,23,42,0.08)]"
        onSubmit={(event) => {
          event.preventDefault();
          if (token.trim()) onSubmit(token.trim(), backendUrl.trim());
        }}
      >
        <p className="text-[11px] font-bold uppercase text-sky-500">PalSphere Admin</p>
        <h1 className="mt-2 text-xl font-bold text-slate-900">输入面板访问 Token</h1>
        <p className="mt-2 text-xs font-medium leading-6 text-slate-500">
          {authError ? '当前 token 已失效或权限不足，请重新输入后继续。' : '后端已启用鉴权，请输入 PANEL_TOKEN。'}
        </p>
        <label className="mt-5 flex flex-col gap-1.5 text-xs font-semibold text-slate-500">
          后端地址
          <input
            type="text"
            value={backendUrl}
            onChange={(event) => setBackendUrl(event.target.value)}
            placeholder="http://127.0.0.1:64217"
            className="rounded-xl border border-slate-200 p-3 font-mono text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
          />
        </label>
        <input
          type="password"
          value={token}
          onChange={(event) => setToken(event.target.value)}
          placeholder="PANEL_TOKEN"
          className="mt-3 w-full rounded-xl border border-slate-200 p-3 font-mono text-xs font-semibold text-slate-700 focus:border-sky-500 focus:outline-none"
        />
        <button type="submit" className="mt-4 w-full rounded-xl bg-sky-500 px-4 py-3 text-xs font-bold text-white hover:bg-sky-600">
          进入管理面板
        </button>
      </form>
    </div>
  );
};

function App() {
  return (
    <BrowserRouter>
      <ServerStoreProvider>
        <AppContent />
      </ServerStoreProvider>
    </BrowserRouter>
  );
}

export default App;
