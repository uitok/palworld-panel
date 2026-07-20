import { describe, expect, it } from 'vitest';
import { formatCPUPercent } from '../utils/monitor';

describe('formatCPUPercent', () => {
  it.each([
    [undefined, false, '不可用'],
    [0, true, '0.0% CPU'],
    [0.04, true, '<0.1% CPU'],
    [12.34, true, '12.3% CPU'],
  ] as const)('formats %s (available=%s)', (value, available, expected) => {
    expect(formatCPUPercent(value, available)).toBe(expected);
  });
});
