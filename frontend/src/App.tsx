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
import { BreedPortal, breedSessionStorageKey } from './pages/BreedPortal';

export const AppContent: React.FC = () => {
  const location = useLocation();
  const { authState, completeAuthentication, refreshAuthentication } = useServerStore();
  const breedTicket = new URLSearchParams(location.search).has('ticket');
  const hasBreedSession = typeof window !== 'undefined' && Boolean(sessionStorage.getItem(breedSessionStorageKey));

  if (location.pathname === '/breeding' && (breedTicket || hasBreedSession)) {
    return <QueryProvider><BreedPortal /></QueryProvider>;
  }

  if (authState === 'loading') {
    return (
      <div className="pp-login">
        <div className="pp-login-card pp-row">
          <span className="pp-login__logo mb-0"><LoaderCircle className="animate-spin" size={20} /></span>
          <span>
            <strong className="block text-sm text-slate-900">正在连接 {appConfig.brand}</strong>
            <span className="mt-1 block text-xs text-slate-500">正在检查服务与登录状态...</span>
          </span>
        </div>
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
              <div className="flex h-full items-center justify-center p-12 text-sm font-semibold text-slate-500">
                <LoaderCircle className="mr-2 animate-spin text-sky-500" size={17} />
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
    <div className="pp-login">
      <form
        className="pp-login-card"
        onSubmit={(event) => {
          event.preventDefault();
          if (username.trim() && password) void submit();
        }}
      >
        <span className="pp-login__logo"><img src="/brand/palpanel-mark.svg" alt="" width="44" height="44" /></span>
        <div className="pp-topbar__eyebrow">{registering ? 'FIRST RUN' : 'WELCOME BACK'}</div>
        <h1 className="pp-login__title">{registering ? '创建管理员账号' : '管理员登录'}</h1>
        <p className="pp-login__sub">{registering ? '为当前面板创建首个本地管理员。' : '登录后继续管理服务器。'}</p>

        {error && <div role="alert" className="pp-note pp-note--danger mb-4">{error}</div>}

        <div className="pp-login__form">
          <label className="pp-field">
            <span className="pp-field__label">用户名</span>
            <span className="relative">
              <User className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
              <input
                className="pp-input w-full pl-9"
                type="text"
                aria-label="用户名"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                autoComplete="username"
                autoFocus
                minLength={3}
                maxLength={32}
                required
              />
            </span>
          </label>

          <label className="pp-field">
            <span className="pp-field__label">密码</span>
            <span className="relative">
              <LockKeyhole className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
              <input
                className="pp-input w-full pl-9"
                type="password"
                aria-label="密码"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete={registering ? 'new-password' : 'current-password'}
                minLength={registering ? 12 : undefined}
                maxLength={128}
                required
              />
            </span>
            {registering && <span className="pp-field__help">至少 12 位，建议使用独立密码。</span>}
          </label>

          {registering && (
            <label className="pp-field">
              <span className="pp-field__label">确认密码</span>
              <input
                className="pp-input"
                type="password"
                aria-label="确认密码"
                value={confirmation}
                onChange={(event) => setConfirmation(event.target.value)}
                autoComplete="new-password"
                minLength={12}
                maxLength={128}
                required
              />
            </label>
          )}

          <button type="submit" disabled={busy} className="pp-btn pp-btn--primary pp-btn--wide mt-2">
            {busy && <LoaderCircle className="animate-spin" size={15} />}
            {registering ? '完成注册' : '登录'}
          </button>
        </div>

        <div className="pp-login__foot">
          <span className="pp-row"><ShieldCheck size={13} />本地安全会话</span>
          <span>{appConfig.brand}</span>
        </div>
      </form>
    </div>
  );
};

const UnavailableGate: React.FC<{ onRetry: () => void }> = ({ onRetry }) => (
  <div className="pp-login">
    <section className="pp-login-card text-center">
      <span className="pp-login__logo mx-auto"><img src="/brand/palpanel-mark.svg" alt="" width="44" height="44" /></span>
      <div className="pp-topbar__eyebrow">{appConfig.brand}</div>
      <h1 className="pp-login__title mt-1">无法连接面板服务</h1>
      <p className="pp-login__sub">请确认后端服务正在运行，然后重新尝试连接。</p>
      <button type="button" onClick={onRetry} className="pp-btn pp-btn--primary mx-auto">
        <RefreshCw size={15} />重新连接
      </button>
    </section>
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
