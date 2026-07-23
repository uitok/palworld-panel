import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BreedingLab } from './BreedingLab';

const mocks = vi.hoisted(() => ({
  catalog: vi.fn(), customContainers: vi.fn(), presets: vi.fn(), status: vi.fn(),
  submit: vi.fn(), waitForJob: vi.fn(), result: vi.fn(), savePreset: vi.fn(),
  pause: vi.fn(), resume: vi.fn(), cancel: vi.fn(), getSaveStatus: vi.fn(),
}));

vi.mock('../api/breeding', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api/breeding')>();
  return {
    ...original,
    breedingApi: {
      catalog: mocks.catalog,
      customContainers: mocks.customContainers,
      presets: mocks.presets,
      status: mocks.status,
      submit: mocks.submit,
      waitForJob: mocks.waitForJob,
      result: mocks.result,
      savePreset: mocks.savePreset,
      pause: mocks.pause,
      resume: mocks.resume,
      cancel: mocks.cancel,
    },
  };
});

vi.mock('../api/saveIndex', () => ({
  saveIndexApi: { getStatus: mocks.getSaveStatus },
}));

const renderLab = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BreedingLab access="session" principal={{ subject: 'qq:1', qq_id: '1', player_uid: 'uid-1', nickname: '测试玩家', balance: 5 }} />
    </QueryClientProvider>,
  );
};

describe('BreedingLab localization', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.catalog.mockResolvedValue({
      version: '2026.7',
      pals: [{ id: 'Anubis', name: '阿努比斯', raw_name: 'Anubis' }, { id: 'PinkCat', name: '捣蛋猫', raw_name: 'Cattiva' }],
      passives: [{ id: 'CraftSpeed_up3', name: '卓绝技艺', raw_name: 'Artisan', supports_surgery: true, surgery_cost: 100 }],
      active_skills: [],
    });
    mocks.customContainers.mockResolvedValue([]);
    mocks.presets.mockResolvedValue([]);
    mocks.submit.mockResolvedValue({ job: { id: 'job-1', status: 'queued', progress: 0 }, balance: 4 });
    mocks.waitForJob.mockImplementation(async (_jobId: string, _access: string, onUpdate?: (job: Record<string, unknown>) => void) => {
      const completed = { id: 'job-1', status: 'success', progress: 100, message: 'completed' };
      onUpdate?.(completed);
      return completed;
    });
    mocks.result.mockResolvedValue({
      job_id: 'job-1', status: 'completed', stale: false,
      result: {
        save_fingerprint: 'save-1',
        results: [{
          pal_id: 'Anubis', pal_name: '阿努比斯', gender: 'male', passives: ['卓绝技艺'], effort_seconds: 600,
          breeding_steps: 0, eggs: 0, wild_pals: 0, gold_cost: 0,
          tree: { type: 'owned', pal_id: 'Anubis', pal_name: '阿努比斯', gender: 'male', passives: ['卓绝技艺'], effort_seconds: 0, self_effort_seconds: 0, cost: 0, location_type: 'palbox' },
        }],
      },
    });
  });

  afterEach(() => cleanup());

  it('shows localized pals, passives, statuses, genders, and locations', async () => {
    renderLab();

    expect(await screen.findByRole('option', { name: '阿努比斯 · Anubis' })).toBeInTheDocument();
    const palSelect = screen.getByLabelText('目标帕鲁');
    expect(await screen.findByText('卓绝技艺')).toBeInTheDocument();
    expect(screen.getByText('空闲')).toBeInTheDocument();

    fireEvent.change(palSelect, { target: { value: 'Anubis' } });
    fireEvent.click(screen.getByRole('button', { name: '必需' }));
    fireEvent.click(screen.getByRole('button', { name: '开始计算（1 积分）' }));

    await waitFor(() => expect(mocks.submit).toHaveBeenCalled());
    expect((await screen.findAllByText('阿努比斯')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('卓绝技艺').length).toBeGreaterThan(0);
    expect(screen.getByText('雄性 · 帕鲁终端')).toBeInTheDocument();
    expect(screen.getAllByText('已完成').length).toBeGreaterThan(0);
  });
});
