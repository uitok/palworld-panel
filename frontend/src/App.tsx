import React, { Suspense } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { LoaderCircle, LockKeyhole, RefreshCw, ShieldCheck, User } from 'lucide-react';
import { ServerStoreProvider } from './store/ServerStoreProvider';
import { useServerStore } from './store/useServerStore';
import { AppLayout } from './components/layout/AppLayout';
import { ErrorBoundary } from './components/layout/ErrorBoundary';
import { QueryProvider } from './queryClient';
import { appRoutes } from './routes';
import { appConfig } from './config/defaults';
import { authApi } from './api/auth';
import { getErrorMessage } from './api/client';

export const AppContent: React.FC = () => {
  const location = useLocation();
  const { authState, completeAuthentication, refreshAuthentication } = useServerStore();

  if (authState === 'loading') {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-slate-100 text-xs font-semibold text-slate-500">
        <LoaderCircle className="mr-2 animate-spin text-sky-500" size={17} />
        正在连接面板...
      </div>
    );
  }

  if (authState === 'unavailable') {
    return <UnavailableGate onRetry={() => void refreshAuthentication()} />;
  }

  if (authState === 'register' || authState === 'login') {
    return <AccountGate mode={authState} onAuthenticated={completeAuthentication} />;
  }

  return (
    <QueryProvider>
      <AppLayout>
        <ErrorBoundary key={location.pathname}>
          <Suspense
            fallback={
              <div className="flex h-full items-center justify-center p-12 text-xs font-semibold text-slate-400">
                正在加载页面...
              </div>
            }
          >
            <Routes>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              {appRoutes.map((route) => (
                <Route key={route.id} path={route.path} element={route.element} />
              ))}
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </AppLayout>
    </QueryProvider>
  );
};

const AccountGate: React.FC<{
  mode: 'register' | 'login';
  onAuthenticated: (session: Awaited<ReturnType<typeof authApi.login>>) => void;
}> = ({ mode, onAuthenticated }) => {
  const [username, setUsername] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [confirmation, setConfirmation] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');
  const registering = mode === 'register';

  const submit = async () => {
    if (registering && password !== confirmation) {
      setError('两次输入的密码不一致');
      return;
    }
    setBusy(true);
    setError('');
    try {
      const session = registering
        ? await authApi.register(username.trim(), password)
        : await authApi.login(username.trim(), password);
      onAuthenticated(session);
    } catch (submitError) {
      setError(getErrorMessage(submitError));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-dvh items-center justify-center bg-slate-100 p-4">
      <form
        className="w-full max-w-sm rounded-lg border border-slate-200 bg-white p-6 shadow-[0_20px_55px_rgba(15,23,42,0.08)]"
        onSubmit={(event) => {
          event.preventDefault();
          if (username.trim() && password) void submit();
        }}
      >
        <div className="flex items-center gap-3 border-b border-slate-100 pb-5">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-500 text-white">
            <ShieldCheck size={21} />
          </div>
          <div>
            <p className="text-[11px] font-bold text-sky-600">{appConfig.brand}</p>
            <h1 className="text-lg font-bold text-slate-900">{registering ? '创建管理员账号' : '管理员登录'}</h1>
          </div>
        </div>

        {error && <p role="alert" className="mt-4 rounded-lg bg-rose-50 px-3 py-2.5 text-xs font-semibold text-rose-700">{error}</p>}

        <label className="mt-5 flex flex-col gap-1.5 text-xs font-semibold text-slate-600">
          用户名
          <span className="relative">
            <User className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
            <input
              type="text"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              autoFocus
              minLength={3}
              maxLength={32}
              required
              className="w-full rounded-lg border border-slate-200 py-2.5 pl-9 pr-3 text-sm font-semibold text-slate-800 focus:border-sky-500 focus:outline-none"
            />
          </span>
        </label>
        <label className="mt-4 flex flex-col gap-1.5 text-xs font-semibold text-slate-600">
          密码
          <span className="relative">
            <LockKeyhole className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete={registering ? 'new-password' : 'current-password'}
              minLength={registering ? 12 : undefined}
              maxLength={128}
              required
              className="w-full rounded-lg border border-slate-200 py-2.5 pl-9 pr-3 text-sm font-semibold text-slate-800 focus:border-sky-500 focus:outline-none"
            />
          </span>
        </label>
        {registering && (
          <label className="mt-4 flex flex-col gap-1.5 text-xs font-semibold text-slate-600">
            确认密码
            <input
              type="password"
              value={confirmation}
              onChange={(event) => setConfirmation(event.target.value)}
              autoComplete="new-password"
              minLength={12}
              maxLength={128}
              required
              className="rounded-lg border border-slate-200 px-3 py-2.5 text-sm font-semibold text-slate-800 focus:border-sky-500 focus:outline-none"
            />
          </label>
        )}
        <button type="submit" disabled={busy} className="mt-5 flex w-full items-center justify-center gap-2 rounded-lg bg-sky-500 px-4 py-3 text-xs font-bold text-white hover:bg-sky-600 disabled:opacity-50">
          {busy && <LoaderCircle className="animate-spin" size={15} />}
          {registering ? '完成注册' : '登录'}
        </button>
      </form>
    </div>
  );
};

const UnavailableGate: React.FC<{ onRetry: () => void }> = ({ onRetry }) => (
  <div className="flex min-h-dvh items-center justify-center bg-slate-100 p-4">
    <div className="w-full max-w-sm rounded-lg border border-slate-200 bg-white p-6 text-center shadow-sm">
      <h1 className="text-lg font-bold text-slate-900">无法连接面板服务</h1>
      <button type="button" onClick={onRetry} className="mt-5 inline-flex items-center gap-2 rounded-lg bg-slate-900 px-4 py-2.5 text-xs font-bold text-white">
        <RefreshCw size={14} />重新连接
      </button>
    </div>
  </div>
);

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
