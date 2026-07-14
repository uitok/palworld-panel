import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AppContent } from './App';
import { ServerStoreProvider } from './store/ServerStoreProvider';

const authMocks = vi.hoisted(() => ({
  status: vi.fn(),
  register: vi.fn(),
  login: vi.fn(),
	logout: vi.fn(),
}));

vi.mock('./api/auth', () => ({ authApi: authMocks }));

vi.mock('./pages/Settings', () => ({ Settings: () => <h1>设置路由内容</h1> }));
vi.mock('./pages/Dashboard', () => ({ Dashboard: () => <h1>总览路由内容</h1> }));

const renderRoute = (path: string) => {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <ServerStoreProvider>
        <AppContent />
      </ServerStoreProvider>
    </MemoryRouter>,
  );
};

describe('app routing', () => {
	afterEach(() => cleanup());

  beforeEach(() => {
	vi.clearAllMocks();
	authMocks.status.mockResolvedValue({
	  initialized: true,
	  authenticated: true,
	  user: { name: 'admin', role: 'admin', permissions: ['read'] },
	});
	authMocks.logout.mockResolvedValue({ logged_out: true });
  });

  it('renders a configured route directly', async () => {
    renderRoute('/settings');
    expect(await screen.findByText('设置路由内容')).toBeInTheDocument();
  });

  it('redirects unknown routes to the dashboard', async () => {
    renderRoute('/does-not-exist');
    expect(await screen.findByText('总览路由内容')).toBeInTheDocument();
  });

  it('registers the first administrator from the browser', async () => {
	authMocks.status.mockResolvedValue({ initialized: false, authenticated: false });
	authMocks.register.mockResolvedValue({ name: 'admin', role: 'admin', permissions: ['read'] });
    renderRoute('/dashboard');

	fireEvent.change(await screen.findByLabelText('用户名'), { target: { value: 'admin' } });
	fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'strong-password-123' } });
	fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'strong-password-123' } });
	fireEvent.click(screen.getByRole('button', { name: '完成注册' }));

	await waitFor(() => expect(authMocks.register).toHaveBeenCalledWith('admin', 'strong-password-123'));
	expect(await screen.findByText('总览路由内容')).toBeInTheDocument();
  });

  it('logs in when authentication is already initialized', async () => {
	authMocks.status.mockResolvedValue({ initialized: true, authenticated: false });
	authMocks.login.mockResolvedValue({ name: 'admin', role: 'admin', permissions: ['read'] });
	renderRoute('/settings');

	fireEvent.change(await screen.findByLabelText('用户名'), { target: { value: 'admin' } });
	fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'strong-password-123' } });
	fireEvent.click(screen.getByRole('button', { name: '登录' }));

	await waitFor(() => expect(authMocks.login).toHaveBeenCalledWith('admin', 'strong-password-123'));
	expect(await screen.findByText('设置路由内容')).toBeInTheDocument();
  });

  it('retries authentication after the service becomes available', async () => {
	authMocks.status.mockRejectedValueOnce(new Error('offline'));
	authMocks.status.mockResolvedValueOnce({
	  initialized: true,
	  authenticated: true,
	  user: { name: 'admin', role: 'admin', permissions: ['read'] },
	});
	renderRoute('/dashboard');

	fireEvent.click(await screen.findByRole('button', { name: '重新连接' }));
	expect(await screen.findByText('总览路由内容')).toBeInTheDocument();
	expect(authMocks.status).toHaveBeenCalledTimes(2);
  });

  it('logs out and returns to the account gate', async () => {
	renderRoute('/dashboard');
	fireEvent.click((await screen.findAllByRole('button', { name: '退出登录' }))[0]);

	await waitFor(() => expect(authMocks.logout).toHaveBeenCalledTimes(1));
	expect(await screen.findByText('管理员登录')).toBeInTheDocument();
  });
});
