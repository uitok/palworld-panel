import { describe, expect, it } from 'vitest';
import { mapAccessEntries } from './players';

describe('mapAccessEntries', () => {
  it('treats null and non-array payloads as empty arrays', () => {
    expect(mapAccessEntries(null)).toEqual([]);
    expect(mapAccessEntries({})).toEqual([]);
  });

  it('maps array and wrapped players payloads', () => {
    const entry = { steam_id: '76561198000000001', nickname: 'Tester', reason: 'manual' };

    expect(mapAccessEntries([entry])).toMatchObject([entry]);
    expect(mapAccessEntries({ players: [entry] })).toMatchObject([entry]);
  });
});
