import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LiveMap } from './LiveMap';

const mocks = vi.hoisted(() => ({
  getMapEntities: vi.fn(),
  rebuild: vi.fn(),
}));

vi.mock('../api/saveIndex', () => ({
  saveIndexApi: mocks,
}));

const renderPage = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={['/map']}>
      <QueryClientProvider client={client}>
        <LiveMap />
      </QueryClientProvider>
    </MemoryRouter>,
  );
};

describe('LiveMap', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.rebuild.mockResolvedValue({ status: 'waiting' });
    mocks.getMapEntities.mockResolvedValue({
      entities: [
        { type: 'player', id: 'player-1', label: 'Builder', x: 100, y: 200, z: 30, is_online: true, live: true, source: 'live', guild_name: 'Builders' },
        { type: 'base', id: 'base-1', label: '主基地', x: 150, y: 250, z: 20, source: 'save', pals_count: 12 },
        { type: 'pal', id: 'pal-1', label: '捣蛋猫', x: 300, y: 400, z: 0, source: 'save', level: 8 },
      ],
      status: { enabled: true, available: true, stale: false, building: false, parser_available: true, counts: {}, warnings: [] },
      summary: { total: 3, returned: 3, limit: 100, offset: 0, truncated: false },
      live: { available: true, source: 'paldefender', online_players: 1, refreshed_at: '2026-07-16T00:00:00Z' },
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('shows the live source, online players, and opt-in Pal markers', async () => {
    renderPage();

    expect(await screen.findByText('PalDefender')).toBeInTheDocument();
    expect(screen.getByText(/底图为坐标示意/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Builder 100, 200, 30/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /捣蛋猫 300, 400, 0/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '帕鲁' }));
    expect(screen.getByRole('button', { name: /捣蛋猫 300, 400, 0/ })).toBeInTheDocument();
  });
});
