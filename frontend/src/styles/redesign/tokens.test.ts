import { describe, expect, it } from 'vitest';
import indexCSS from '../../index.css?raw';

describe('redesign token ownership', () => {
  it('keeps Tailwind theme colors, geometry, spacing, and shadows sourced from pp tokens', () => {
    for (const declaration of [
      '--color-slate-50: var(--pp-color-slate-50);',
      '--color-sky-500: var(--pp-color-sky-500);',
      '--color-emerald-500: var(--pp-color-emerald-500);',
      '--color-blue-500: var(--pp-color-blue-500);',
      '--radius-sm: var(--pp-radius-tailwind-sm);',
      '--radius-4xl: var(--pp-radius-tailwind-4xl);',
      '--spacing: var(--pp-gap-1);',
      '--shadow-sm: var(--pp-shadow-sm);',
    ]) {
      expect(indexCSS).toContain(declaration);
    }
  });
});
