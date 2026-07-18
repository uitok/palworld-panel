import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { CommunityServers } from './CommunityServers';

const mocks = vi.hoisted(() => ({
  list: vi.fn(), refresh: vi.fn(), sourceStatus: vi.fn(),
}));

vi.mock('../api/communityServers', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api/communityServers')>();
  return { ...original, communityServersApi: mocks };
});

const result = {
  servers: [{
    id: 'cn-1', name: '中文生存服', address: '203.0.113.8', port: 8211, connect: '203.0.113.8:8211',
    players: 18, max_players: 32, password: false, country: 'CN', version: '0.6.4', description: '欢迎新玩家', status: 'online',
  }],
  total: 1, source_total: 1, page: 1, page_size: 30, source: 'battlemetrics',
  fetched_at: '2026-07-18T08:00:00Z', stale: true, cache_age_seconds: 90,
};

describe('CommunityServers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.list.mockResolvedValue(result);
    mocks.refresh.mockResolvedValue({ ...result, stale: false, cache_age_seconds: 0 });
    mocks.sourceStatus.mockResolvedValue({
      source: 'battlemetrics', base_url: 'https://api.battlemetrics.com', proxy_configured: true,
      reachable: false, cache_available: true, cache_fresh: false, cached_queries: 1, rate_limit_per_minute: 30,
    });
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });

  it('defaults to China, shows stale cache, filters, details, and copies the address', async () => {
    render(<CommunityServers />);

    expect(await screen.findByText('中文生存服')).toBeInTheDocument();
    expect(mocks.list).toHaveBeenCalledWith(expect.objectContaining({ region: 'cn', status: 'online', page: 1 }));
    expect(screen.getByText(/正在显示 90 秒前的缓存/)).toBeInTheDocument();
    expect(screen.getByText('已配置国内代理')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('范围'), { target: { value: 'global' } });
    fireEvent.change(screen.getByLabelText('服务器名称'), { target: { value: 'friends' } });
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'false' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({ region: 'global', search: 'friends', password: false, page: 1 })));

    fireEvent.click(screen.getByRole('button', { name: /中文生存服/ }));
    expect(await screen.findByText('欢迎新玩家')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /复制 203.0.113.8:8211/ }));
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith('203.0.113.8:8211'));
    expect(await screen.findByText('已复制 203.0.113.8:8211')).toBeInTheDocument();
  });

  it('forces a refresh without changing the active query', async () => {
    render(<CommunityServers />);
    await screen.findByText('中文生存服');
    fireEvent.click(screen.getByRole('button', { name: '刷新数据源' }));
    await waitFor(() => expect(mocks.refresh).toHaveBeenCalledWith(expect.objectContaining({ region: 'cn', status: 'online' })));
  });
});

