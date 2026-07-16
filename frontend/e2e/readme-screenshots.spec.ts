import { expect, test, type Page } from '@playwright/test';

const envelope = (data: unknown) => ({ ok: true, data });

const adminSession = {
  name: 'PalPanel 管理员',
  role: 'admin',
  permissions: [
    'read',
    'server:control',
    'config:write',
    'backup:write',
    'mods:write',
    'players:write',
    'security:write',
    'audit:read',
    'world:reset',
    'ai:config',
  ],
};

const saveStatus = {
  enabled: true,
  state: 'ready',
  stale: false,
  source_path: '',
  updated_at: '2026-07-16T09:18:00+08:00',
  duration_ms: 1486,
  warnings: [],
  parser: 'palpanel-sav-cli-go',
  counts: {
    players: 8,
    guilds: 2,
    bases: 3,
    pals: 126,
    containers: 12,
    map_entities: 40,
  },
};

const breedingCatalog = {
  version: 'Palworld v0.6 · 299 Pals',
  pals: [
    { id: 'Anubis', name: '阿努比斯' },
    { id: 'Jetragon', name: '空涡龙' },
    { id: 'Lamball', name: '棉悠悠' },
    { id: 'Lyleen', name: '百合女王' },
    { id: 'Shadowbeak', name: '异构格里芬' },
  ],
  passives: [
    { id: 'Legend', name: '传说', supports_surgery: true, surgery_cost: 1000 },
    { id: 'Musclehead', name: '脑筋', supports_surgery: false, surgery_cost: 0 },
    { id: 'Serenity', name: '沉着冷静', supports_surgery: true, surgery_cost: 600 },
    { id: 'Swift', name: '神速', supports_surgery: false, surgery_cost: 0 },
    { id: 'Artisan', name: '工匠精神', supports_surgery: true, surgery_cost: 500 },
    { id: 'Lucky', name: '稀有', supports_surgery: false, surgery_cost: 0 },
  ],
  active_skills: [],
};

const monitorHistory = Array.from({ length: 12 }, (_, index) => ({
  id: `sample-${index}`,
  created_at: new Date(Date.UTC(2026, 6, 16, 0, index * 5)).toISOString(),
  cpu_available: true,
  cpu_percent: 24 + ((index * 7) % 21),
  memory_available: true,
  memory_usage_bytes: (5.4 + index * 0.08) * 1024 * 1024 * 1024,
  memory_limit_bytes: 16 * 1024 * 1024 * 1024,
  disk_available: true,
  disk_free_bytes: 182 * 1024 * 1024 * 1024,
  disk_total_bytes: 512 * 1024 * 1024 * 1024,
  current_players: 3 + (index % 4),
  max_players: 32,
  rest_healthy: true,
  rcon_healthy: true,
  game_port_healthy: true,
  query_port_healthy: true,
}));

const installAdminBackend = async (page: Page) => {
  await page.route((url) => url.pathname.startsWith('/api/'), async (route) => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = {};

    switch (path) {
      case '/api/auth/status':
        data = { initialized: true, authenticated: true, user: adminSession };
        break;
      case '/api/auth/me':
        data = adminSession;
        break;
      case '/api/server/status':
        data = {
          installed: true,
          status: 'running',
          pending_restart: false,
          runtime_mode: 'windows_steamcmd',
          setup_step: 'ready',
          config_exists: true,
          container: { exists: true, status: 'running' },
          startup_args: ['-useperfthreads', '-NoAsyncLoadingThread'],
          ports: { game: 8211, query: 27015, rest: 8212 },
          warnings: [],
          paths: { palworld_settings: 'D:\\PalServer\\Pal\\Saved\\Config\\WindowsServer\\PalWorldSettings.ini' },
          server_imported: true,
          pid: 18420,
          cpu_percent: 31.8,
          memory_usage_bytes: 6.2 * 1024 * 1024 * 1024,
          version: 'v0.6.7.81231',
        };
        break;
      case '/api/server/metrics':
        data = {
          server_fps: 59.8,
          current_players: 6,
          max_players: 32,
          uptime: 201180,
          total_pals: 126,
          active_bases: 3,
          frame_time: 16.7,
          days: 482,
        };
        break;
      case '/api/server/logs':
        data = {
          logs: '[09:16:03] Server startup complete\n[09:16:08] REST API listening on 8212\n[09:17:21] Player Cattiva joined the server\n[09:18:02] World save completed',
          source: 'file',
          available: true,
          updated_at: '2026-07-16T09:18:02+08:00',
        };
        break;
      case '/api/server/world':
        data = {
          active_world_id: '7F2A4B8C40E14817A30F225994CF9001',
          save_exists: true,
          last_modified: '2026-07-16T09:18:02+08:00',
          server_running: true,
          reset_available: true,
        };
        break;
      case '/api/monitor/history':
        data = monitorHistory;
        break;
      case '/api/save/index/status':
        data = saveStatus;
        break;
      case '/api/save-sources':
        data = {
          items: [
            {
              id: 'server',
              name: '当前服务器存档',
              kind: 'server',
              active: true,
              fingerprint: 'sha256:6fd3818a',
              parser_version: 'palpanel-sav-cli-go',
              indexed_at: '2026-07-16T09:18:00+08:00',
              created_at: '2026-07-15T08:00:00+08:00',
              updated_at: '2026-07-16T09:18:00+08:00',
            },
            {
              id: 'save-local-01',
              name: '单人世界备份 · 樱岛',
              kind: 'import',
              active: false,
              fingerprint: 'sha256:0c294fd3',
              parser_version: 'palpanel-sav-cli-go',
              indexed_at: '2026-07-15T22:36:00+08:00',
              created_at: '2026-07-15T22:34:00+08:00',
              updated_at: '2026-07-15T22:36:00+08:00',
            },
          ],
          active_status: saveStatus,
        };
        break;
      case '/api/breeding/catalog':
        data = breedingCatalog;
        break;
      case '/api/breeding/custom-containers':
        data = [
          { id: 'perfect-iv', name: '满 IV 候选', pals: [{}, {}, {}, {}, {}] },
          { id: 'breeding-stock', name: '常用育种素材', pals: [{}, {}, {}, {}, {}, {}, {}, {}] },
        ];
        break;
      case '/api/breeding/presets':
        data = [
          { id: 'fast', name: '快速路线', config: { settings: { max_breeding_steps: 4, max_solver_iterations: 12 } } },
          { id: 'perfect', name: '毕业词条', config: { settings: { max_breeding_steps: 8, max_solver_iterations: 40 } } },
        ];
        break;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(envelope(data)),
    });
  });
};

const installBreedSessionBackend = async (page: Page) => {
  const principal = {
    subject: 'qq:10001',
    qq_id: '10001',
    player_uid: '4A2F0CBB000000000000000000000001',
    nickname: '帕鲁研究员',
    balance: 86,
  };
  await page.addInitScript((session) => {
    sessionStorage.setItem('palpanel_breed_principal', JSON.stringify(session));
  }, principal);
  await page.route((url) => url.pathname.startsWith('/api/'), async (route) => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = {};
    switch (path) {
      case '/api/auth/status':
        data = { initialized: true, authenticated: false };
        break;
      case '/api/breed/me':
        data = principal;
        break;
      case '/api/breed/catalog':
        data = breedingCatalog;
        break;
      case '/api/breed/custom-containers':
        data = [{ id: 'qq-stock', name: '我的配种素材', pals: [{}, {}, {}, {}, {}, {}] }];
        break;
      case '/api/breed/presets':
        data = [{ id: 'qq-fast', name: '每日快捷计算', config: { settings: { max_breeding_steps: 5 } } }];
        break;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope(data)) });
  });
};

const waitForFontsAndMotion = async (page: Page) => {
  await page.evaluate(async () => {
    await document.fonts.ready;
  });
  await page.waitForTimeout(250);
};

test.describe('README screenshots', () => {
  test.skip(process.env.UPDATE_README_SCREENSHOTS !== '1', 'Set UPDATE_README_SCREENSHOTS=1 to regenerate documentation images.');

  test('captures the current desktop client', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1024 });
    await installAdminBackend(page);

    await page.goto('/dashboard');
    await expect(page.getByText('在线人数与系统负载')).toBeVisible();
    await expect(page.getByText('6 / 32')).toBeVisible();
    await waitForFontsAndMotion(page);
    await page.screenshot({ path: '../docs/images/dashboard-new.png', animations: 'disabled' });

    await page.goto('/save-sources');
    await expect(page.getByRole('heading', { name: '存档中心' })).toBeVisible();
    await expect(page.getByText('单人世界备份 · 樱岛')).toBeVisible();
    await waitForFontsAndMotion(page);
    await page.screenshot({ path: '../docs/images/save-sources-new.png', animations: 'disabled' });

    await page.goto('/breeding');
    await expect(page.getByRole('heading', { name: '配种实验室' })).toBeVisible();
    await expect(page.getByRole('option', { name: /阿努比斯/ })).toHaveCount(1);
    await page.getByLabel('目标帕鲁').selectOption('Anubis');
    await page.locator('.passive-row').filter({ hasText: '传说' }).getByRole('button', { name: '必需' }).click();
    await waitForFontsAndMotion(page);
    await page.screenshot({ path: '../docs/images/breeding-lab-new.png', animations: 'disabled' });
  });

  test('captures the QQ breeding client on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await installBreedSessionBackend(page);
    await page.goto('/breeding');
    await expect(page.getByRole('heading', { name: '配种实验室' })).toBeVisible();
    await expect(page.getByText('86 积分')).toBeVisible();
    await page.getByLabel('目标帕鲁').selectOption('Jetragon');
    await waitForFontsAndMotion(page);
    await page.screenshot({ path: '../docs/images/breeding-mobile-new.png', animations: 'disabled' });
  });
});
