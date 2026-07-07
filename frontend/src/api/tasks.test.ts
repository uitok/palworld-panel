import { describe, expect, it } from 'vitest';
import { mapJobs, mapJobStatus } from './tasks';

describe('task dto mapping', () => {
  it('maps backend job statuses to frontend statuses', () => {
    expect(mapJobStatus('queued')).toBe('waiting');
    expect(mapJobStatus('completed')).toBe('success');
    expect(mapJobStatus('running')).toBe('running');
    expect(mapJobStatus('failed')).toBe('failed');
  });

  it('treats null and non-array job payloads as empty arrays', () => {
    expect(mapJobs(null)).toEqual([]);
    expect(mapJobs({ ok: true })).toEqual([]);
  });
});
