import React, { Suspense } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { CheckCircle2, Gauge, LoaderCircle, LockKeyhole, RefreshCw, ServerCog, ShieldCheck, User } from 'lucide-react';
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
      <div className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-slate-950 p-6 text-sm font-semibold text-slate-300">
        <div className="absolute -left-24 -top-24 h-80 w-80 rounded-full bg-sky-500/15 blur-3xl" />
        <div className="relative flex items-center gap-3 rounded-2xl border border-white/10 bg-white/[0.055] px-5 py-4 shadow-2xl backdrop-blur-xl">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-sky-500/15 text-sky-300">
            <LoaderCircle className="animate-spin" size={19} />
          </span>
          <span>
            <strong className="block text-sm text-white">正在连接 {appConfig.brand}</strong>
            <span className="mt-0.5 block text-xs font-medium text-slate-500">正在检查服务与登录状态...</span>
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
    <div className="grid min-h-dvh overflow-hidden bg-slate-950 lg:grid-cols-[minmax(360px,0.92fr)_minmax(520px,1.08fr)]">
      <aside className="relative hidden overflow-hidden p-10 text-white lg:flex lg:flex-col lg:justify-between xl:p-14">
        <div className="absolute -left-32 -top-24 h-[30rem] w-[30rem] rounded-full bg-sky-500/14 blur-3xl" />
        <div className="absolute -bottom-44 right-[-8rem] h-[32rem] w-[32rem] rounded-full bg-blue-500/12 blur-3xl" />

        <div className="relative flex items-center gap-3">
          <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-gradient-to-br from-sky-400 to-blue-500 shadow-xl shadow-sky-950/30 ring-1 ring-white/15">
            <ServerCog size={22} />
          </span>
          <span>
            <strong className="block text-lg font-bold tracking-tight">{appConfig.brand}</strong>
            <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Palworld operations</span>
          </span>
        </div>

        <div className="relative max-w-xl py-12">
          <span className="inline-flex items-center gap-2 rounded-full border border-sky-300/15 bg-sky-400/10 px-3 py-1.5 text-xs font-semibold text-sky-200">
            <Gauge size={14} />
            清晰掌握每一次运行状态
          </span>
          <h1 className="mt-6 text-4xl font-bold leading-[1.12] tracking-[-0.045em] xl:text-5xl">
            把复杂的服务器运维，<br />
            <span className="text-sky-300">收进一个安静的控制台。</span>
          </h1>
          <p className="mt-6 max-w-lg text-base font-medium leading-8 text-slate-400">
            从开服、监控到存档、Mod 与安全维护，重要信息保持清晰，危险操作始终可控。
          </p>

          <div className="mt-9 grid gap-3 text-sm font-semibold text-slate-300 sm:grid-cols-2">
            {['同源会话认证', '关键操作确认', '实时状态汇总', '多端自适应界面'].map((item) => (
              <span key={item} className="flex items-center gap-2.5">
                <CheckCircle2 size={16} className="text-sky-300" />
                {item}
              </span>
            ))}
          </div>
        </div>

        <p className="relative text-xs font-medium text-slate-600">Palworld server management, without the clutter.</p>
      </aside>

      <main className="relative flex items-center justify-center bg-slate-50 p-4 sm:p-8 lg:rounded-l-[32px] lg:p-12">
        <div className="w-full max-w-md">
          <div className="mb-7 flex items-center gap-3 lg:hidden">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-slate-900 text-sky-300">
              <ServerCog size={20} />
            </span>
            <div>
              <strong className="block text-base font-bold text-slate-900">{appConfig.brand}</strong>
              <span className="text-xs font-medium text-slate-500">Palworld 服务器控制台</span>
            </div>
          </div>

          <form
            className="rounded-2xl border border-slate-200 bg-white p-6 shadow-[0_26px_70px_-38px_rgba(8,17,31,0.48)] sm:p-8"
            onSubmit={(event) => {
              event.preventDefault();
              if (username.trim() && password) void submit();
            }}
          >
            <div className="flex items-start justify-between gap-4 border-b border-slate-100 pb-6">
              <div>
                <span className="text-xs font-bold uppercase tracking-[0.14em] text-sky-700">
                  {registering ? 'First run' : 'Welcome back'}
                </span>
                <h1 className="mt-1.5 text-2xl font-bold tracking-[-0.03em] text-slate-900">
                  {registering ? '创建管理员账号' : '管理员登录'}
                </h1>
                <p className="mt-2 text-sm font-medium leading-6 text-slate-500">
                  {registering ? '为当前面板创建首个本地管理员。' : '登录后继续管理服务器。'}
                </p>
              </div>
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-sky-50 text-sky-700 ring-1 ring-sky-100">
                <ShieldCheck size={20} />
              </span>
            </div>

            {error && (
              <p role="alert" className="mt-5 rounded-xl border border-rose-100 bg-rose-50 px-4 py-3 text-sm font-semibold leading-5 text-rose-700">
                {error}
              </p>
            )}

            <label className="mt-6 flex flex-col gap-2 text-sm font-semibold text-slate-700">
              用户名
              <span className="relative">
                <User className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" size={16} />
                <input
                  type="text"
                  aria-label="用户名"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  autoComplete="username"
                  autoFocus
                  minLength={3}
                  maxLength={32}
                  required
                  className="h-11 w-full rounded-xl border border-slate-200 bg-slate-50/60 pl-10 pr-3 text-sm font-semibold text-slate-900 shadow-inner shadow-slate-100/30 focus:border-sky-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-sky-500/10"
                />
              </span>
            </label>

            <label className="mt-4 flex flex-col gap-2 text-sm font-semibold text-slate-700">
              密码
              <span className="relative">
                <LockKeyhole className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" size={16} />
                <input
                  type="password"
                  aria-label="密码"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete={registering ? 'new-password' : 'current-password'}
                  minLength={registering ? 12 : undefined}
                  maxLength={128}
                  required
                  className="h-11 w-full rounded-xl border border-slate-200 bg-slate-50/60 pl-10 pr-3 text-sm font-semibold text-slate-900 shadow-inner shadow-slate-100/30 focus:border-sky-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-sky-500/10"
                />
              </span>
              {registering && <span className="text-xs font-medium text-slate-400">至少 12 位，建议使用独立密码。</span>}
            </label>

            {registering && (
              <label className="mt-4 flex flex-col gap-2 text-sm font-semibold text-slate-700">
                确认密码
                <input
                  type="password"
                  aria-label="确认密码"
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                  autoComplete="new-password"
                  minLength={12}
                  maxLength={128}
                  required
                  className="h-11 rounded-xl border border-slate-200 bg-slate-50/60 px-3 text-sm font-semibold text-slate-900 shadow-inner shadow-slate-100/30 focus:border-sky-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-sky-500/10"
                />
              </label>
            )}

            <button
              type="submit"
              disabled={busy}
              className="mt-6 flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-slate-900 px-4 text-sm font-bold text-white shadow-lg shadow-slate-900/15 transition-colors hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {busy && <LoaderCircle className="animate-spin text-sky-300" size={16} />}
              {registering ? '完成注册' : '登录'}
            </button>

            <p className="mt-4 flex items-center justify-center gap-1.5 text-center text-xs font-medium text-slate-400">
              <ShieldCheck size={13} />
              凭据仅通过当前面板的安全会话提交
            </p>
          </form>
        </div>
      </main>
    </div>
  );
};

const UnavailableGate: React.FC<{ onRetry: () => void }> = ({ onRetry }) => (
  <div className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-slate-950 p-5">
    <div className="absolute -top-32 left-1/2 h-96 w-96 -translate-x-1/2 rounded-full bg-rose-500/10 blur-3xl" />
    <div className="animate-scale-up relative w-full max-w-md rounded-2xl border border-white/10 bg-white/[0.065] p-7 text-center shadow-2xl backdrop-blur-xl sm:p-8">
      <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-rose-500/12 text-rose-300 ring-1 ring-rose-300/10">
        <ServerCog size={22} />
      </span>
      <p className="mt-5 text-xs font-bold uppercase tracking-[0.16em] text-slate-500">{appConfig.brand}</p>
      <h1 className="mt-2 text-xl font-bold text-white">无法连接面板服务</h1>
      <p className="mt-3 text-sm font-medium leading-6 text-slate-400">请确认后端服务正在运行，然后重新尝试连接。</p>
      <button
        type="button"
        onClick={onRetry}
        className="mx-auto mt-6 inline-flex h-10 items-center gap-2 rounded-xl bg-white px-4 text-sm font-bold text-slate-900 transition-colors hover:bg-slate-100"
      >
        <RefreshCw size={15} />
        重新连接
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
