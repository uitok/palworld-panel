import { expect, test, type Page } from '@playwright/test';
import { createServer } from 'node:http';

type Role = 'admin' | 'viewer';

const envelope = (data: unknown) => ({ ok: true, data });

const sessionForRole = (role: Role) => ({
  name: role,
  role,
  permissions: role === 'admin'
    ? ['read', 'server:control', 'config:write', 'backup:write', 'mods:write', 'players:write', 'security:write', 'audit:read', 'world:reset', 'ai:config']
    : ['read'],
});

const installFakeBackend = async (page: Page, role: Role = 'admin', initiallyAuthenticated = true, locale = 'zh-CN') => {
  await page.addInitScript((initialLocale) => {
    if (!window.localStorage.getItem('palpanel.locale')) window.localStorage.setItem('palpanel.locale', initialLocale);
  }, locale);
  let authorization = '';
  let loginBody: unknown;
  let authenticated = initiallyAuthenticated;
  let backendUnavailable = false;
  await page.route((url) => url.pathname.startsWith('/api/'), async (route) => {
    const request = route.request();
    if (request.headers().authorization) authorization = request.headers().authorization;
    const path = new URL(request.url()).pathname;
    if (path.startsWith('/api/backups/') && path.endsWith('/download')) {
      await route.fulfill({
        status: 200,
        headers: {
          'content-type': 'application/zip',
          'content-disposition': 'attachment; filename="palpanel manual.zip"',
          'content-length': '24',
        },
        body: 'streamed-backup-contents',
      });
      return;
    }
    if (backendUnavailable && path !== '/api/auth/status') {
      await route.fulfill({ status: 502, contentType: 'text/plain', body: 'Bad Gateway' });
      return;
    }
    let data: unknown = {};
    let status = 200;
    const headers: Record<string, string> = {};
    switch (path) {
      case '/api/auth/status':
        data = {
          initialized: true,
          authenticated,
          user: authenticated ? sessionForRole(role) : undefined,
        };
        break;
      case '/api/auth/login':
        loginBody = request.postDataJSON();
        authenticated = true;
        data = sessionForRole(role);
        headers['set-cookie'] = 'palpanel_session=e2e-session; Path=/; HttpOnly; SameSite=Lax';
        break;
      case '/api/auth/me':
        data = sessionForRole(role);
        break;
      case '/api/server/status':
        data = {
          installed: true,
          pending_restart: false,
          runtime_mode: 'wine_docker',
          setup_step: 'ready',
          config_exists: true,
          container: { exists: false, status: 'missing' },
          startup_args: [],
          ports: { game: 8211, query: 27015, rest: 8212 },
          warnings: [],
          paths: { palworld_settings: '/data/PalWorldSettings.ini' },
        };
        break;
      case '/api/server/prerequisites':
        data = [{ id: 'windows', label: 'Windows host', ok: true, required: true }, { id: 'steamcmd', label: 'SteamCMD', ok: true, required: false }];
        break;
      case '/api/server/runtime':
        data = { mode: 'windows_steamcmd' };
        break;
      case '/api/server/host':
        data = {
          os: 'windows', arch: 'amd64', supported: true, systemd: false, recommended_runtime: 'windows_steamcmd',
          docker: { cli_installed: false, daemon_reachable: false },
          sudo: { is_root: false, sudo_installed: false, passwordless: false, can_elevate: false, needs_password: false },
          current_user_in_docker_group: false, warnings: [],
        };
        break;
      case '/api/server/startup':
        data = { startup: { port: 8211, players: 32, public_lobby: false, public_ip: '', public_port: 8211, log_format: 'text', use_perf_threads: true, no_async_loading_thread: true, use_multithread_for_ds: true, number_of_worker_threads_server: 0, workshop_dir: '', no_mods: false }, args: [], issues: [] };
        break;
      case '/api/server/version':
        data = { installed: true, current_build_id: '100', latest_build_id: '100', update_available: false, compatibility_warnings: [] };
        break;
      case '/api/server/logs':
        data = { logs: '', source: 'none', available: false, reason: 'not_started' };
        break;
      case '/api/server/world':
        data = { active_world_id: 'world', reset_available: true, running: false };
        break;
      case '/api/monitor/history':
        data = [];
        break;
      case '/api/monitor/snapshot':
        data = { sample: { id: 'sample-1', created_at: '2026-07-20T00:00:00Z', cpu_available: false, cpu_percent: 0, memory_available: false, memory_usage_bytes: 0, memory_limit_bytes: 0, disk_available: true, disk_free_bytes: 1, disk_total_bytes: 2, current_players: 0, max_players: 32, rest_healthy: false, rcon_healthy: false, game_port_healthy: false, query_port_healthy: false } };
        break;
      case '/api/jobs':
        data = [{ id: 'job-1', type: 'backup', status: 'completed', progress: 100, message: 'backup completed', created_at: '2026-07-14T00:00:00Z', updated_at: '2026-07-14T00:01:00Z' }];
        break;
      case '/api/schedules':
        data = [{ id: 'schedule-1', type: 'backup', enabled: true, interval_minutes: 60, timezone: 'UTC', next_run_at: '2026-07-14T01:00:00Z', created_at: '2026-07-14T00:00:00Z', updated_at: '2026-07-14T00:00:00Z' }];
        break;
      case '/api/alerts':
        data = [];
        break;
      case '/api/backups':
        data = [{ name: 'palpanel manual.zip', path: '/data/backups/palpanel manual.zip', size_bytes: 24, created_at: '2026-07-20T00:00:00Z', reason: 'manual', status: 'available' }];
        break;
      case '/api/backups/webdav/config':
        data = { enabled: false, base_url: '', username: '', remote_path: 'PalPanel', upload_after_backup: false, password_configured: false };
        break;
      case '/api/config/palworld/schema':
        data = { version: '1.0.0', fields: [{ key: 'ServerName', label: '服务器名称', group: 'server_management', type: 'string', default: 'Palworld Server', description: '' }] };
        break;
      case '/api/config/palworld':
        data = { path: '/data/PalWorldSettings.ini', settings: { ServerName: 'E2E Server' }, pending_restart: false, issues: [] };
        break;
      case '/api/ai/translation/config':
        data = { enabled: false, base_url: '', model: '', api_key_configured: false };
        break;
      case '/api/community-servers':
        data = {
          servers: [{ id: 'cn-e2e', name: '国内社区服', address: '203.0.113.8', port: 8211, connect: '203.0.113.8:8211', players: 18, max_players: 32, password: false, country: 'CN', version: '1.0', status: 'online' }],
          total: 1, source_total: 1, page: 1, page_size: 30, source: 'battlemetrics', fetched_at: '2026-07-18T08:00:00Z', stale: true, cache_age_seconds: 90,
        };
        break;
      case '/api/community-servers/source-status':
        data = { source: 'battlemetrics', enabled: true, base_url: 'https://api.battlemetrics.com', proxy_configured: true, reachable: false, cache_available: true, cache_fresh: false, cache_writable: true, cached_queries: 1, rate_limit_per_minute: 30 };
        break;
      case '/api/mods':
        data = [];
        break;
      case '/api/mods/workshop/auth/status':
        data = { supported: true, steamcmd_installed: true, credentials_secure: false, login_in_progress: false, logged_in: false, verification_required: true, password_configured: false, steam_guard_required: false, message: 'Enter the Steam account name and password used for Workshop downloads.' };
        break;
    }
    if (!authenticated && path !== '/api/auth/status' && path !== '/api/auth/login') {
      status = 401;
    }
    await route.fulfill({ status, headers, contentType: 'application/json', body: JSON.stringify(envelope(data)) });
  });
  return {
    authorization: () => authorization,
    loginBody: () => loginBody,
    setBackendUnavailable: (unavailable: boolean) => { backendUnavailable = unavailable; },
  };
};

test('logs in with a server session cookie and sends no legacy bearer auth', async ({ page }) => {
  const backend = await installFakeBackend(page, 'admin', false);
  await page.goto('/dashboard');
  const username = page.getByLabel('用户名');
  const password = page.getByLabel('密码');
  await expect(username).toHaveCSS('padding-left', '36px');
  await expect(password).toHaveCSS('padding-left', '36px');
  await username.fill('admin');
  await password.fill('strong-password-123');
  await page.getByRole('button', { name: '登录' }).click();
  await expect(page.getByRole('heading', { name: '服务器总览' })).toBeVisible();
  expect(backend.loginBody()).toEqual({ username: 'admin', password: 'strong-password-123' });
  expect(backend.authorization()).toBe('');
  await expect.poll(async () => {
    const cookie = (await page.context().cookies()).find((item) => item.name === 'palpanel_session');
    return cookie?.value;
  }).toBe('e2e-session');
});

test('viewer session cannot see the world reset command', async ({ page }) => {
  await installFakeBackend(page, 'viewer');
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: '服务器总览' })).toBeVisible();
  await expect(page.getByRole('button', { name: '重置世界' })).toHaveCount(0);
});

test('pages remain renderable while the backend temporarily returns 502', async ({ page }) => {
  const backend = await installFakeBackend(page);
  const pageErrors: string[] = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));
  backend.setBackendUnavailable(true);

  for (const path of ['/setup', '/dashboard', '/tasks', '/settings', '/mods', '/backups']) {
    await page.goto(path);
    await page.waitForLoadState('networkidle');
    await expect(page.getByText('页面加载失败')).toHaveCount(0);
  }
  expect(pageErrors).toEqual([]);
});

test('desktop shell follows the main branch layout without horizontal overflow', async ({ page }) => {
  test.setTimeout(120_000);
  await installFakeBackend(page);
  const routes = ['/setup', '/dashboard', '/player-center', '/backups'] as const;
  for (const width of [757, 1024, 1440, 1864]) {
    await page.setViewportSize({ width, height: 900 });
    for (const path of routes) {
      await page.goto(path);
      await page.locator('#app-main > div').waitFor({ state: 'attached' });
      const layout = await page.evaluate(() => {
        const header = document.querySelector('.pp-topbar__inner')?.getBoundingClientRect();
        const content = document.querySelector('#app-main > div')?.getBoundingClientRect();
        const shell = document.querySelector('.pp-shell__content')?.getBoundingClientRect();
        const rail = document.querySelector('.pp-rail')?.getBoundingClientRect();
        return {
          header: header && { x: header.x, width: header.width },
          content: content && { x: content.x, width: content.width },
          shell: shell && { x: shell.x, width: shell.width },
          rail: rail && { right: rail.right },
          overflow: document.documentElement.scrollWidth - window.innerWidth,
        };
      });
      expect(layout.header).not.toBeNull();
      expect(layout.content).not.toBeNull();
      expect(layout.shell).not.toBeNull();
      if (width >= 1024) {
        expect(layout.rail).not.toBeNull();
        expect(Math.abs((layout.rail?.right ?? 0) - (layout.shell?.x ?? 0))).toBeLessThanOrEqual(1);
      }
      expect((layout.header?.width ?? 0)).toBeLessThanOrEqual((layout.shell?.width ?? 0) + 1);
      expect((layout.content?.width ?? 0)).toBeLessThanOrEqual((layout.shell?.width ?? 0) + 1);
      expect(layout.overflow).toBeLessThanOrEqual(1);
    }
  }
});

test('server center keeps the main branch grouping and community servers stay available', async ({ page }) => {
  await installFakeBackend(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/dashboard');

  const navigation = page.getByRole('navigation', { name: '主导航' });
  const serverCenter = navigation.getByRole('button', { name: '服务器中心' });
  await expect(serverCenter).toHaveAttribute('aria-expanded', 'true');
  const serverSubmenu = serverCenter.locator('xpath=following-sibling::*[1]');
  await expect(serverSubmenu.getByRole('link')).toHaveCount(2);
  await expect(serverSubmenu.getByRole('link', { name: '服务器总览' })).toBeVisible();
  await expect(serverSubmenu.getByRole('link', { name: '实时监控' })).toBeVisible();
  await expect(serverSubmenu.getByRole('link', { name: '社区服务器' })).toHaveCount(0);
  await expect(navigation.locator('a.pp-nav__item[href="/community-servers"]')).toBeVisible();
});

test('header navigation button stays visible and toggles the desktop sidebar', async ({ page }) => {
  await installFakeBackend(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/dashboard');

  const toggle = page.getByRole('button', { name: '切换导航栏' });
  const rail = page.locator('.pp-rail').first();
  await expect(toggle).toBeVisible();
  await expect(rail).toHaveCSS('width', '236px');
  await toggle.click();
  await expect(rail).toHaveCSS('width', '72px');
  await toggle.click();
  await expect(rail).toHaveCSS('width', '236px');
});

test('backup download is handled as a browser attachment without navigation', async ({ page }) => {
  const payload = Buffer.from('streamed-backup-contents');
  const downloadServer = createServer((request, response) => {
    if (request.url?.startsWith('/api/backups/') && request.url.endsWith('/download')) {
      response.writeHead(200, {
        'content-type': 'application/zip',
        'content-disposition': 'attachment; filename="palpanel manual.zip"',
        'content-length': payload.length,
      });
      response.end(payload);
      return;
    }
    response.writeHead(404).end();
  });
  await new Promise<void>((resolve, reject) => {
    downloadServer.once('error', reject);
    downloadServer.listen(64217, '127.0.0.1', resolve);
  });
  try {
    await installFakeBackend(page);
    await page.goto('/backups');
    const before = page.url();
    const downloadPromise = page.waitForEvent('download');
    await page.locator('a[title="下载"]:visible').first().click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe('palpanel manual.zip');
    expect(await download.createReadStream().then(async (stream) => {
      const chunks: Buffer[] = [];
      for await (const chunk of stream) chunks.push(Buffer.from(chunk));
      return Buffer.concat(chunks).toString('utf8');
    })).toBe('streamed-backup-contents');
    expect(page.url()).toBe(before);
  } finally {
    await new Promise<void>((resolve) => downloadServer.close(() => resolve()));
  }
});

test('Workshop credential dialog uses the full viewport instead of the transformed page container', async ({ page }) => {
  await installFakeBackend(page);
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/mods');

  await page.getByRole('button', { name: '配置 Steam 登录' }).click();

  const dialog = page.getByRole('dialog', { name: '登录 Steam 以使用 Workshop' });
  await expect(dialog).toBeVisible();
  const box = await dialog.boundingBox();
  expect(box).not.toBeNull();
  expect(box?.x).toBe(0);
  expect(box?.y).toBe(0);
  expect(box?.width).toBe(1280);
  expect(box?.height).toBe(720);
  await expect(page.locator('body')).toHaveCSS('overflow', 'hidden');
  await expect(dialog.getByText('请输入用于 Workshop 下载的 Steam 账户名和密码。')).toBeVisible();
  await expect(dialog.getByLabel('Steam 密码')).toBeVisible();
});

test('renders task and schedule data from the API contract', async ({ page }) => {
  await installFakeBackend(page);
  await page.goto('/tasks');
  await expect(page.getByRole('cell', { name: 'backup completed', exact: true })).toBeVisible();
  await page.getByRole('button', { name: /计划任务/ }).click();
  await expect(page.getByRole('cell', { name: '每 60 分钟', exact: true })).toBeVisible();
});

test('loads schema-backed server settings', async ({ page }) => {
  await installFakeBackend(page);
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: '系统设置' })).toBeVisible();
  await expect(page.getByLabel('服务器名称')).toHaveValue('E2E Server');
});

test('selects and persists the interface language', async ({ page }) => {
  await installFakeBackend(page, 'admin', true, 'en-US');
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'System settings' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en-US');

  await page.getByRole('button', { name: '简体中文' }).click();
  await expect(page.getByRole('heading', { name: '系统设置' })).toBeVisible();
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN');
  await expect(page.getByRole('heading', { name: '系统设置' })).toBeVisible();
});

test('mobile navigation stays in view and closes from the backdrop', async ({ page }) => {
  await installFakeBackend(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/dashboard');

  await page.locator('.pp-topbar button').first().click();
  const drawer = page.locator('.pp-rail.is-mobile');
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveCSS('transform', 'none');
  expect((await drawer.boundingBox())?.x).toBe(0);

  await page.mouse.click(370, 400);
  await expect(drawer).toHaveCount(0);
});

test('setup wizard stays inside a medium-width viewport', async ({ page }) => {
  await installFakeBackend(page);
  for (const width of [757, 1024, 1100]) {
    await page.setViewportSize({ width, height: 760 });
    await page.goto('/setup');
    await expect(page.getByRole('heading', { name: '一键开服' })).toBeVisible();
    expect(await page.evaluate(() => ({ document: document.documentElement.scrollWidth, body: document.body.scrollWidth, viewport: window.innerWidth }))).toEqual({ document: width, body: width, viewport: width });
    const hero = page.getByRole('heading', { name: '一键开服' }).locator('xpath=ancestor::section[1]');
    const box = await hero.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    for (const button of await hero.getByRole('button').all()) {
      const buttonBox = await button.boundingBox();
      expect(buttonBox).not.toBeNull();
      expect(buttonBox!.x).toBeGreaterThanOrEqual(0);
      expect(buttonBox!.x + buttonBox!.width).toBeLessThanOrEqual(width);
    }
    if (width < 1280) {
      const headingBox = await page.getByRole('heading', { name: '一键开服' }).boundingBox();
      const primaryBox = await hero.getByRole('button').first().boundingBox();
      expect(primaryBox!.y).toBeGreaterThan(headingBox!.y + headingBox!.height);
    }
  }
});

test('community server discovery remains usable on a mobile viewport with stale data', async ({ page }) => {
  await installFakeBackend(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/community-servers');

  await expect(page.getByRole('heading', { name: '社区服务器' })).toBeVisible();
  await expect(page.getByText('国内社区服')).toBeVisible();
  await expect(page.getByText(/正在显示 90 秒前的缓存/)).toBeVisible();
  await page.getByRole('button', { name: /国内社区服/ }).click();
  await expect(page.getByRole('button', { name: /复制 203.0.113.8:8211/ })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});
