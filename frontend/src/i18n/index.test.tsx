import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { LanguageSwitcher } from '../components/ui/LanguageSwitcher';
import { detectLocale, I18nProvider, localeStorageKey, useI18n } from '.';

const Probe = () => {
  const { locale, t } = useI18n();
  return <p>{locale}: {t('community.title')}</p>;
};

describe('lightweight i18n', () => {
  const browserLanguages = navigator.languages;
  beforeEach(() => window.localStorage.clear());
  afterEach(() => {
    cleanup();
    Object.defineProperty(navigator, 'languages', { configurable: true, value: browserLanguages });
  });

  it('prefers the persisted locale', () => {
    window.localStorage.setItem(localeStorageKey, 'en-US');
    expect(detectLocale()).toBe('en-US');
  });

  it('selects a supported locale from browser preferences', () => {
    Object.defineProperty(navigator, 'languages', { configurable: true, value: ['zh-Hans-CN', 'en-US'] });
    expect(detectLocale()).toBe('zh-CN');
  });

  it('switches translations and persists the choice without dependencies', async () => {
    window.localStorage.setItem(localeStorageKey, 'zh-CN');
    render(<I18nProvider><LanguageSwitcher /><Probe /></I18nProvider>);

    expect(screen.getByText('zh-CN: 社区服务器')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('界面语言'), { target: { value: 'en-US' } });

    expect(await screen.findByText('en-US: Community servers')).toBeInTheDocument();
    await waitFor(() => expect(window.localStorage.getItem(localeStorageKey)).toBe('en-US'));
    expect(document.documentElement.lang).toBe('en-US');
  });
});
