import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { AppContent } from './App';
import { ServerStoreProvider } from './store/useServerStore';

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
  it('renders a configured route directly', () => {
    renderRoute('/settings');
    expect(screen.getAllByText('服务器设置').length).toBeGreaterThan(0);
  });

  it('redirects unknown routes to the dashboard', async () => {
    renderRoute('/does-not-exist');
    expect(await screen.findByText('系统总览')).toBeInTheDocument();
  });
});
