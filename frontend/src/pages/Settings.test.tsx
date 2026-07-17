import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { settingsApi } from '../api/settings';
import { serverApi } from '../api/server';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { Settings } from './Settings';

const auxiliaryMocks = vi.hoisted(() => ({
	authStatus: vi.fn(),
  authMe: vi.fn(),
	listKeys: vi.fn(),
	createKey: vi.fn(),
	revokeKey: vi.fn(),
	clipboardWrite: vi.fn(),
  getAIConfig: vi.fn(),
  updateAIConfig: vi.fn(),
  testAIConfig: vi.fn(),
}));

vi.mock('../api/auth', () => ({ authApi: {
	status: auxiliaryMocks.authStatus,
	me: auxiliaryMocks.authMe,
	listKeys: auxiliaryMocks.listKeys,
	createKey: auxiliaryMocks.createKey,
	revokeKey: auxiliaryMocks.revokeKey,
} }));
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
	Object.defineProperty(navigator, 'clipboard', {
	  configurable: true,
	  value: { writeText: auxiliaryMocks.clipboardWrite },
	});
	auxiliaryMocks.authStatus.mockResolvedValue({ initialized: true, authenticated: true, user: { name: 'admin', role: 'admin', permissions: ['read', 'ai:config', 'security:write'] } });
    auxiliaryMocks.authMe.mockResolvedValue({ name: 'admin', role: 'admin', permissions: ['read', 'ai:config'] });
	auxiliaryMocks.listKeys.mockResolvedValue([]);
	auxiliaryMocks.createKey.mockResolvedValue({ id: 'key_1', name: '本机自动化', prefix: 'ppk_example', token: 'ppk_full-once', created_at: '2026-07-14T00:00:00Z' });
	auxiliaryMocks.revokeKey.mockResolvedValue({ revoked: true });
    auxiliaryMocks.getAIConfig.mockResolvedValue({ configured: false, base_url: '', model: '', api_key_present: false, timeout_seconds: 90, proxy_configured: false, proxy_url: '', custom_header_names: [] });
    auxiliaryMocks.updateAIConfig.mockResolvedValue({ configured: true, base_url: 'https://ai.example/v1', model: 'translate-model', api_key_present: true, timeout_seconds: 90, proxy_configured: false, proxy_url: '', custom_header_names: [] });
    auxiliaryMocks.testAIConfig.mockResolvedValue({ ok: true, base_url: 'https://ai.example/v1', model: 'translate-model', message: 'ok', timeout_seconds: 90, proxy_configured: false, custom_header_names: [] });

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
      server_imported: false,
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
    auxiliaryMocks.getAIConfig.mockResolvedValue({ configured: true, base_url: 'https://ai.example/v1', model: 'old-model', api_key_present: true, timeout_seconds: 90, proxy_configured: false, proxy_url: '', custom_header_names: [] });
    renderSettings();

    await screen.findByRole('button', { name: '保存 AI 配置' });
    fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'translate-model' } });
    fireEvent.click(screen.getByRole('button', { name: '保存 AI 配置' }));

    await waitFor(() => {
      expect(auxiliaryMocks.updateAIConfig).toHaveBeenCalledWith({
        base_url: 'https://ai.example/v1',
        model: 'translate-model',
        timeout_seconds: 90,
      });
    });
  });

  it('saves AI timeout, proxy, and private custom headers without replacing existing secrets', async () => {
    auxiliaryMocks.getAIConfig.mockResolvedValue({
      configured: true,
      base_url: 'https://ai.example/v1',
      model: 'old-model',
      api_key_present: true,
      timeout_seconds: 45,
      proxy_configured: true,
      proxy_url: 'socks5://127.0.0.1:10808',
      custom_header_names: ['X-Tenant-ID'],
    });
    renderSettings();

    await screen.findByRole('button', { name: '保存 AI 配置' });
    fireEvent.change(screen.getByLabelText('请求超时（秒）'), { target: { value: '120' } });
    fireEvent.change(screen.getByLabelText(/Proxy URL/), { target: { value: 'socks5://proxy-user:proxy-pass@127.0.0.1:10808' } });
    fireEvent.change(screen.getByLabelText(/自定义请求头/), { target: { value: '{"X-Tenant-ID":"tenant-b"}' } });
    fireEvent.click(screen.getByRole('button', { name: '保存 AI 配置' }));

    await waitFor(() => {
      expect(auxiliaryMocks.updateAIConfig).toHaveBeenCalledWith({
        base_url: 'https://ai.example/v1',
        model: 'old-model',
        timeout_seconds: 120,
        proxy_url: 'socks5://proxy-user:proxy-pass@127.0.0.1:10808',
        custom_headers: { 'X-Tenant-ID': 'tenant-b' },
      });
    });
  });

  it('shows a development key once, copies it, and revokes it', async () => {
	renderSettings();

	await screen.findByText('开发密钥');
	fireEvent.click(screen.getByRole('button', { name: '创建' }));
	expect(await screen.findByText('ppk_full-once')).toBeInTheDocument();
	fireEvent.click(screen.getByRole('button', { name: '复制开发密钥' }));
	expect(auxiliaryMocks.clipboardWrite).toHaveBeenCalledWith('ppk_full-once');
	fireEvent.click(screen.getByRole('button', { name: '撤销 本机自动化' }));

	await waitFor(() => expect(auxiliaryMocks.revokeKey).toHaveBeenCalledWith('key_1'));
	expect((await screen.findAllByText(/已撤销/)).length).toBeGreaterThan(0);
  });
});
