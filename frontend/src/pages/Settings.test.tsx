import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { settingsApi } from '../api/settings';
import { serverApi } from '../api/server';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { storageKeys } from '../config/defaults';
import { Settings } from './Settings';

const auxiliaryMocks = vi.hoisted(() => ({
  authMe: vi.fn(),
  getAIConfig: vi.fn(),
  updateAIConfig: vi.fn(),
  testAIConfig: vi.fn(),
}));

vi.mock('../api/auth', () => ({ authApi: { me: auxiliaryMocks.authMe } }));
vi.mock('../api/aiTranslation', () => ({
  aiTranslationApi: {
    getConfig: auxiliaryMocks.getAIConfig,
    updateConfig: auxiliaryMocks.updateAIConfig,
    testConfig: auxiliaryMocks.testAIConfig,
  },
}));

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
    getVersion: vi.fn(),
  },
}));

const renderSettings = () =>
  render(
    <ServerStoreProvider>
      <Settings />
    </ServerStoreProvider>,
  );

describe('Settings page', () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem(storageKeys.token, 'test-token');
    auxiliaryMocks.authMe.mockResolvedValue({ name: 'admin', role: 'admin', permissions: ['read', 'ai:config'] });
    auxiliaryMocks.getAIConfig.mockResolvedValue({ configured: false, base_url: '', model: '', api_key_present: false });
    auxiliaryMocks.updateAIConfig.mockResolvedValue({ configured: true, base_url: 'https://ai.example/v1', model: 'translate-model', api_key_present: true });
    auxiliaryMocks.testAIConfig.mockResolvedValue({ ok: true, base_url: 'https://ai.example/v1', model: 'translate-model', message: 'ok' });

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
    vi.mocked(serverApi.getVersion).mockResolvedValue({
      installed: true,
      current_build_id: '24088465',
      latest_build_id: '24088465',
      update_available: false,
      last_checked_at: '2026-07-10T00:00:00Z',
      source: 'test',
      manifest_path: '/srv/appmanifest_2394010.acf',
      game_version: 'v1.0.0.81201',
      compatibility_target: '1.0.0',
      compatible: true,
      compatibility_warnings: [],
    });

    vi.mocked(settingsApi.getSchema).mockResolvedValue({
      version: '1.0.0',
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
        {
          key: 'bEnableVoiceChat',
          label: '启用语音聊天',
          group: 'server_management',
          type: 'bool',
          default: 'False',
          requires_restart: true,
          description: '是否启用游戏内语音聊天。',
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
    expect(screen.getByText('配置规范 1.0.0')).toBeInTheDocument();
    expect(screen.getByText('v1.0.0.81201')).toBeInTheDocument();
    expect(screen.getByText(/当前未设置/)).toBeInTheDocument();
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
      expect(settingsApi.updateSettings).not.toHaveBeenCalledWith(
        expect.objectContaining({ bEnableVoiceChat: expect.anything() }),
      );
    });
  });

  it('saves AI configuration without requiring an API key replacement', async () => {
    auxiliaryMocks.getAIConfig.mockResolvedValue({ configured: true, base_url: 'https://ai.example/v1', model: 'old-model', api_key_present: true });
    renderSettings();

    expect(await screen.findByText('AI 翻译')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'translate-model' } });
    fireEvent.click(screen.getByRole('button', { name: '保存 AI 配置' }));

    await waitFor(() => {
      expect(auxiliaryMocks.updateAIConfig).toHaveBeenCalledWith({
        base_url: 'https://ai.example/v1',
        model: 'translate-model',
      });
    });
  });
});
