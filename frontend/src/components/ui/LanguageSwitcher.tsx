import React from 'react';
import { Languages } from 'lucide-react';
import { supportedLocales, useI18n, type Locale } from '../../i18n';

export const LanguageSwitcher: React.FC<{ compact?: boolean; buttons?: boolean; className?: string }> = ({ compact = false, buttons = false, className = '' }) => {
  const { locale, setLocale, t } = useI18n();
  if (buttons) {
    return (
      <div role="group" aria-label={t('language.label')} className={`inline-flex rounded-lg border border-slate-200 bg-slate-50 p-1 ${className}`}>
        {supportedLocales.map((value) => (
          <button key={value} type="button" onClick={() => setLocale(value)} aria-pressed={locale === value} className={`rounded-md px-3 py-1.5 text-xs font-semibold ${locale === value ? 'bg-white text-sky-700 shadow-sm' : 'text-slate-500'}`}>
            {t(`language.${value}`)}
          </button>
        ))}
      </div>
    );
  }
  return (
    <label className={`inline-flex items-center gap-2 ${className}`}>
      <Languages size={compact ? 14 : 16} aria-hidden="true" />
      {!compact && <span className="text-xs font-semibold">{t('language.label')}</span>}
      <select
        aria-label={t('language.label')}
        value={locale}
        onChange={(event) => setLocale(event.target.value as Locale)}
        className="min-w-0 rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs font-semibold text-slate-600 outline-none focus:border-sky-400"
      >
        {supportedLocales.map((value) => <option key={value} value={value}>{t(`language.${value}`)}</option>)}
      </select>
    </label>
  );
};
