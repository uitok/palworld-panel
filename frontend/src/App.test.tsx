import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppContent } from './App';
import { ServerStoreProvider } from './store/ServerStoreProvider';
import { storageKeys } from './config/defaults';

vi.mock('./pages/Settings', () => ({ Settings: () => <h1>设置路由内容</h1> }));
vi.mock('./pages/Dashboard', () => ({ Dashboard: () => <h1>总览路由内容</h1> }));

const renderRoute = (path: string) => {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <ServerStoreProvider>
        <AppContent />
      </ServerStoreProvider>
    </MemoryRouter>,
  );
};

describe('app routing', () => {
  beforeEach(() => {
    localStorage.setItem(storageKeys.token, 'test-token');
  });

  it('renders a configured route directly', async () => {
    renderRoute('/settings');
    expect(await screen.findByText('设置路由内容')).toBeInTheDocument();
  });

  it('redirects unknown routes to the dashboard', async () => {
    renderRoute('/does-not-exist');
    expect(await screen.findByText('总览路由内容')).toBeInTheDocument();
  });
});
