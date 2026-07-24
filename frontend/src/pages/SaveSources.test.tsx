import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SaveSources } from './SaveSources';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  inspectArchive: vi.fn(),
  selectImportCandidate: vi.fn(),
  importInspected: vi.fn(),
  activate: vi.fn(),
  rebuild: vi.fn(),
  rename: vi.fn(),
  remove: vi.fn(),
  migrationPlayers: vi.fn(),
  previewMigration: vi.fn(),
  startMigration: vi.fn(),
}));

vi.mock('../api/saveSources', () => ({ saveSourcesApi: mocks }));

const renderSaveSources = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<QueryClientProvider client={client}><SaveSources /></QueryClientProvider>);
};

const status = {
  state: 'ready', parser: 'test', warnings: [], error: '', updated_at: '',
  counts: { players: 0, guilds: 0, bases: 0, pals: 0, containers: 0, map_entities: 0 },
};

describe('SaveSources archive inspection', () => {
  afterEach(() => cleanup());

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.list.mockResolvedValue({ items: [], active_status: status });
    mocks.selectImportCandidate.mockImplementation(async (inspectionID: string, candidateID: string) => ({
      id: inspectionID, file_name: 'world.zip', candidates: [], selected_candidate_id: candidateID,
      requires_selection: false, expires_at: '2026-07-22T13:00:00Z',
    }));
    mocks.importInspected.mockResolvedValue({ id: 'save-1', name: 'Imported', kind: 'import' });
    mocks.migrationPlayers.mockResolvedValue({ source: { id: 'import-one', name: '旧世界', kind: 'import' }, players: [{ player_uid: '25527209-0000-0000-0000-000000000000', nickname: '旧玩家', level: 55 }] });
    mocks.previewMigration.mockResolvedValue({ source: { id: 'import-one', name: '旧世界', kind: 'import' }, target_mode: 'unknown', mode_source: 'unproven', requires_manual_confirmation: true, mappings: [{ source_uid: '25527209-0000-0000-0000-000000000000', steam_id: '76561198452436974', target_uid: 'f8f86740-0000-0000-0000-000000000000' }], conflicts: [], ready: true });
    mocks.startMigration.mockResolvedValue({ id: 'job-migrate', type: 'save_migration', status: 'queued', progress: 0, message: 'queued' });
  });

  it('requires an explicit valid world selection before importing a multi-world archive', async () => {
    mocks.inspectArchive.mockResolvedValue({
      id: 'inspect-1', file_name: 'world.zip', selected_candidate_id: '', requires_selection: true,
      expires_at: '2026-07-22T13:00:00Z',
      candidates: [
        { id: 'candidate-a', relative_path: 'world-a', world_id: 'world-a', player_count: 3, level_sha256: 'a', level_size: 10, valid: true, warnings: [], errors: [] },
        { id: 'candidate-b', relative_path: 'world-b', world_id: 'world-b', player_count: 5, level_sha256: 'b', level_size: 20, valid: true, warnings: [], errors: [] },
        { id: 'candidate-bad', relative_path: 'broken', world_id: 'broken', player_count: 0, level_sha256: 'c', level_size: 4, valid: false, warnings: [], errors: ['parse failed'] },
      ],
    });
    renderSaveSources();

    fireEvent.change(screen.getByLabelText('存档归档文件'), { target: { files: [new File(['archive'], 'world.zip')] } });
    fireEvent.click(screen.getByRole('button', { name: '检查存档' }));

    expect(await screen.findByText('选择要导入的世界')).toBeInTheDocument();
    expect(screen.getByLabelText('世界 world-b')).toBeEnabled();
    expect(screen.getByLabelText('世界 broken')).toBeDisabled();
    fireEvent.click(screen.getByLabelText('世界 world-b'));
    fireEvent.click(screen.getByRole('button', { name: '导入所选世界' }));

    await waitFor(() => expect(mocks.selectImportCandidate).toHaveBeenCalledWith('inspect-1', 'candidate-b'));
    await waitFor(() => expect(mocks.importInspected).toHaveBeenCalledWith('inspect-1', ''));
  });

  it('imports the server-selected candidate immediately when exactly one world is valid', async () => {
    mocks.inspectArchive.mockResolvedValue({
      id: 'inspect-2', file_name: 'single.zip', selected_candidate_id: 'candidate-only', requires_selection: false,
      expires_at: '2026-07-22T13:00:00Z',
      candidates: [{ id: 'candidate-only', relative_path: 'world', world_id: 'world', player_count: 1, level_sha256: 'a', level_size: 10, valid: true, warnings: [], errors: [] }],
    });
    renderSaveSources();

    fireEvent.change(screen.getByLabelText('存档归档文件'), { target: { files: [new File(['archive'], 'single.zip')] } });
    fireEvent.click(screen.getByRole('button', { name: '检查存档' }));

    await waitFor(() => expect(mocks.importInspected).toHaveBeenCalledWith('inspect-2', ''));
    expect(mocks.selectImportCandidate).not.toHaveBeenCalled();
  });

  it('keeps invalid inspection results actionable instead of hiding the upload controls', async () => {
    mocks.inspectArchive.mockResolvedValue({
      id: 'inspect-invalid', file_name: 'broken.zip', selected_candidate_id: '', requires_selection: false,
      expires_at: '2026-07-22T13:00:00Z',
      candidates: [{ id: 'broken', relative_path: 'world', world_id: 'world', player_count: 0, level_sha256: 'a', level_size: 10, valid: false, warnings: [], errors: ['无法解析 Level.sav'] }],
    });
    renderSaveSources();

    fireEvent.change(screen.getByLabelText('存档归档文件'), { target: { files: [new File(['archive'], 'broken.zip')] } });
    fireEvent.click(screen.getByRole('button', { name: '检查存档' }));

    expect(await screen.findByText('没有可导入的世界')).toBeInTheDocument();
    expect(screen.getByText('无法解析 Level.sav')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重新选择归档' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '导入所选世界' })).toBeDisabled();
  });

  it('shows warnings for a single valid world before importing it', async () => {
    mocks.inspectArchive.mockResolvedValue({
      id: 'inspect-warning', file_name: 'warning.zip', selected_candidate_id: 'candidate-warning', requires_selection: false,
      expires_at: '2026-07-22T13:00:00Z',
      candidates: [{
        id: 'candidate-warning', relative_path: 'world', world_id: 'world', player_count: 1,
        level_sha256: 'a', level_size: 10, valid: true,
        warnings: ['parser_incompatible'], errors: [],
      }],
    });
    renderSaveSources();

    fireEvent.change(screen.getByLabelText('存档归档文件'), { target: { files: [new File(['archive'], 'warning.zip')] } });
    fireEvent.click(screen.getByRole('button', { name: '检查存档' }));

    expect(await screen.findByText(/存档格式不兼容/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '导入所选世界' })).toBeEnabled();
    expect(mocks.importInspected).not.toHaveBeenCalled();
  });

  it('returns to archive inspection after an automatically selected world fails to import', async () => {
    mocks.importInspected.mockRejectedValueOnce(new Error('导入失败'));
    mocks.inspectArchive.mockResolvedValue({
      id: 'inspect-retry', file_name: 'single.zip', selected_candidate_id: 'only', requires_selection: false,
      expires_at: '2026-07-22T13:00:00Z',
      candidates: [{ id: 'only', relative_path: 'world', world_id: 'world', player_count: 1, level_sha256: 'a', level_size: 10, valid: true, warnings: [], errors: [] }],
    });
    renderSaveSources();

    fireEvent.change(screen.getByLabelText('存档归档文件'), { target: { files: [new File(['archive'], 'single.zip')] } });
    fireEvent.click(screen.getByRole('button', { name: '检查存档' }));

    expect(await screen.findByText('导入失败')).toBeInTheDocument();
    expect(screen.getByLabelText('存档归档文件')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '检查存档' })).toBeEnabled();
  });

  it('requires explicit high-risk confirmation when target UID mode cannot be proven', async () => {
    mocks.list.mockResolvedValue({ items: [{ id: 'import-one', name: '旧世界', kind: 'import', active: true }], active_status: status });
    renderSaveSources();

    fireEvent.click(await screen.findByRole('button', { name: '玩家迁移向导' }));
    fireEvent.click(screen.getByRole('button', { name: '载入旧玩家' }));
    fireEvent.click(await screen.findByLabelText('选择旧玩家 旧玩家'));
    fireEvent.change(screen.getByLabelText('旧玩家的新 SteamID64'), { target: { value: '76561198452436974' } });
    fireEvent.click(screen.getByRole('button', { name: '运行迁移预检' }));

    expect(await screen.findByText('无法自动证明目标 UID 模式')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '开始自动迁移' })).toBeDisabled();
    fireEvent.click(screen.getByLabelText('手动选择 NoSteam UID'));
    fireEvent.change(screen.getByLabelText('高风险确认'), { target: { value: 'USE NOSTEAM UID' } });
    expect(screen.getByRole('button', { name: '开始自动迁移' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: '开始自动迁移' }));
    await waitFor(() => expect(mocks.startMigration).toHaveBeenCalledWith(expect.objectContaining({
      source_id: 'import-one', target_mode: 'nosteam', manual_mode_confirmation: 'USE NOSTEAM UID', confirmation: 'MIGRATE PLAYERS',
    })));
  });
});
