import { describe, expect, it } from 'vitest';
import { backupsApi } from './backups';

describe('backups api', () => {
  it('builds an encoded same-origin attachment URL', () => {
    expect(backupsApi.downloadUrl('palpanel manual.zip')).toBe('/api/backups/palpanel%20manual.zip/download');
  });
});
