import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AxiosResponse } from 'axios';
import { apiClient } from './client';
import { mapLocalScanResult, mapSteamWorkshopAuthStatus, mapWorkshopItem, mapWorkshopStatus, modsApi } from './mods';
import { AI_OPERATION_TIMEOUT_MS } from './requestTimeouts';

describe('mods api mapping', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('maps local scan reconciliation data and requests a fresh scan', async () => {
    const payload = {
      server_dir: 'D:\\Pal Server',
      scanned_at: '2026-07-15T10:00:00Z',
      findings: [{
        id: 'localmod_123',
        revision: 'revision-1',
        ownership: 'manual',
        state: 'duplicate',
        source: 'legacy_pak',
        confidence: 'high',
        name: '本地 Mod',
        package_name: 'LocalPackage',
        version: '2.0',
        enabled: true,
        duplicate: true,
        paths: ['D:\\Pal Server\\Pal\\Content\\Paks\\~mods\\Local.pak'],
        classifications: ['manual', 'present', 'duplicate'],
        issues: ['two versions were found'],
        ignored: false,
        actions: [{ action: 'ignore', available: true, confirmation_required: false }],
      }],
      skipped_paths: ['D:\\Pal Server\\Pal\\Binaries\\Win64\\Mods\\linked'],
      warnings: ['a link was skipped'],
    };
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { ok: true, data: payload },
      status: 200,
    } as AxiosResponse);

    expect(mapLocalScanResult(payload)).toMatchObject({
      server_dir: 'D:\\Pal Server',
      findings: [{
        ownership: 'manual',
        state: 'duplicate',
        source: 'legacy_pak',
        confidence: 'high',
        id: 'localmod_123',
        revision: 'revision-1',
        actions: [{ action: 'ignore', available: true }],
        paths: ['D:\\Pal Server\\Pal\\Content\\Paks\\~mods\\Local.pak'],
      }],
    });
    await expect(modsApi.scanLocal()).resolves.toMatchObject({ findings: [{ name: '本地 Mod' }] });
    expect(post).toHaveBeenCalledWith('/mods/local/scan', undefined, { timeout: 60_000 });

    post.mockResolvedValueOnce({
      data: { ok: true, data: { action: 'ignore', finding_id: 'localmod_123', message: 'ignored', scan: payload } },
      status: 200,
    } as AxiosResponse);
    await expect(modsApi.actOnLocalFinding({ id: 'localmod_123', revision: 'revision-1' }, 'ignore')).resolves.toMatchObject({
      action: 'ignore',
      scan: { findings: [{ id: 'localmod_123' }] },
    });
    expect(post).toHaveBeenLastCalledWith('/mods/local/findings/localmod_123/actions', {
      action: 'ignore', revision: 'revision-1', confirm: undefined,
    });
  });

  it('maps Workshop metadata with safe fallbacks', () => {
    expect(mapWorkshopItem(null)).toMatchObject({
      id: '',
      title: 'Untitled Workshop Item',
      tags: [],
      installed: false,
      enabled: false,
      update_available: false,
    });

    const item = mapWorkshopItem({
      id: 123456,
      title: 'Server Mod',
      preview_url: null,
      tags: ['QoL', '', 'Server'],
      file_size: '2048',
      subscriptions: '99',
      time_updated: '200',
      installed: true,
      enabled: true,
      update_available: true,
    });

    expect(item).toMatchObject({
      id: '123456',
      title: 'Server Mod',
      tags: ['QoL', 'Server'],
      file_size: 2048,
      subscriptions: 99,
      time_updated: 200,
      installed: true,
      enabled: true,
      update_available: true,
    });
    expect(item.steam_url).toContain('123456');
  });

  it('maps Workshop status without exposing key material', () => {
    expect(
      mapWorkshopStatus({
        configured: true,
        key_source: 'environment',
        app_id: 1623730,
        key: 'should-not-be-read',
      }),
    ).toEqual({
      configured: true,
      key_source: 'environment',
      app_id: '1623730',
    });
  });

  it('does not preserve obsolete or unknown Workshop key sources', () => {
    expect(mapWorkshopStatus({ configured: true, key_source: 'embedded', app_id: '1623730' })).toEqual({
      configured: false,
      key_source: '',
      app_id: '1623730',
    });
  });

  it('accepts the bundled Workshop key source without exposing key material', () => {
    expect(mapWorkshopStatus({ configured: true, key_source: 'bundled', app_id: '1623730', key: 'hidden' })).toEqual({
      configured: true,
      key_source: 'bundled',
      app_id: '1623730',
    });
  });

  it('maps Steam Workshop authentication without reading credential material', async () => {
    const status = mapSteamWorkshopAuthStatus({
      supported: true,
      steamcmd_installed: true,
      credentials_secure: true,
      login_in_progress: false,
      logged_in: true,
      verification_required: false,
      account_name: 'steam_account',
      last_verified_at: '2026-07-15T12:00:00Z',
      message: 'verified',
      password: 'must-not-be-read',
      guard_code: 'must-not-be-read',
    });

    expect(status).toEqual({
      supported: true,
      steamcmd_installed: true,
      credentials_secure: true,
      login_in_progress: false,
      logged_in: true,
      verification_required: false,
      account_name: 'steam_account',
      last_verified_at: '2026-07-15T12:00:00Z',
      message: 'verified',
    });
    expect(status).not.toHaveProperty('password');
    expect(status).not.toHaveProperty('guard_code');
  });

  it('uses only the Steam account name for start and verification', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { ok: true, data: { supported: true, steamcmd_installed: true, logged_in: false } },
      status: 200,
    } as AxiosResponse);
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: { ok: true, data: { supported: true, steamcmd_installed: true, logged_in: false, login_in_progress: true } },
      status: 200,
    } as AxiosResponse);

    await modsApi.workshopAuthStatus();
    await modsApi.startWorkshopAuth('  steam_account  ');
    await modsApi.verifyWorkshopAuth('steam_account');

    expect(get).toHaveBeenCalledWith('/mods/workshop/auth/status', { timeout: 60_000 });
    expect(post).toHaveBeenNthCalledWith(1, '/mods/workshop/auth/start', { account_name: 'steam_account' }, { timeout: 60_000 });
    expect(post).toHaveBeenNthCalledWith(2, '/mods/workshop/auth/verify', { account_name: 'steam_account' }, { timeout: 60_000 });
  });

  it('posts Workshop install requests with item_id and enable fields', async () => {
    const postSpy = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: {
        ok: true,
        data: {
          id: 'job_1',
          type: 'workshop_download',
          status: 'waiting',
          progress: 0,
          message: 'queued',
          created_at: new Date(0).toISOString(),
        },
      },
      status: 202,
    } as AxiosResponse);

    await modsApi.downloadWorkshop('123456789', true);

    expect(postSpy).toHaveBeenCalledWith('/mods/workshop', { item_id: '123456789', enable: true });
    postSpy.mockRestore();
  });

  it('allows Workshop AI translation to run through the provider timeout', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: {
        ok: true,
        data: {
          text: '中文译文',
          target_language: 'zh-CN',
          model: 'translation-model',
          generated_at: '2026-07-10T08:23:16Z',
          cached: false,
        },
      },
      status: 200,
    });

    await expect(modsApi.translateWorkshop('3625364851')).resolves.toMatchObject({ text: '中文译文' });
    expect(post).toHaveBeenCalledWith(
      '/mods/workshop/3625364851/translate',
      { force: false },
      { timeout: AI_OPERATION_TIMEOUT_MS },
    );
    expect(AI_OPERATION_TIMEOUT_MS).toBeGreaterThan(105_000);
    post.mockRestore();
  });

  it('maps URL inspection, candidate selection, and import jobs', async () => {
    const post = vi.spyOn(apiClient, 'post')
      .mockResolvedValueOnce({
        data: {
          ok: true,
          data: {
            id: 'inspection/one',
            source_type: 'github_release',
            source: 'https://github.com/example/mod/releases/latest',
            candidates: [{
              id: 'candidate one',
              source_type: 'github_asset',
              file_name: 'mod.zip',
              file_size: '2048',
              action: 'new',
              ready: true,
              warnings: ['The new mod will be installed disabled.'],
            }],
            selected_candidate_id: 'candidate one',
            expires_at: '2026-07-14T01:00:00Z',
          },
        },
        status: 200,
      } as AxiosResponse)
      .mockResolvedValueOnce({
        data: {
          ok: true,
          data: {
            id: 'inspection/one',
            source_type: 'github_release',
            source: 'https://github.com/example/mod/releases/latest',
            candidates: [],
            selected_candidate_id: 'candidate one',
            expires_at: '2026-07-14T01:00:00Z',
          },
        },
        status: 200,
      } as AxiosResponse)
      .mockResolvedValueOnce({
        data: {
          ok: true,
          data: {
            id: 'job_import',
            type: 'mod_import',
            status: 'queued',
            progress: 0,
            message: 'queued mod import',
            created_at: '2026-07-14T00:00:00Z',
            updated_at: '2026-07-14T00:00:00Z',
          },
        },
        status: 202,
      } as AxiosResponse);

    const inspection = await modsApi.inspectImport({ source: 'https://github.com/example/mod/releases/latest' });
    expect(inspection.candidates[0]).toMatchObject({
      id: 'candidate one',
      source_type: 'github_asset',
      file_size: 2048,
      action: 'new',
      ready: true,
    });
    await modsApi.selectImportCandidate(inspection.id, 'candidate one');
    const job = await modsApi.importInspected(inspection.id, 'candidate one');

    expect(post).toHaveBeenNthCalledWith(1, '/mods/import/inspect', {
      source: 'https://github.com/example/mod/releases/latest',
    });
    expect(post).toHaveBeenNthCalledWith(2, '/mods/import/inspect/inspection%2Fone/select', {
      candidate_id: 'candidate one',
    });
    expect(post).toHaveBeenNthCalledWith(3, '/mods/import', {
      inspection_id: 'inspection/one',
      candidate_id: 'candidate one',
    });
    expect(job).toMatchObject({ id: 'job_import', type: 'mod_import', status: 'waiting' });
  });

  it('sends local ZIP inspections as multipart form data', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({
      data: {
        ok: true,
        data: {
          id: 'inspection_local',
          source_type: 'local_zip',
          source: 'local.zip',
          candidates: [],
          expires_at: '2026-07-14T01:00:00Z',
        },
      },
      status: 200,
    } as AxiosResponse);
    const file = new File(['zip'], 'local.zip', { type: 'application/zip' });

    await modsApi.inspectImport({ file });

    const form = post.mock.calls[0][1] as FormData;
    expect(post.mock.calls[0][0]).toBe('/mods/import/inspect');
    expect(form.get('file')).toBe(file);
    expect(post.mock.calls[0][2]).toEqual({ headers: { 'Content-Type': 'multipart/form-data' } });
  });
});
