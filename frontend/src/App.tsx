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
import { useI18n } from './i18n';
import { LanguageSwitcher } from './components/ui/LanguageSwitcher';

export const AppContent: React.FC = () => {
  const { t } = useI18n();
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
            <strong className="block text-sm text-slate-900">{t('auth.connectingTitle', { brand: appConfig.brand })}</strong>
            <span className="mt-1 block text-xs text-slate-500">{t('auth.connectingDescription')}</span>
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
                {t('auth.loadingPage')}
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
  const { t } = useI18n();
  const [username, setUsername] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [confirmation, setConfirmation] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');
  const registering = mode === 'register';

  const submit = async () => {
    if (registering && password !== confirmation) {
      setError(t('auth.passwordMismatch'));
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
        <LanguageSwitcher compact className="mb-4 w-full justify-end" />
        <span className="pp-login__logo"><img src="/brand/palpanel-mark.svg" alt="" width="44" height="44" /></span>
        <div className="pp-topbar__eyebrow">{registering ? t('auth.firstRun') : t('auth.welcomeBack')}</div>
        <h1 className="pp-login__title">{registering ? t('auth.registerTitle') : t('auth.loginTitle')}</h1>
        <p className="pp-login__sub">{registering ? t('auth.registerDescription') : t('auth.loginDescription')}</p>

        {error && <div role="alert" className="pp-note pp-note--danger mb-4">{error}</div>}

        <div className="pp-login__form">
          <label className="pp-field">
            <span className="pp-field__label">{t('auth.username')}</span>
            <span className="relative">
              <User className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
              <input
                className="pp-input w-full pl-9"
                type="text"
                aria-label={t('auth.username')}
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
            <span className="pp-field__label">{t('auth.password')}</span>
            <span className="relative">
              <LockKeyhole className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
              <input
                className="pp-input w-full pl-9"
                type="password"
                aria-label={t('auth.password')}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete={registering ? 'new-password' : 'current-password'}
                minLength={registering ? 12 : undefined}
                maxLength={128}
                required
              />
            </span>
            {registering && <span className="pp-field__help">{t('auth.passwordHelp')}</span>}
          </label>

          {registering && (
            <label className="pp-field">
              <span className="pp-field__label">{t('auth.confirmPassword')}</span>
              <input
                className="pp-input"
                type="password"
                aria-label={t('auth.confirmPassword')}
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
            {registering ? t('auth.register') : t('auth.login')}
          </button>
        </div>

        <div className="pp-login__foot">
          <span className="pp-row"><ShieldCheck size={13} />{t('auth.localSession')}</span>
          <span>{appConfig.brand}</span>
        </div>
      </form>
    </div>
  );
};

const UnavailableGate: React.FC<{ onRetry: () => void }> = ({ onRetry }) => {
  const { t } = useI18n();
  return <div className="pp-login">
    <section className="pp-login-card text-center">
      <LanguageSwitcher compact className="mb-4 w-full justify-end text-left" />
      <span className="pp-login__logo mx-auto"><img src="/brand/palpanel-mark.svg" alt="" width="44" height="44" /></span>
      <div className="pp-topbar__eyebrow">{appConfig.brand}</div>
      <h1 className="pp-login__title mt-1">{t('auth.unavailableTitle')}</h1>
      <p className="pp-login__sub">{t('auth.unavailableDescription')}</p>
      <button type="button" onClick={onRetry} className="pp-btn pp-btn--primary mx-auto">
        <RefreshCw size={15} />{t('auth.retry')}
      </button>
    </section>
  </div>;
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
