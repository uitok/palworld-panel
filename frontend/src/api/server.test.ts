import { describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { mapLogs, serverApi } from './server';

describe('server log mapping', () => {
  it('preserves source and availability metadata for empty logs', () => {
    expect(mapLogs({ logs: '', source: 'file', available: true, reason: 'waiting_for_output', updated_at: '2026-07-10T00:00:00Z' })).toEqual({
      logs: '',
      source: 'file',
      available: true,
      reason: 'waiting_for_output',
      updated_at: '2026-07-10T00:00:00Z',
    });
  });

  it('keeps compatibility with legacy string and array payloads', () => {
    expect(mapLogs('line')).toEqual({ logs: 'line', source: 'none', available: true });
    expect(mapLogs(['one', 'two']).logs).toBe('one\ntwo');
  });

  it('requests an explicit game log channel', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ status: 200, data: { ok: true, data: { logs: 'joined', source: 'paldefender-game', available: true } } });

    const result = await serverApi.getLogs(80, 'joined', 'info', '', 'game');

    expect(get).toHaveBeenCalledWith('/server/logs?tail=80&search=joined&level=info&channel=game');
    expect(result).toMatchObject({ source: 'paldefender-game', logs: 'joined' });
  });
});
