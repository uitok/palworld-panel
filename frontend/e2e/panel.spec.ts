import { expect, test, type Page } from '@playwright/test';

type Role = 'admin' | 'viewer';

const envelope = (data: unknown) => ({ ok: true, data });

const sessionForRole = (role: Role) => ({
  name: role,
  role,
  permissions: role === 'admin'
    ? ['read', 'server:control', 'config:write', 'backup:write', 'mods:write', 'players:write', 'security:write', 'audit:read', 'world:reset', 'ai:config']
    : ['read'],
});

const installFakeBackend = async (page: Page, role: Role = 'admin', initiallyAuthenticated = true) => {
  let authorization = '';
  let loginBody: unknown;
  let authenticated = initiallyAuthenticated;
  await page.route((url) => url.pathname.startsWith('/api/'), async (route) => {
    const request = route.request();
    if (request.headers().authorization) authorization = request.headers().authorization;
    const path = new URL(request.url()).pathname;
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
      case '/api/jobs':
        data = [{ id: 'job-1', type: 'backup', status: 'completed', progress: 100, message: 'backup completed', created_at: '2026-07-14T00:00:00Z', updated_at: '2026-07-14T00:01:00Z' }];
        break;
      case '/api/schedules':
        data = [{ id: 'schedule-1', type: 'backup', enabled: true, interval_minutes: 60, timezone: 'UTC', next_run_at: '2026-07-14T01:00:00Z', created_at: '2026-07-14T00:00:00Z', updated_at: '2026-07-14T00:00:00Z' }];
        break;
      case '/api/alerts':
        data = [];
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
    }
    if (!authenticated && path !== '/api/auth/status' && path !== '/api/auth/login') {
      status = 401;
    }
    await route.fulfill({ status, headers, contentType: 'application/json', body: JSON.stringify(envelope(data)) });
  });
  return {
    authorization: () => authorization,
    loginBody: () => loginBody,
  };
};

test('logs in with a server session cookie and sends no legacy bearer auth', async ({ page }) => {
  const backend = await installFakeBackend(page, 'admin', false);
  await page.goto('/dashboard');
  await page.getByLabel('用户名').fill('admin');
  await page.getByLabel('密码').fill('strong-password-123');
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
