import { describe, expect, it } from 'vitest';
import { isPageResourceLoadError } from './pageResourceError';

describe('ErrorBoundary resource failure detection', () => {
  it('recognizes failed lazy-loaded page resources', () => {
    expect(isPageResourceLoadError(new Error('Failed to fetch dynamically imported module: /assets/Setup.js'))).toBe(true);
    expect(isPageResourceLoadError(new Error('ChunkLoadError: Loading chunk 42 failed'))).toBe(true);
  });

  it('keeps ordinary rendering errors recoverable without a full reload', () => {
    expect(isPageResourceLoadError(new Error('Cannot read properties of undefined'))).toBe(false);
  });
});
