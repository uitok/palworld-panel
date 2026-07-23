import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Players } from './Players';

const mocks = vi.hoisted(() => ({
  getPlayersList: vi.fn(),
  kickPlayer: vi.fn(),
  banPlayer: vi.fn(),
  rebuild: vi.fn(),
}));

vi.mock('../api/players', () => ({
  playersApi: {
    getPlayersList: mocks.getPlayersList,
    kickPlayer: mocks.kickPlayer,
    banPlayer: mocks.banPlayer,
  },
}));
vi.mock('../api/saveIndex', () => ({ saveIndexApi: { rebuild: mocks.rebuild } }));
vi.mock('../store/useServerStore', () => ({ useServerStore: () => ({ refreshKey: 0 }) }));

const renderPage = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter initialEntries={['/players']}>
      <QueryClientProvider client={client}>
        <Players />
      </QueryClientProvider>
    </MemoryRouter>,
  );
};

describe('Players archive', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.rebuild.mockResolvedValue({ status: 'waiting' });
    mocks.getPlayersList.mockResolvedValue({
      items: [{
        id: 'steam_1',
        steam_id: 'steam_1',
        player_uid: 'uid_1',
        nickname: 'Archivist',
        level: 18,
        guild_name: '',
        is_online: false,
        online_source: 'none',
        online_stale: true,
        last_online_time: '2026-07-14T12:00:00Z',
        x: 1,
        y: 2,
        z: 3,
      }],
      status: {
        enabled: true,
        state: 'ready',
        stale: false,
        source_path: '',
        updated_at: '',
        fingerprint: '',
        parser: 'sav-cli',
        parser_available: true,
        counts: { players: 1, guilds: 0, bases: 0, pals: 0, map_objects: 0 },
        warnings: [],
      },
      summary: { total: 1, returned: 1, limit: 50, offset: 0, truncated: false },
    });
  });

  afterEach(() => cleanup());

  it('shows stale REST state without marking the player online', async () => {
    renderPage();

    expect(await screen.findByText(/官方 REST 在线状态暂不可用/)).toBeInTheDocument();
    expect(screen.getAllByText('离线').length).toBeGreaterThan(0);
  });

  it('shows stale REST state when the current page has no players', async () => {
    mocks.getPlayersList.mockResolvedValue({
      items: [],
      status: {
        enabled: true,
        state: 'ready',
        stale: true,
        source_path: '',
        updated_at: '',
        fingerprint: '',
        parser: 'sav-cli',
        parser_available: true,
        counts: { players: 0, guilds: 0, bases: 0, pals: 0, map_objects: 0 },
        warnings: ['online player REST data is stale or unavailable'],
      },
      summary: { total: 0, returned: 0, limit: 50, offset: 0, truncated: false },
    });

    renderPage();

    expect(await screen.findByText(/官方 REST 在线状态暂不可用/)).toBeInTheDocument();
  });
});
