import { expect, test, type Page } from '@playwright/test';

const envelope = (data: unknown) => ({ ok: true, data });

const installBreedSession = async (page: Page) => {
  await page.addInitScript(() => {
    sessionStorage.setItem('palpanel_breed_principal', JSON.stringify({
      subject: 'qq:10001', qq_id: '10001', player_uid: 'uid-1', nickname: '测试玩家', balance: 19,
    }));
  });
  await page.route('**/api/auth/status', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ initialized: true, authenticated: false })) });
  });
  await page.route((url) => url.pathname.startsWith('/api/breed/'), async (route) => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = {};
    switch (path) {
      case '/api/breed/me':
        data = { subject: 'qq:10001', qq_id: '10001', player_uid: 'uid-1', nickname: '测试玩家', balance: 19 };
        break;
      case '/api/breed/catalog':
        data = {
          version: 'v26',
          pals: [{ id: 'Anubis', name: '阿努比斯' }, { id: 'Lamball', name: '棉悠悠' }],
          passives: [{ id: 'Legend', name: '传说', supports_surgery: true, surgery_cost: 1000 }],
          active_skills: [],
        };
        break;
      case '/api/breed/custom-containers':
        data = [{ id: 'custom-1', name: '育种候选', pals: [{}, {}] }];
        break;
      case '/api/breed/presets':
        data = [{ id: 'preset-1', name: '速通预设', config: { settings: { max_breeding_steps: 4 } } }];
        break;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope(data)) });
  });
};

test('QQ breeding session hides administrator-only sources and shows points', async ({ page }) => {
  await installBreedSession(page);
  await page.goto('/breeding');
  await expect(page.getByRole('heading', { name: '配种实验室' })).toBeVisible();
  await expect(page.getByText('测试玩家').first()).toBeVisible();
  await expect(page.getByText('19 积分').first()).toBeVisible();
  await expect(page.getByLabel('限定玩家 UID')).toHaveCount(0);
  await expect(page.getByRole('option', { name: /阿努比斯/ })).toHaveCount(1);
});

test('breeding workspace remains within a mobile viewport', async ({ page }) => {
  await installBreedSession(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/breeding');
  await expect(page.getByRole('heading', { name: '配种实验室' })).toBeVisible();
  const dimensions = await page.evaluate(() => ({ width: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.width);
  await expect(page.getByText('高级设置', { exact: true })).toBeVisible();
});
