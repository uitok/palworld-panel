import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { PalDefenderGM } from './PalDefenderGM';

const mocks = vi.hoisted(() => ({
  authApi: { status: vi.fn(), logout: vi.fn() },
  playersApi: { getPlayersList: vi.fn(), getPlayer: vi.fn(), getInventory: vi.fn() },
  palsApi: { getPalsList: vi.fn() },
  gmApi: {
    status: vi.fn(), players: vi.fn(), player: vi.fn(), items: vi.fn(), inventory: vi.fn(),
    removeItems: vi.fn(), teleport: vi.fn(), localTechnologyCatalog: vi.fn(), technologyCatalog: vi.fn(), palCatalog: vi.fn(), passiveCatalog: vi.fn(),
    progression: vi.fn(), giveProgression: vi.fn(), techs: vi.fn(), learnTech: vi.fn(), forgetTech: vi.fn(),
    pals: vi.fn(), givePals: vi.fn(), giveCustomPal: vi.fn(), releasePal: vi.fn(), templates: vi.fn(), template: vi.fn(), putTemplate: vi.fn(),
    givePalTemplates: vi.fn(), exportPals: vi.fn(), exportedPalTemplates: vi.fn(), exportedPalTemplate: vi.fn(),
    accessSettings: vi.fn(), putAccessSettings: vi.fn(), whitelist: vi.fn(), whitelistAdd: vi.fn(),
    whitelistRemove: vi.fn(), toggleAdmin: vi.fn(), giveItems: vi.fn(), sendMessage: vi.fn(),
    broadcast: vi.fn(), kick: vi.fn(), ban: vi.fn(), unban: vi.fn(),
  },
}));

vi.mock('../api/auth', () => ({ authApi: mocks.authApi }));
vi.mock('../api/players', () => ({ playersApi: mocks.playersApi }));
vi.mock('../api/pals', () => ({ palsApi: mocks.palsApi }));
vi.mock('../api/paldefenderGM', () => ({ palDefenderGMApi: mocks.gmApi }));

const saveStatus = { state: 'ready', ready: true };
const builderSave = {
  id: 'steam_1', steam_id: 'steam_1', player_uid: 'uid_1', nickname: 'Builder', level: 45,
  guild_id: 'guild_1', guild_name: 'Guild', is_online: false, last_online_time: '2026-07-15T12:00:00Z',
  x: 10, y: 20, z: 30,
};
const archivistSave = {
  id: 'steam_save', steam_id: 'steam_save', player_uid: 'uid_save', nickname: 'Archivist', level: 18,
  guild_id: '', guild_name: '', is_online: false, last_online_time: '2026-07-14T12:00:00Z',
  x: 101, y: 202, z: 3,
};
const builderGM = {
  Name: 'Builder', IP: '127.0.0.1', UserId: 'steam_1', PlayerUID: 'uid_1', GuildName: 'Guild', GuildUUID: 'guild_1',
  Status: 'Online', WorldLocation: { x: 1, y: 2, z: 3 }, MapLocation: { x: 4, y: 5, z: 6 },
};
const offlineGM = {
  Name: 'OfflineUser', IP: '', UserId: 'steam_2', PlayerUID: 'uid_2', GuildName: '', GuildUUID: '',
  Status: 'Offline', WorldLocation: { x: 0, y: 0, z: 0 }, MapLocation: { x: 0, y: 0, z: 0 },
};

const renderPage = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <MemoryRouter initialEntries={['/gm']}>
      <QueryClientProvider client={client}>
        <ServerStoreProvider>
          <PalDefenderGM />
        </ServerStoreProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  );
};

describe('PalDefender player center', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    mocks.authApi.status.mockResolvedValue({
      initialized: true,
      authenticated: true,
      user: { name: 'admin', role: 'admin', permissions: ['read', 'players:write', 'security:write'] },
    });
    mocks.authApi.logout.mockResolvedValue({ logged_out: true });
    mocks.playersApi.getPlayersList.mockResolvedValue({ items: [builderSave, archivistSave], status: saveStatus, summary: { total: 2, returned: 2 } });
    mocks.playersApi.getPlayer.mockImplementation(async (identifier: string) => ({
      player: identifier === 'uid_save' ? archivistSave : builderSave,
      status: saveStatus,
    }));
    mocks.playersApi.getInventory.mockResolvedValue({
      status: saveStatus,
      containers: [{ container_id: 'bag_1', owner_type: 'player', owner_id: 'uid_1', slots: [{ slot: 0, item_id: 'Money', item_name: '金币', count: 25, durability: 0 }] }],
    });
    mocks.palsApi.getPalsList.mockResolvedValue({
      items: [{ id: 'pal_1', instance_id: 'pal_1', character_id: 'Anubis', name: '阿努比斯', nickname: '矿工', level: 50, rarity: 'Common', gender: 'male', rank: 4, owner_nickname: 'Builder', owner_steam_id: 'steam_1', skills: [], passives: [], raw_passives: [], raw_skills: [], work_suitability: [], health: 100, max_health: 100, status: 'Healthy', x: 0, y: 0, z: 0 }],
      status: saveStatus,
      summary: { total: 1, returned: 1 },
    });
    mocks.gmApi.status.mockResolvedValue({ configured: true, available: true, installed: true, load_verified: true, rest_enabled: true, state: 'ready', version: { Version: '1.8.3' } });
    mocks.gmApi.players.mockResolvedValue({ Meta: { PlayerCount: 2, OnlineCount: 1 }, Players: [builderGM, offlineGM] });
    mocks.gmApi.player.mockImplementation(async (identifier: string) => identifier === 'steam_2' ? offlineGM : builderGM);
    mocks.gmApi.items.mockResolvedValue({ items: [{ id: 'Money', name: '金币', icon: 'money' }, { id: 'ExplosiveBullet', name: '火箭弹', icon: 'explosivebullet' }], returned: 2 });
    mocks.gmApi.inventory.mockResolvedValue({
      Meta: { Player: 'steam_1', PlayerUID: 'uid_1' },
      Inventory: {
        Items: { Available: true, ContainerID: 'bag', UsedSlots: 1, MaxSlots: 42, FreeSlots: 41, Slots: { 0: { ItemID: 'Money', Count: 25 } } },
        KeyItems: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        Weapons: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        Armor: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        Food: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
        DropSlot: { Available: true, ContainerID: '', UsedSlots: 0, MaxSlots: 0, FreeSlots: 0, Slots: {} },
      },
    });
    mocks.gmApi.progression.mockResolvedValue({
      Meta: { Player: 'steam_1', PlayerUID: 'uid_1' },
      Progression: { Player: { level: 45, exp: 1000, unusedStatusPoints: 0 }, Currencies: { relics: {}, technologyPoints: 12, ancientTechnologyPoints: 3 }, Bosses: {}, Captures: {}, Activities: {} },
    });
    mocks.gmApi.techs.mockResolvedValue({ Meta: { Player: 'steam_1', PlayerUID: 'uid_1', UnlockedCount: 1, LockedCount: 1, TotalCount: 2 }, Techs: { Unlocked: ['Technology_1'] } });
    mocks.gmApi.localTechnologyCatalog.mockResolvedValue({ items: [{ id: 'Technology_1', name: '原始作业台', level: 1, category: '建筑', boss: false }, { id: 'Technology_2', name: '石斧', level: 1, category: '科技', boss: false }], returned: 2 });
    mocks.gmApi.technologyCatalog.mockResolvedValue({ catalog: { command: '/gettechids', output: '', entries: ['Technology_1', 'Technology_2'] }, reference_url: '' });
    mocks.gmApi.palCatalog.mockResolvedValue({ items: [{ id: 'Anubis', name: '阿努比斯' }, { id: 'PinkCat', name: '捣蛋猫' }], returned: 2 });
    mocks.gmApi.passiveCatalog.mockResolvedValue({ items: [{ id: 'Legend', name: '传说' }, { id: 'CraftSpeed_up3', name: '卓绝技艺' }], returned: 2 });
    mocks.gmApi.pals.mockResolvedValue({ Meta: { Player: 'steam_1', PlayerUID: 'uid_1', TeamCount: 1, PalboxCount: 10, BaseCampCount: 2 }, Pals: {} });
    mocks.gmApi.templates.mockResolvedValue({ templates: [{ name: 'starter.json', path: 'starter.json', size: 10, modified_at: '' }], reference_url: '' });
    mocks.gmApi.exportedPalTemplates.mockResolvedValue({ player_id: 'steam_1', templates: [{ name: 'anubis.json', path: 'anubis.json', size: 20, modified_at: '' }], reference_url: '' });
    mocks.gmApi.template.mockResolvedValue({ PalID: 'Anubis', Level: 50 });
    mocks.gmApi.exportedPalTemplate.mockResolvedValue({ PalID: 'Anubis', Level: 50, IVs: { Health: 100 } });
    mocks.gmApi.accessSettings.mockResolvedValue({ use_whitelist: false, whitelist_message: 'Not allowed', use_admin_whitelist: false, admin_auto_login: false, admin_ips: ['127.0.0.1'], reload_required: false, reference_url: '' });
    mocks.gmApi.whitelist.mockResolvedValue({ command: '/whitelist_get', output: '', entries: [] });
    for (const name of ['removeItems', 'teleport', 'giveProgression', 'learnTech', 'forgetTech', 'givePals', 'giveCustomPal', 'releasePal', 'givePalTemplates', 'exportPals', 'putTemplate', 'putAccessSettings', 'whitelistAdd', 'whitelistRemove', 'toggleAdmin', 'sendMessage', 'broadcast', 'kick', 'ban', 'unban'] as const) {
      mocks.gmApi[name].mockResolvedValue({ Success: true });
    }
    mocks.gmApi.giveItems.mockResolvedValue({ Granted: { Items: 5 } });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('merges save-index and PalDefender players and selects the online player first', async () => {
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Builder' }, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Builder 在线' })).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Archivist 离线' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'OfflineUser 离线' })).toBeInTheDocument();
    await waitFor(() => expect(mocks.gmApi.player).toHaveBeenCalledWith('steam_1'));
    expect(mocks.playersApi.getPlayer).toHaveBeenCalledWith('uid_1');
  });

  it('keeps save players available when PalDefender is not configured', async () => {
    mocks.gmApi.status.mockResolvedValue({ configured: false, available: false, state: 'not_configured' });
    mocks.playersApi.getPlayersList.mockResolvedValue({ items: [archivistSave], status: saveStatus, summary: { total: 1, returned: 1 } });
    renderPage();

    expect(await screen.findByRole('heading', { name: 'Archivist' })).toBeInTheDocument();
    expect(screen.getAllByText('REST Token 未配置').length).toBeGreaterThan(0);
    expect(screen.getByText(/存档玩家、帕鲁和背包快照仍可正常查看/)).toBeInTheDocument();
    expect(mocks.gmApi.players).not.toHaveBeenCalled();
    expect(mocks.gmApi.player).not.toHaveBeenCalled();
  });

  it('gives a catalog item to the selected online player and refreshes inventory', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '物品' }));

    const itemImage = await screen.findByRole('img', { name: '金币图标' });
    fireEvent.click(itemImage.closest('button')!);
    fireEvent.change(screen.getByLabelText('发放数量'), { target: { value: '5' } });
    fireEvent.click(screen.getByRole('button', { name: '加入列表' }));
    fireEvent.click(screen.getByRole('button', { name: '确认发放' }));

    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Builder'));
    await waitFor(() => expect(mocks.gmApi.giveItems).toHaveBeenCalledWith('steam_1', [{ ItemID: 'Money', Count: 5 }]));
    expect(await screen.findByText('已向 Builder 发放 5 件物品')).toBeInTheDocument();
    await waitFor(() => expect(mocks.gmApi.inventory.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('grants technology points and unlocks multiple technologies for the selected player', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '成长' }));

    fireEvent.change(screen.getByLabelText('普通科技点'), { target: { value: '10' } });
    fireEvent.change(screen.getByLabelText('古代科技点'), { target: { value: '2' } });
    fireEvent.click(screen.getByRole('button', { name: '确认发放' }));
    await waitFor(() => expect(mocks.gmApi.giveProgression).toHaveBeenCalledWith('steam_1', { TechnologyPoints: 10, AncientTechnologyPoints: 2 }));

    fireEvent.change(screen.getByLabelText('科技 ID'), { target: { value: 'Technology_2\nTechnology_3' } });
    fireEvent.click(screen.getByRole('button', { name: '解锁科技' }));
    await waitFor(() => expect(mocks.gmApi.learnTech).toHaveBeenCalledWith('steam_1', { Technology: ['Technology_2', 'Technology_3'] }));
  });

  it('adjusts an existing item total by removing the negative delta', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '物品' }));
    fireEvent.click(await screen.findByRole('button', { name: '调整总量' }));
    fireEvent.change(screen.getByLabelText('目标物品总量'), { target: { value: '20' } });
    fireEvent.click(screen.getByRole('button', { name: '确认调整' }));
    await waitFor(() => expect(mocks.gmApi.removeItems).toHaveBeenCalledWith('steam_1', { Items: [{ ItemID: 'Money', Count: 5 }] }));
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('25 → 20'));
  });

  it('teleports the selected online player to coordinates', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '传送' }));
    fireEvent.change(screen.getByLabelText('X'), { target: { value: '100' } });
    fireEvent.change(screen.getByLabelText('Y'), { target: { value: '-200' } });
    fireEvent.click(screen.getByRole('button', { name: '确认传送' }));
    await waitFor(() => expect(mocks.gmApi.teleport).toHaveBeenCalledWith('steam_1', { Mode: 'coordinates', X: 100, Y: -200 }));
  });

  it('gives a pal and loads an exported pal into the template editor', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '帕鲁' }));

    fireEvent.change(screen.getByLabelText('帕鲁 ID'), { target: { value: 'Anubis' } });
    fireEvent.change(screen.getByLabelText('帕鲁等级'), { target: { value: '50' } });
    fireEvent.click(screen.getByRole('button', { name: '发放帕鲁' }));
    await waitFor(() => expect(mocks.gmApi.givePals).toHaveBeenCalledWith('steam_1', { Pals: [{ PalID: 'Anubis', Level: 50 }] }));

    fireEvent.click(screen.getByRole('button', { name: '导出玩家帕鲁' }));
    await waitFor(() => expect(mocks.gmApi.exportPals).toHaveBeenCalledWith('steam_1'));
    await waitFor(() => expect(mocks.gmApi.exportedPalTemplate).toHaveBeenCalledWith('steam_1', 'anubis.json'));
    expect(screen.getByLabelText('模板名称')).toHaveValue('anubis');
    expect(screen.getByLabelText('IV 生命')).toHaveValue(100);

    fireEvent.click(await screen.findByRole('button', { name: /卓绝技艺/ }));
    fireEvent.click(screen.getByRole('checkbox', { name: '觉醒个体' }));
    fireEvent.change(screen.getByLabelText('IV 近战攻击'), { target: { value: '95' } });
    fireEvent.change(screen.getByLabelText('魂强化 作业速度'), { target: { value: '10' } });
    fireEvent.change(screen.getByLabelText('采矿'), { target: { value: '5' } });
    fireEvent.click(screen.getByRole('button', { name: '直接发放当前配置' }));
    await waitFor(() => expect(mocks.gmApi.giveCustomPal).toHaveBeenCalledWith('steam_1', expect.objectContaining({
      PalID: 'Anubis', IsAwakening: true, Passives: ['CraftSpeed_up3'],
      IVs: expect.objectContaining({ Health: 100, AttackMelee: 95 }),
      PalSouls: expect.objectContaining({ CraftSpeed: 10 }),
      ExtraWorkSuitabilities: { Mining: 5 },
    })));
  });

  it('creates a real template file from the selected PalID and shows confirmation', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '帕鲁' }));
    fireEvent.change(screen.getByLabelText('帕鲁 ID'), { target: { value: 'Anubis' } });
    fireEvent.click(screen.getByRole('button', { name: '新建模板文件' }));
    await waitFor(() => expect(mocks.gmApi.putTemplate).toHaveBeenCalledWith(expect.stringMatching(/^pal_Anubis_\d+$/), { PalID: 'Anubis', Level: 1 }));
    expect(await screen.findByText(/模板文件 .*\.json 已创建/)).toBeInTheDocument();
    expect((screen.getByLabelText('模板名称') as HTMLInputElement).value).toMatch(/^pal_Anubis_\d+$/);
  });

  it('explains why a template file cannot be created without a PalID', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '帕鲁' }));
    fireEvent.click(screen.getByRole('button', { name: '新建模板文件' }));
    expect(await screen.findByText(/新建模板前请先在上方选择帕鲁/)).toBeInTheDocument();
    expect(mocks.gmApi.putTemplate).not.toHaveBeenCalled();
  });

  it('disables live PalDefender export for an offline player but keeps prior exports available', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'OfflineUser 离线' }));
    fireEvent.click(await screen.findByRole('tab', { name: '帕鲁' }));
    expect(screen.getByRole('button', { name: '导出玩家帕鲁' })).toBeDisabled();
    expect(screen.getByText(/玩家离线时 PalDefender 不会加载用于导出的帕鲁容器/)).toBeInTheDocument();
    expect(screen.getByLabelText('导出帕鲁模板')).toBeEnabled();
  });

  it('imports a JSON Pal template and preserves supported fields outside the visual editor', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '帕鲁' }));

    const payload = JSON.stringify({
      PalID: 'Anubis', Level: 55, Exp: 123456, Passives: ['Legend'],
      DisableWorkPreferences: ['Mining'], PhysicalHealth: 'Healthful', IsAwakening: true,
    });
    const file = new File([payload], 'legend-anubis.json', { type: 'application/json' });
    if (typeof file.text !== 'function') Object.defineProperty(file, 'text', { value: vi.fn().mockResolvedValue(payload) });
    fireEvent.change(screen.getByLabelText('导入帕鲁模板文件'), { target: { files: [file] } });

    expect(await screen.findByText(/已导入 legend-anubis.json/)).toBeInTheDocument();
    const templatePalID = screen.getAllByRole('textbox').find((input) => input.getAttribute('aria-label') === 'PalID');
    expect(templatePalID).toHaveValue('Anubis');
    expect(screen.getByRole('checkbox', { name: '觉醒个体' })).toBeChecked();
    fireEvent.click(screen.getByRole('button', { name: '直接发放当前配置' }));
    await waitFor(() => expect(mocks.gmApi.giveCustomPal).toHaveBeenCalledWith('steam_1', expect.objectContaining({
      PalID: 'Anubis', Level: 55, Exp: 123456, Passives: ['Legend'], IsAwakening: true,
      DisableWorkPreferences: ['Mining'], PhysicalHealth: 'Healthful',
    })));
  });

  it('requires typed confirmation before releasing one matching pal', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '帕鲁' }));
    fireEvent.click(await screen.findByRole('button', { name: '放生 矿工' }));
    fireEvent.change(screen.getByLabelText('放生确认玩家名称'), { target: { value: 'Builder' } });
    fireEvent.click(screen.getByRole('button', { name: '确认放生' }));
    await waitFor(() => expect(mocks.gmApi.releasePal).toHaveBeenCalledWith('steam_1', { PalID: 'Anubis', Level: 50, Gender: 'male', Rank: 4 }));
  });

  it('manages the current player whitelist, temporary admin, and persistent access settings', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '管理' }));

    fireEvent.click(await screen.findByRole('button', { name: '加入白名单' }));
    await waitFor(() => expect(mocks.gmApi.whitelistAdd).toHaveBeenCalledWith('steam_1'));
    fireEvent.click(screen.getByRole('button', { name: '切换临时管理员' }));
    await waitFor(() => expect(mocks.gmApi.toggleAdmin).toHaveBeenCalledWith('steam_1'));

    fireEvent.click(screen.getByRole('checkbox', { name: '启用玩家白名单' }));
    fireEvent.change(screen.getByLabelText('白名单提示'), { target: { value: '请联系管理员' } });
    fireEvent.click(screen.getByRole('button', { name: '保存访问配置' }));
    await waitFor(() => expect(mocks.gmApi.putAccessSettings).toHaveBeenCalledWith(expect.objectContaining({ use_whitelist: true, whitelist_message: '请联系管理员', admin_ips: ['127.0.0.1'] })));
  });

  it('keeps writes disabled for a read-only session', async () => {
    mocks.authApi.status.mockResolvedValue({ initialized: true, authenticated: true, user: { name: 'viewer', role: 'viewer', permissions: ['read'] } });
    renderPage();
    fireEvent.click(await screen.findByRole('tab', { name: '物品' }));

    const itemImage = await screen.findByRole('img', { name: '金币图标' });
    fireEvent.click(itemImage.closest('button')!);
    fireEvent.click(screen.getByRole('button', { name: '加入列表' }));
    expect(screen.getByRole('button', { name: '确认发放' })).toBeDisabled();

    fireEvent.click(screen.getByRole('tab', { name: '管理' }));
    expect(screen.getByText(/security:write/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '封禁' })).toBeDisabled();
  });

  it('blocks online-only actions for an offline PalDefender player', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'OfflineUser 离线' }));
    await waitFor(() => expect(mocks.gmApi.player).toHaveBeenCalledWith('steam_2'));

    fireEvent.click(screen.getByRole('tab', { name: '物品' }));
    const itemImage = await screen.findByRole('img', { name: '金币图标' });
    fireEvent.click(itemImage.closest('button')!);
    fireEvent.click(screen.getByRole('button', { name: '加入列表' }));
    expect(screen.getByRole('button', { name: '确认发放' })).toBeDisabled();

    fireEvent.click(screen.getByRole('tab', { name: '消息' }));
    fireEvent.change(screen.getByLabelText('消息内容'), { target: { value: 'hello' } });
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled();

    fireEvent.click(screen.getByRole('tab', { name: '管理' }));
    expect(screen.getByRole('button', { name: '踢出' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '封禁' })).toBeEnabled();
  });
});
