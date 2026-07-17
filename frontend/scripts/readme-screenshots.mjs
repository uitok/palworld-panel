import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const frontendDir = fileURLToPath(new URL('..', import.meta.url));
const playwrightCli = fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url));
const result = spawnSync(
  process.execPath,
  [playwrightCli, 'test', 'e2e/readme-screenshots.spec.ts', '--project=chromium', '--workers=1'],
  {
    cwd: frontendDir,
    env: { ...process.env, UPDATE_README_SCREENSHOTS: '1' },
    stdio: 'inherit',
  },
);

if (result.error) throw result.error;
process.exit(result.status ?? 1);
