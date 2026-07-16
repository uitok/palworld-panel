import React, { useEffect, useState } from 'react';
import { Dna, LoaderCircle, LogOut, ShieldCheck } from 'lucide-react';
import { breedSessionApi, type BreedSessionPrincipal } from '../api/breeding';
import { getErrorMessage } from '../api/client';

const BreedingLab = React.lazy(() => import('./BreedingLab').then((module) => ({ default: module.BreedingLab })));

export const breedSessionStorageKey = 'palpanel_breed_principal';

const exchangeRequests = new Map<string, Promise<BreedSessionPrincipal>>();

const exchangeOnce = (ticket: string) => {
  const current = exchangeRequests.get(ticket);
  if (current) return current;
  const request = breedSessionApi.exchange(ticket);
  exchangeRequests.set(ticket, request);
  return request;
};

const cachedPrincipal = (): BreedSessionPrincipal | null => {
  try {
    const raw = sessionStorage.getItem(breedSessionStorageKey);
    return raw ? JSON.parse(raw) as BreedSessionPrincipal : null;
  } catch {
    return null;
  }
};

export const BreedPortal: React.FC = () => {
  const params = new URLSearchParams(window.location.search);
  const ticket = params.get('ticket') || '';
  const quickQuery = params.get('quick') || '';
  const [principal, setPrincipal] = useState<BreedSessionPrincipal | null>(cachedPrincipal);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    if (ticket) {
      const cleanURL = new URL(window.location.href);
      cleanURL.searchParams.delete('ticket');
      window.history.replaceState({}, '', `${cleanURL.pathname}${cleanURL.search}${cleanURL.hash}`);
    }
    const request = ticket ? exchangeOnce(ticket) : breedSessionApi.me();
    void request
      .then((identity) => {
        if (!active) return;
        const merged = { ...cachedPrincipal(), ...identity };
        sessionStorage.setItem(breedSessionStorageKey, JSON.stringify(merged));
        setPrincipal(merged);
        setState('ready');
      })
      .catch((reason) => {
        if (!active) return;
        sessionStorage.removeItem(breedSessionStorageKey);
        setError(getErrorMessage(reason));
        setState('error');
      });
    return () => { active = false; };
  }, [ticket]);

  const updateBalance = (balance: number) => {
    setPrincipal((current) => {
      if (!current) return current;
      const updated = { ...current, balance };
      sessionStorage.setItem(breedSessionStorageKey, JSON.stringify(updated));
      return updated;
    });
  };

  if (state !== 'ready' || !principal) {
    return (
      <main className="breed-portal-gate">
        <section className="pp-card">
          <span className="breed-portal-mark"><Dna size={24} /></span>
          <h1>{state === 'error' ? '配种链接不可用' : '正在验证 QQ 配种会话'}</h1>
          <p>{state === 'error' ? error || '链接已过期或已经使用，请回到 QQ 群重新发送 /pz。' : '正在交换一次性票据并加载你的帕鲁数据。'}</p>
          {state === 'loading' ? <LoaderCircle className="animate-spin" size={22} /> : <a className="pp-button accent" href="/">返回面板登录</a>}
        </section>
      </main>
    );
  }

  return (
    <div className="breed-portal-shell">
      <header className="breed-portal-header">
        <a href="/" className="breed-portal-brand"><span><Dna size={18} /></span><strong>PalPanel 配种实验室</strong></a>
        <div className="breed-portal-identity">
          <ShieldCheck size={15} />
          <span>{principal.nickname || principal.player_uid}</span>
          <b>{principal.balance ?? '-'} 积分</b>
          <a href="/" aria-label="退出配种会话" onClick={() => sessionStorage.removeItem(breedSessionStorageKey)}><LogOut size={15} /></a>
        </div>
      </header>
      <main className="breed-portal-content">
        <React.Suspense fallback={<div className="breed-portal-loading"><LoaderCircle className="animate-spin" size={22} />正在加载配种实验室...</div>}>
          <BreedingLab access="session" principal={principal} quickQuery={quickQuery} onBalanceChange={updateBalance} />
        </React.Suspense>
      </main>
    </div>
  );
};
