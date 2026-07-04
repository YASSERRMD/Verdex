'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import type { SupportedLanguage } from '@/types';
import { directionFor, type TextDirection } from './direction';
import { translate, type TranslationKey } from './strings';

const LOCALE_COOKIE = 'verdex_locale';
const DEFAULT_LOCALE: SupportedLanguage = 'en';

/**
 * Task 6's client-side locale-switching mechanism: a small
 * context/cookie-based switcher, not a full routing-i18n framework
 * rewrite. A cookie (rather than sessionStorage, which apps/web's own
 * auth.ts already uses for the session) is used deliberately so a
 * chosen locale is readable synchronously on first paint via
 * document.cookie, before React hydrates -- the same requirement
 * driving RootLayout's dir attribute (see src/app/layout.tsx).
 */
function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return match ? decodeURIComponent(match[1]) : null;
}

function writeCookie(name: string, value: string): void {
  if (typeof document === 'undefined') return;
  // 1 year, mirroring a typical long-lived user-preference cookie.
  const maxAgeSeconds = 60 * 60 * 24 * 365;
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${maxAgeSeconds}; SameSite=Lax`;
}

function isSupportedLanguage(value: string | null): value is SupportedLanguage {
  return value === 'ar' || value === 'ur' || value === 'ta' || value === 'en';
}

/** readLocaleCookie is exported for server-side / non-React callers. */
export function readLocaleCookie(): SupportedLanguage {
  const raw = readCookie(LOCALE_COOKIE);
  return isSupportedLanguage(raw) ? raw : DEFAULT_LOCALE;
}

interface LocaleContextValue {
  locale: SupportedLanguage;
  direction: TextDirection;
  setLocale: (locale: SupportedLanguage) => void;
  t: (key: TranslationKey) => string;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

export interface LocaleProviderProps {
  children: ReactNode;
  /** Overrides the initial locale (mainly for tests); defaults to the cookie value. */
  initialLocale?: SupportedLanguage;
}

/**
 * LocaleProvider is the root of task 6's locale-switching mechanism:
 * it reads verdex_locale from a cookie on mount, exposes the current
 * locale/direction/translate function via useLocale, and persists a
 * change back to the cookie (and the root <html> dir attribute) when
 * setLocale is called.
 */
export function LocaleProvider({ children, initialLocale }: LocaleProviderProps) {
  const [locale, setLocaleState] = useState<SupportedLanguage>(initialLocale ?? DEFAULT_LOCALE);

  // Hydrate from the cookie on mount (client-only; avoids a
  // server/client render mismatch since the cookie is not read during
  // server rendering here).
  useEffect(() => {
    if (initialLocale) return;
    setLocaleState(readLocaleCookie());
  }, [initialLocale]);

  const direction = useMemo(() => directionFor(locale), [locale]);

  // Keep the root <html> element's dir/lang attributes in sync
  // whenever the locale changes, so RTL layout (task 3) applies
  // immediately without a full page reload.
  useEffect(() => {
    if (typeof document === 'undefined') return;
    document.documentElement.dir = direction;
    document.documentElement.lang = locale;
  }, [locale, direction]);

  const setLocale = useCallback((next: SupportedLanguage) => {
    setLocaleState(next);
    writeCookie(LOCALE_COOKIE, next);
  }, []);

  const t = useCallback((key: TranslationKey) => translate(locale, key), [locale]);

  const value = useMemo<LocaleContextValue>(
    () => ({ locale, direction, setLocale, t }),
    [locale, direction, setLocale, t],
  );

  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

/**
 * useLocale reads the current locale/direction/translate function from
 * the nearest LocaleProvider. Falls back to a default (English/LTR)
 * value with a no-op setLocale when rendered outside a LocaleProvider,
 * rather than throwing, so existing components/tests that do not wrap
 * themselves in a provider keep working unchanged (task 1's
 * "additive, not a breaking rewrite" requirement).
 */
export function useLocale(): LocaleContextValue {
  const ctx = useContext(LocaleContext);
  if (ctx) return ctx;
  return {
    locale: DEFAULT_LOCALE,
    direction: directionFor(DEFAULT_LOCALE),
    setLocale: () => {},
    t: (key) => translate(DEFAULT_LOCALE, key),
  };
}
