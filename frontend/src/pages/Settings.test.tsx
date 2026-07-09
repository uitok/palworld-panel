import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { settingsApi } from '../api/settings';
import { serverApi } from '../api/server';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { storageKeys } from '../config/defaults';
import { Settings } from './Settings';

vi.mock('../api/settings', () => ({
  settingsApi: {
    getSchema: vi.fn(),
    getSettings: vi.fn(),
    validateSettings: vi.fn(),
    updateSettings: vi.fn(),
  },
}));

vi.mock('../api/server', () => ({
  serverApi: {
    getStatus: vi.fn(),
  },
}));

const renderSettings = () =>
  render(
    <ServerStoreProvider>
      <Settings />
    </ServerStoreProvider>,
  );

describe('Settings page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem(storageKeys.token, 'test-token');

    vi.mocked(serverApi.getStatus).mockResolvedValue({
      status: 'stopped',
      installed: true,
      pending_restart: false,
      runtime_mode: 'wine_docker',
      setup_step: 'configured',
      config_exists: true,
      container: { exists: false, status: 'missing' },
      startup_args: [],
      ports: { game: 8211, rest: 8212 },
      warnings: [],
      paths: {},
      settings_path: '/srv/PalWorldSettings.ini',
    });

    vi.mocked(settingsApi.getSchema).mockResolvedValue({
      version: '0.7.2',
      fields: [
        {
          key: 'ServerName',
          label: '服务器名称',
          group: 'server_management',
          type: 'string',
          default: 'Palworld Server',
          requires_restart: true,
          description: '服务器名称。',
        },
        {
          key: 'DeathPenalty',
          label: '死亡惩罚',
          group: 'server_management',
          type: 'enum',
          default: 'All',
          enum: ['None', 'All'],
          enum_labels: {
            None: '不掉落',
            All: '全部掉落（物品、装备和队伍帕鲁）',
          },
          requires_restart: true,
          description: '死亡惩罚。',
        },
      ],
    });

    vi.mocked(settingsApi.getSettings).mockResolvedValue({
      settings: { ServerName: '测试服', DeathPenalty: 'All' },
      path: '/srv/PalWorldSettings.ini',
      pending_restart: false,
      issues: [],
    });
    vi.mocked(settingsApi.validateSettings).mockResolvedValue({ valid: true, issues: [] });
    vi.mocked(settingsApi.updateSettings).mockResolvedValue({
      settings: { ServerName: '测试服', DeathPenalty: 'None' },
      path: '/srv/PalWorldSettings.ini',
      pending_restart: true,
      issues: [],
    });
  });

  it('shows localized field and enum labels while saving raw keys and enum values', async () => {
    renderSettings();

    expect(await screen.findByText('服务器名称')).toBeInTheDocument();
    expect(screen.getByText('ServerName')).toBeInTheDocument();
    expect(screen.getByText('全部掉落（物品、装备和队伍帕鲁）')).toBeInTheDocument();

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'None' } });
    fireEvent.click(screen.getByRole('button', { name: /^保存$/ }));

    await waitFor(() => {
      expect(settingsApi.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          ServerName: '测试服',
          DeathPenalty: 'None',
        }),
      );
    });
  });
});
