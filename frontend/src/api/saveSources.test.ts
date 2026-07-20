import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { SAVE_ARCHIVE_IMPORT_TIMEOUT_MS, SAVE_INDEX_OPERATION_TIMEOUT_MS } from './requestTimeouts';
import { saveSourcesApi } from './saveSources';

describe('save sources api timeouts', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('does not apply the global eight-second timeout to archive imports', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { ok: true, data: { id: 'save-1', name: 'Imported', kind: 'import' } } });
    const file = new File(['archive'], 'world.tar.gz', { type: 'application/gzip' });

    await saveSourcesApi.importArchive(file, 'Imported');

    expect(post).toHaveBeenCalledWith('/save-sources/import', expect.any(FormData), {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: SAVE_ARCHIVE_IMPORT_TIMEOUT_MS,
    });
    expect(SAVE_ARCHIVE_IMPORT_TIMEOUT_MS).toBe(0);
  });

  it('allows activation and rebuild to wait for save indexing', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { ok: true, data: {} } });

    await saveSourcesApi.activate('save with spaces');
    await saveSourcesApi.rebuild('save with spaces');

    expect(post).toHaveBeenNthCalledWith(1, '/save-sources/save%20with%20spaces/activate', undefined, { timeout: SAVE_INDEX_OPERATION_TIMEOUT_MS });
    expect(post).toHaveBeenNthCalledWith(2, '/save-sources/save%20with%20spaces/rebuild', undefined, { timeout: SAVE_INDEX_OPERATION_TIMEOUT_MS });
    expect(SAVE_INDEX_OPERATION_TIMEOUT_MS).toBe(180_000);
  });
});
