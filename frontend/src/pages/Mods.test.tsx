import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
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
    inspectImport: vi.fn(),
    selectImportCandidate: vi.fn(),
    importInspected: vi.fn(),
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
    status: vi.fn(),
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
    mocks.authApi.status.mockResolvedValue({ initialized: true, authenticated: true, user: { name: 'operator', role: 'operator', permissions: ['read', 'mods:write'] } });
    mocks.modsApi.list.mockResolvedValue([]);
    mocks.modsApi.workshopStatus.mockResolvedValue({ configured: true, key_source: 'environment', app_id: '1623730' });
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
    mocks.modsApi.inspectImport.mockResolvedValue({
      id: 'inspection_1', source_type: 'workshop', source: '123456789', expires_at: '2026-07-14T01:00:00Z',
      selected_candidate_id: 'candidate_1',
      candidates: [{ id: 'candidate_1', source_type: 'workshop', file_name: '123456789', action: 'unknown', ready: true, warnings: [] }],
    });
    mocks.modsApi.importInspected.mockResolvedValue({
      id: 'job_import', type: 'mod_import', status: 'waiting', progress: 0, message: 'queued', created_at: new Date(0).toISOString(),
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

  it('keeps unified Workshop import available after store search fails', async () => {
    mocks.modsApi.searchWorkshop.mockRejectedValue(new Error('Steam API request timed out'));

    renderMods();

    expect(await screen.findByText('Steam API request timed out')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '导入 Mod' }));
    fireEvent.change(screen.getByLabelText('导入来源'), { target: { value: '123456789' } });
    fireEvent.click(screen.getByRole('button', { name: '检查' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认导入' }));

    await waitFor(() => expect(mocks.modsApi.inspectImport).toHaveBeenCalledWith({ source: '123456789', file: undefined }));
    expect(mocks.modsApi.importInspected).toHaveBeenCalledWith('inspection_1', 'candidate_1');
  });

  it('downloads and inspects a selected GitHub ZIP candidate before import', async () => {
    mocks.modsApi.inspectImport.mockResolvedValue({
      id: 'inspection_github',
      source_type: 'github_release',
      source: 'https://github.com/example/mod/releases/latest',
      expires_at: '2026-07-14T01:00:00Z',
      candidates: [
        { id: 'candidate_one', source_type: 'github_asset', file_name: 'client.zip', action: 'unknown', ready: false },
        { id: 'candidate_two', source_type: 'github_asset', file_name: 'server.zip', action: 'unknown', ready: false },
      ],
    });
    mocks.modsApi.selectImportCandidate.mockResolvedValue({
      id: 'inspection_github',
      source_type: 'github_release',
      source: 'https://github.com/example/mod/releases/latest',
      expires_at: '2026-07-14T01:00:00Z',
      selected_candidate_id: 'candidate_two',
      candidates: [
        { id: 'candidate_one', source_type: 'github_asset', file_name: 'client.zip', action: 'unknown', ready: false },
        {
          id: 'candidate_two', source_type: 'github_asset', file_name: 'server.zip', name: 'Server Mod',
          package_name: 'ServerMod', version: '2.0', action: 'update', existing_mod_id: 'mod_existing', ready: true,
          warnings: ['The existing enabled state and record identity will be preserved.'],
        },
      ],
    });

    renderMods();
    fireEvent.click(await screen.findByRole('button', { name: '导入 Mod' }));
    fireEvent.change(screen.getByLabelText('导入来源'), { target: { value: 'https://github.com/example/mod/releases/latest' } });
    fireEvent.click(screen.getByRole('button', { name: '检查' }));
    fireEvent.change(await screen.findByLabelText('候选 ZIP'), { target: { value: 'candidate_two' } });

    await waitFor(() => expect(mocks.modsApi.selectImportCandidate).toHaveBeenCalledWith('inspection_github', 'candidate_two'));
    expect(await screen.findByText('更新现有 Mod')).toBeInTheDocument();
    expect(screen.getByText('ServerMod')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '确认导入' }));
    await waitFor(() => expect(mocks.modsApi.importInspected).toHaveBeenCalledWith('inspection_github', 'candidate_two'));
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
