import { describe, expect, it } from 'vitest';
import { mapPal } from './pals';

describe('Pal DTO mapping', () => {
  it('preserves localized display fields and raw identifiers', () => {
    expect(
      mapPal({
        id: 'pal-1',
        character_id: 'PinkCat',
        species_name: '捣蛋猫',
        name: '捣蛋猫',
        rarity: 'Common',
        rarity_name: '普通',
        passives: ['卓绝技艺'],
        raw_passives: ['CraftSpeed_up3'],
        level: 2,
      }),
    ).toMatchObject({
      id: 'pal-1',
      character_id: 'PinkCat',
      species_name: '捣蛋猫',
      name: '捣蛋猫',
      rarity: 'Common',
      rarity_name: '普通',
      passives: ['卓绝技艺'],
      raw_passives: ['CraftSpeed_up3'],
      level: 2,
    });
  });

  it('keeps the existing fallback for unknown Pal IDs', () => {
    expect(mapPal({ character_id: 'FuturePal_1' }).name).toBe('FuturePal_1');
  });
});
