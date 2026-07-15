import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { PalDefenderGM } from './PalDefenderGM';

const mocks = vi.hoisted(() => ({
  authApi: { status: vi.fn(), logout: vi.fn() },
  gmApi: {
    status: vi.fn(),
    players: vi.fn(),
    player: vi.fn(),
    items: vi.fn(),
    inventory: vi.fn(),
    giveItems: vi.fn(),
    sendMessage: vi.fn(),
    broadcast: vi.fn(),
    kick: vi.fn(),
    ban: vi.fn(),
    unban: vi.fn(),
  },
}));

vi.mock('../api/auth', () => ({ authApi: mocks.authApi }));
vi.mock('../api/paldefenderGM', () => ({ palDefenderGMApi: mocks.gmApi }));

const renderPage = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <ServerStoreProvider>
          <PalDefenderGM />
        </ServerStoreProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
};

describe('PalDefender GM workspace', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    mocks.authApi.status.mockResolvedValue({
      initialized: true,
      authenticated: true,
      user: { name: 'operator', role: 'operator', permissions: ['read', 'players:write'] },
    });
    mocks.authApi.logout.mockResolvedValue({ logged_out: true });
    mocks.gmApi.status.mockResolvedValue({ configured: true, available: true, version: { Version: '1.8.1' } });
    mocks.gmApi.players.mockResolvedValue({
      Meta: { PlayerCount: 2, OnlineCount: 1 },
      Players: [
        { Name: 'Builder', IP: '127.0.0.1', UserId: 'steam_1', PlayerUID: 'uid_1', GuildName: 'Guild', GuildUUID: 'guild_1', Status: 'Online', WorldLocation: { x: 1, y: 2, z: 3 }, MapLocation: { x: 4, y: 5, z: 6 } },
        { Name: 'OfflineUser', IP: '', UserId: 'steam_2', PlayerUID: 'uid_2', GuildName: '', GuildUUID: '', Status: 'Offline', WorldLocation: { x: 0, y: 0, z: 0 }, MapLocation: { x: 0, y: 0, z: 0 } },
      ],
    });
    mocks.gmApi.player.mockImplementation(async (identifier: string) => (
      identifier === 'steam_2'
        ? { Name: 'OfflineUser', IP: '', UserId: 'steam_2', PlayerUID: 'uid_2', GuildName: '', GuildUUID: '', Status: 'Offline', WorldLocation: { x: 0, y: 0, z: 0 }, MapLocation: { x: 0, y: 0, z: 0 } }
        : { Name: 'Builder', IP: '127.0.0.1', UserId: 'steam_1', PlayerUID: 'uid_1', GuildName: 'Guild', GuildUUID: 'guild_1', Status: 'Online', WorldLocation: { x: 1, y: 2, z: 3 }, MapLocation: { x: 4, y: 5, z: 6 } }
    ));
    mocks.gmApi.inventory.mockResolvedValue({
      Meta: { Player: 'steam_1', PlayerUID: 'uid_1' },
      Inventory: {
        Items: { Available: true, ContainerID: 'container_1', UsedSlots: 2, MaxSlots: 42, FreeSlots: 40, Slots: { 0: { ItemID: 'Money', Count: 25 }, 1: { ItemID: 'FutureItem', Count: 1 } } },
        KeyItems: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        Weapons: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        Armor: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        Food: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        DropSlot: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
      },
    });
    mocks.gmApi.items.mockResolvedValue({
      items: [
        { id: 'Money', name: '金币', icon: 'money' },
        { id: 'ExplosiveBullet', name: '火箭弹', icon: 'explosivebullet' },
      ],
      returned: 2,
    });
    mocks.gmApi.giveItems.mockResolvedValue({ Granted: { Items: 5 } });
    mocks.gmApi.sendMessage.mockResolvedValue({ Success: true });
    mocks.gmApi.broadcast.mockResolvedValue({ Success: true });
    mocks.gmApi.kick.mockResolvedValue({ Success: true });
    mocks.gmApi.ban.mockResolvedValue({ Success: true });
    mocks.gmApi.unban.mockResolvedValue({ Success: true });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('gives items, refreshes inventory, sends a message, and bans the selected player', async () => {
    renderPage();

    expect(await screen.findByText('Money')).toBeInTheDocument();
    expect(await screen.findByText('金币')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: '金币图标' })).toHaveAttribute('src', '/assets/items/money.webp');
    expect(screen.getByText('FutureItem')).toBeInTheDocument();
    expect(screen.queryByRole('img', { name: 'FutureItem图标' })).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: '确认发放' })).toBeEnabled());
    await waitFor(() => expect(mocks.gmApi.items).toHaveBeenCalledWith('', 5000));
    fireEvent.focus(screen.getByLabelText('物品 ID 1'));
    fireEvent.change(screen.getByLabelText('物品 ID 1'), { target: { value: '火箭' } });
    fireEvent.click(await screen.findByText('火箭弹'));
    fireEvent.change(screen.getByLabelText('数量 1'), { target: { value: '5' } });
    fireEvent.click(screen.getByRole('button', { name: '添加' }));
    fireEvent.change(screen.getByLabelText('物品 ID 2'), { target: { value: 'Money' } });
    fireEvent.change(screen.getByLabelText('数量 2'), { target: { value: '2' } });
    fireEvent.click(screen.getByRole('button', { name: '确认发放' }));

    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('2 种物品'));
    await waitFor(() => expect(mocks.gmApi.giveItems).toHaveBeenCalledWith('steam_1', [
      { ItemID: 'ExplosiveBullet', Count: 5 },
      { ItemID: 'Money', Count: 2 },
    ]));
    expect(await screen.findByText('已发放 5 件物品')).toBeInTheDocument();
    expect(mocks.gmApi.inventory).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole('tab', { name: '消息' }));
    fireEvent.change(screen.getByLabelText('消息内容'), { target: { value: 'Maintenance soon' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => expect(mocks.gmApi.sendMessage).toHaveBeenCalledWith('steam_1', { SendType: 'PlayerLogImportant', Message: 'Maintenance soon' }));

    fireEvent.click(screen.getByRole('tab', { name: '管理' }));
    fireEvent.change(screen.getByLabelText('操作原因'), { target: { value: 'abuse' } });
    fireEvent.click(screen.getByLabelText('同时封禁 IP'));
    fireEvent.click(screen.getByRole('button', { name: '封禁' }));
    await waitFor(() => expect(mocks.gmApi.ban).toHaveBeenCalledWith('steam_1', { Reason: 'abuse', IP: true }));
  });

  it('shows the configuration state without querying players', async () => {
    mocks.gmApi.status.mockResolvedValue({ configured: false, available: false });
    renderPage();

    expect(await screen.findByText('REST Token 未配置')).toBeInTheDocument();
    expect(mocks.gmApi.players).not.toHaveBeenCalled();
  });

  it.each([
    ['not_installed', 'PalDefender 尚未安装'],
    ['not_loaded', 'PalDefender 尚未通过启动日志确认加载'],
    ['rest_disabled', 'PalDefender REST API 未启用'],
    ['server_not_running', '游戏服务或 PalDefender REST 未运行'],
  ] as const)('shows the %s readiness state', async (state, title) => {
    mocks.gmApi.status.mockResolvedValue({
      configured: true,
      available: false,
      installed: state !== 'not_installed',
      load_verified: !['not_installed', 'not_loaded'].includes(state),
      rest_enabled: state !== 'rest_disabled',
      state,
    });
    renderPage();

    expect(await screen.findByText(title)).toBeInTheDocument();
    expect(mocks.gmApi.players).not.toHaveBeenCalled();
  });

  it('keeps GM writes disabled for a read-only session', async () => {
    mocks.authApi.status.mockResolvedValue({
      initialized: true,
      authenticated: true,
      user: { name: 'viewer', role: 'viewer', permissions: ['read'] },
    });
    renderPage();

    expect(await screen.findByRole('button', { name: '确认发放' })).toBeDisabled();
    fireEvent.click(screen.getByRole('tab', { name: '管理' }));
    expect(screen.getByRole('button', { name: '封禁' })).toBeDisabled();
  });

  it('blocks online-only actions for an offline player', async () => {
    renderPage();

    fireEvent.click(await screen.findByRole('button', { name: /OfflineUser/ }));
    await waitFor(() => expect(mocks.gmApi.player).toHaveBeenCalledWith('steam_2'));

    expect(screen.getByRole('button', { name: '确认发放' })).toBeDisabled();
    fireEvent.click(screen.getByRole('tab', { name: '消息' }));
    fireEvent.change(screen.getByLabelText('消息内容'), { target: { value: 'hello' } });
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled();
    fireEvent.click(screen.getByRole('tab', { name: '管理' }));
    expect(screen.getByRole('button', { name: '踢出' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '封禁' })).toBeEnabled();
  });

  it('does not execute a dangerous action when confirmation is rejected', async () => {
    vi.mocked(window.confirm).mockReturnValue(false);
    renderPage();

    fireEvent.click(await screen.findByRole('tab', { name: '管理' }));
    fireEvent.click(screen.getByRole('button', { name: '封禁' }));

    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Builder'));
    expect(mocks.gmApi.ban).not.toHaveBeenCalled();
  });
});
