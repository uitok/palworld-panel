import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { QueryProvider } from '../queryClient';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { Dashboard } from './Dashboard';

const mocks = vi.hoisted(() => ({
  authApi: { status: vi.fn(), me: vi.fn() },
  serverApi: {
    getStatus: vi.fn(), getMetrics: vi.fn(), getLogs: vi.fn(), getWorld: vi.fn(), resetWorld: vi.fn(),
    start: vi.fn(), stop: vi.fn(), forceStop: vi.fn(),
  },
  monitorApi: { history: vi.fn() },
  tasksApi: { waitForJob: vi.fn() },
}));

vi.mock('../api/auth', () => ({ authApi: mocks.authApi }));
vi.mock('../api/server', () => ({ serverApi: mocks.serverApi }));
vi.mock('../api/monitor', () => ({ monitorApi: mocks.monitorApi }));
vi.mock('../api/tasks', () => ({ tasksApi: mocks.tasksApi }));

describe('Dashboard world reset and stopped logs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
	mocks.authApi.status.mockResolvedValue({ initialized: true, authenticated: true, user: { name: 'admin', role: 'admin', permissions: ['read', 'world:reset'] } });
    mocks.authApi.me.mockResolvedValue({ name: 'admin', role: 'admin', permissions: ['read', 'world:reset'] });
    mocks.serverApi.getStatus.mockResolvedValue({
      status: 'stopped', installed: true, pending_restart: false, runtime_mode: 'wine_docker', setup_step: 'configured',
      config_exists: true, container: { exists: true, status: 'exited' }, startup_args: [], ports: { game: 8211, rest: 8212 },
      warnings: [], paths: {},
    });
    mocks.serverApi.getMetrics.mockResolvedValue({});
    mocks.serverApi.getLogs.mockResolvedValue({ logs: 'last persisted line', source: 'paldefender-game', available: true });
    mocks.serverApi.getWorld.mockResolvedValue({
      active_world_id: 'ABC123', save_exists: true, last_modified: '2026-07-10T00:00:00Z', server_running: false, reset_available: true,
    });
    mocks.serverApi.resetWorld.mockResolvedValue({ id: 'job_reset', type: 'world_reset', status: 'waiting', progress: 0, created_at: '2026-07-10T00:00:00Z' });
    mocks.monitorApi.history.mockResolvedValue([]);
    mocks.tasksApi.waitForJob.mockResolvedValue({ id: 'job_reset', type: 'world_reset', status: 'success', progress: 100, message: 'done', created_at: '2026-07-10T00:00:00Z' });
  });

  it('reads logs while stopped and requires the exact reset phrase', async () => {
    render(
      <QueryProvider>
        <ServerStoreProvider><Dashboard /></ServerStoreProvider>
      </QueryProvider>,
    );

    expect(await screen.findByText('last persisted line')).toBeInTheDocument();
	expect(screen.getAllByText('游戏事件').length).toBeGreaterThan(0);
	await waitFor(() => expect(mocks.serverApi.getLogs).toHaveBeenCalledWith(80, '', '', '', 'game'));
	fireEvent.change(screen.getByLabelText('日志来源'), { target: { value: 'launcher' } });
	await waitFor(() => expect(mocks.serverApi.getLogs).toHaveBeenCalledWith(80, '', '', '', 'launcher'));
    fireEvent.click(await screen.findByRole('button', { name: '重置世界' }));
    expect((await screen.findAllByText('ABC123')).length).toBeGreaterThan(0);
    const execute = screen.getByRole('button', { name: '执行重置' });
    expect(execute).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/RESET WORLD/), { target: { value: 'RESET WORLD' } });
    expect(execute).toBeEnabled();
    fireEvent.click(execute);

    await waitFor(() => expect(mocks.serverApi.resetWorld).toHaveBeenCalledWith('ABC123', 'RESET WORLD'));
    expect(mocks.tasksApi.waitForJob).toHaveBeenCalledWith('job_reset', expect.any(Function));
  });
});
