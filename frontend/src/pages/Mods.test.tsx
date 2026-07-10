import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { storageKeys } from '../config/defaults';
import { Mods } from './Mods';

const mocks = vi.hoisted(() => ({
  modsApi: {
    list: vi.fn(),
    workshopStatus: vi.fn(),
    searchWorkshop: vi.fn(),
    getWorkshopItem: vi.fn(),
    translateWorkshop: vi.fn(),
    downloadWorkshop: vi.fn(),
    upload: vi.fn(),
    setEnabled: vi.fn(),
    delete: vi.fn(),
  },
  serverApi: {
    getStatus: vi.fn(),
  },
  tasksApi: {
    waitForJob: vi.fn(),
  },
  authApi: {
    me: vi.fn(),
  },
}));

vi.mock('../api/mods', () => ({
  modsApi: mocks.modsApi,
}));

vi.mock('../api/server', () => ({
  serverApi: mocks.serverApi,
}));

vi.mock('../api/tasks', () => ({
  tasksApi: mocks.tasksApi,
}));

vi.mock('../api/auth', () => ({
  authApi: mocks.authApi,
}));

const renderMods = () =>
  render(
    <ServerStoreProvider>
      <Mods />
    </ServerStoreProvider>,
  );

describe('Mods Workshop store', () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem(storageKeys.token, 'operator-token');
    mocks.modsApi.list.mockResolvedValue([]);
    mocks.modsApi.workshopStatus.mockResolvedValue({ configured: true, key_source: 'embedded', app_id: '1623730' });
    mocks.modsApi.searchWorkshop.mockResolvedValue({ items: [], total: 0, page_size: 24 });
    mocks.modsApi.getWorkshopItem.mockResolvedValue({
      id: '123456789', title: 'Test Mod', summary: 'Original description', steam_url: 'https://steamcommunity.com/sharedfiles/filedetails/?id=123456789',
      tags: [], installed: false, enabled: false, update_available: false,
    });
    mocks.modsApi.translateWorkshop.mockResolvedValue({
      text: '中文译文', target_language: 'zh-CN', model: 'translate-model', generated_at: '2026-07-10T00:00:00Z', cached: false,
    });
    mocks.modsApi.downloadWorkshop.mockResolvedValue({
      id: 'job_1',
      type: 'workshop_download',
      status: 'waiting',
      progress: 0,
      message: 'queued',
      created_at: new Date(0).toISOString(),
    });
    mocks.serverApi.getStatus.mockResolvedValue({ pending_restart: false });
    mocks.authApi.me.mockResolvedValue({ name: 'operator', role: 'operator', permissions: ['read', 'mods:write'] });
    mocks.tasksApi.waitForJob.mockResolvedValue({
      id: 'job_1',
      type: 'workshop_download',
      status: 'success',
      progress: 100,
      message: 'done',
      created_at: new Date(0).toISOString(),
    });
  });

  it('searches the store even when status reports configured false', async () => {
    mocks.modsApi.workshopStatus.mockResolvedValue({ configured: false, app_id: '1623730' });

    renderMods();

    await waitFor(() => expect(mocks.modsApi.searchWorkshop).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('Mod 商店搜索未启用')).not.toBeInTheDocument();
  });

  it('shows Steam API errors without replacing them with key setup text', async () => {
    mocks.modsApi.searchWorkshop.mockRejectedValue(new Error('Steam API returned HTTP 403: forbidden'));

    renderMods();

    expect(await screen.findByText('Steam API returned HTTP 403: forbidden')).toBeInTheDocument();
    expect(screen.queryByText(/STEAM_WEB_API_KEY/)).not.toBeInTheDocument();
  });

  it('keeps manual Workshop ID install available after store search fails', async () => {
    mocks.modsApi.searchWorkshop.mockRejectedValue(new Error('Steam API request timed out'));

    renderMods();

    expect(await screen.findByText('Steam API request timed out')).toBeInTheDocument();
    fireEvent.click(screen.getByText('手动安装'));
    fireEvent.change(screen.getByPlaceholderText('Workshop Item ID'), { target: { value: '123456789' } });
    fireEvent.click(screen.getByText('下载'));

    await waitFor(() => expect(mocks.modsApi.downloadWorkshop).toHaveBeenCalledWith('123456789', false));
  });

  it('translates authoritative Workshop details and switches to Chinese', async () => {
    mocks.modsApi.searchWorkshop.mockResolvedValue({
      items: [{
        id: '123456789', title: 'Test Mod', summary: 'Short', steam_url: 'https://steamcommunity.com/sharedfiles/filedetails/?id=123456789',
        tags: [], installed: false, enabled: false, update_available: false,
      }],
      total: 1,
      page_size: 24,
    });
    renderMods();

    fireEvent.click(await screen.findByRole('button', { name: '查看详情' }));
    fireEvent.click(await screen.findByRole('button', { name: '翻译为中文' }));

    await waitFor(() => expect(mocks.modsApi.translateWorkshop).toHaveBeenCalledWith('123456789', false));
    expect(await screen.findByText('中文译文')).toBeInTheDocument();
    expect(screen.getByText(/translate-model/)).toBeInTheDocument();
  });
});
