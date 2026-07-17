import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Backups } from './Backups';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  getWebDAVConfig: vi.fn(),
  testWebDAVConfig: vi.fn(),
  updateWebDAVConfig: vi.fn(),
}));

vi.mock('../api/backups', () => ({
  backupsApi: {
    list: mocks.list,
    getWebDAVConfig: mocks.getWebDAVConfig,
    testWebDAVConfig: mocks.testWebDAVConfig,
    updateWebDAVConfig: mocks.updateWebDAVConfig,
    create: vi.fn(),
    restore: vi.fn(),
    verify: vi.fn(),
    delete: vi.fn(),
    download: vi.fn(),
    uploadWebDAV: vi.fn(),
  },
}));

vi.mock('../api/tasks', () => ({
  tasksApi: { waitForJob: vi.fn() },
}));

describe('Backups WebDAV policy', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.list.mockResolvedValue([]);
    mocks.getWebDAVConfig.mockResolvedValue({
      enabled: false,
      base_url: 'https://dav.example.com/root',
      username: 'panel',
      remote_path: 'PalPanel/server-01',
      upload_after_backup: false,
      password_configured: true,
    });
    mocks.testWebDAVConfig.mockResolvedValue({ connected: true });
  });

  it('renders WebDAV settings and links directly to scheduled tasks', async () => {
    render(
      <MemoryRouter>
        <Backups />
      </MemoryRouter>,
    );

    expect(await screen.findByDisplayValue('https://dav.example.com/root')).toBeInTheDocument();
    expect(screen.getByDisplayValue('PalPanel/server-01')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '设置定时备份与重启' })).toHaveAttribute('href', '/tasks?tab=schedules');
    fireEvent.click(screen.getByRole('button', { name: '连接测试' }));
    expect(mocks.testWebDAVConfig).toHaveBeenCalledWith(expect.objectContaining({ base_url: 'https://dav.example.com/root' }));
  });
});
