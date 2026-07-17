import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { ServerStoreProvider } from '../store/ServerStoreProvider';
import { TaskQueue } from './TaskQueue';

vi.mock('../api/tasks', () => ({
  tasksApi: {
    getJobs: vi.fn().mockResolvedValue(null),
    createBackupJob: vi.fn(),
    createUpdateJob: vi.fn(),
  },
}));

vi.mock('../api/schedules', () => ({
  schedulesApi: {
    list: vi.fn().mockResolvedValue([]),
    alerts: vi.fn().mockResolvedValue([]),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    run: vi.fn(),
    ackAlert: vi.fn(),
  },
}));

describe('TaskQueue', () => {
  it('renders the empty state when the jobs endpoint returns null', async () => {
    render(
      <MemoryRouter>
        <ServerStoreProvider>
          <TaskQueue />
        </ServerStoreProvider>
      </MemoryRouter>,
    );

    await waitFor(() => expect(screen.getByText('最近任务运行记录')).toBeInTheDocument());
    expect(screen.getAllByText('暂无任务').length).toBeGreaterThan(0);
  });

  it('opens the schedule editor directly for the backup policy link', async () => {
    render(
      <MemoryRouter initialEntries={['/tasks?tab=schedules']}>
        <ServerStoreProvider>
          <TaskQueue />
        </ServerStoreProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByText(/选择“定时安全重启”/)).toBeInTheDocument();
    expect(screen.getByRole('option', { name: '定时安全重启' })).toBeInTheDocument();
  });
});
