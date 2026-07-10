import { describe, expect, it } from 'vitest';
import { mapSchema } from './settings';

describe('settings dto mapping', () => {
  it('preserves localized labels and enum labels', () => {
    const schema = mapSchema({
      version: '1.0.0',
      fields: [
        {
          key: 'DeathPenalty',
          label: '死亡惩罚',
          group: 'game_balance',
          type: 'enum',
          default: 'All',
          enum: ['None', 'All'],
          enum_labels: {
            None: '不掉落',
            All: '全部掉落（物品、装备和队伍帕鲁）',
          },
          requires_restart: true,
          description: '死亡惩罚。',
        },
      ],
    });

    expect(schema.fields[0]).toMatchObject({
      key: 'DeathPenalty',
      label: '死亡惩罚',
      enum: ['None', 'All'],
      enum_labels: {
        None: '不掉落',
        All: '全部掉落（物品、装备和队伍帕鲁）',
      },
    });
  });

  it('preserves an explicitly unset schema default', () => {
    const schema = mapSchema({
      version: '1.0.0',
      fields: [{ key: 'FutureField', group: 'features', type: 'bool', default: null }],
    });
    expect(schema.fields[0].default).toBeUndefined();
  });
});
