'use client';

import { GlobeIcon } from 'lucide-react';
import { useLocale } from '@/lib/i18n/LocaleContext';
import type { SupportedLanguage } from '@/types';

interface LocaleOption {
  code: SupportedLanguage;
  label: string;
}

/**
 * Ordered the same way apps/web's existing LanguageStep.tsx
 * LANGUAGE_OPTIONS is (Arabic, Urdu, Tamil, English), so the setup
 * wizard and this runtime switcher never present the four locales in
 * a different order.
 */
const LOCALE_OPTIONS: LocaleOption[] = [
  { code: 'ar', label: 'العربية' },
  { code: 'ur', label: 'اردو' },
  { code: 'ta', label: 'தமிழ்' },
  { code: 'en', label: 'English' },
];

/**
 * LocaleSwitcher is task 6's real, minimal apps/web locale-switching
 * mechanism: a small select wired into TopBar next to the existing
 * user menu, backed by LocaleProvider's cookie-based useLocale hook
 * rather than a new top-level settings page (TopBar already has a
 * dedicated right-slot area for exactly this kind of control -- see
 * NotificationBell, which sits in the same spot).
 */
export function LocaleSwitcher() {
  const { locale, setLocale, t } = useLocale();

  return (
    <label className="flex items-center gap-1.5 text-neutral-500 dark:text-neutral-400">
      <GlobeIcon className="h-4 w-4" aria-hidden="true" />
      <span className="sr-only">{t('common.language')}</span>
      <select
        data-testid="locale-switcher"
        aria-label={t('common.language')}
        value={locale}
        onChange={(e) => setLocale(e.target.value as SupportedLanguage)}
        className="rounded-md border-0 bg-transparent py-1 pl-1 pr-6 text-sm font-medium text-neutral-600 hover:bg-neutral-100 focus:outline-none focus:ring-2 focus:ring-primary-DEFAULT/30 dark:text-neutral-300 dark:hover:bg-neutral-700"
      >
        {LOCALE_OPTIONS.map((opt) => (
          <option key={opt.code} value={opt.code}>
            {opt.label}
          </option>
        ))}
      </select>
    </label>
  );
}
