import { render, screen, waitFor } from '@testing-library/react';
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

describe('TaskQueue', () => {
  it('renders the empty state when the jobs endpoint returns null', async () => {
    render(
      <ServerStoreProvider>
        <TaskQueue />
      </ServerStoreProvider>,
    );

    await waitFor(() => expect(screen.getByText('最近任务运行记录')).toBeInTheDocument());
    expect(screen.getAllByText('暂无任务').length).toBeGreaterThan(0);
  });
});
