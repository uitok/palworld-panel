import { describe, expect, it } from 'vitest';
import { mapLogs } from './server';

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
});
