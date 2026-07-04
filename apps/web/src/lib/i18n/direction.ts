import type { SupportedLanguage } from '@/types';

export type TextDirection = 'ltr' | 'rtl';

/**
 * Which SupportedLanguage values are right-to-left, mirroring
 * packages/localization's Go-side SupportedLocales table exactly
 * (Arabic and Urdu are RTL; Tamil and English are LTR) -- task 3's
 * RTL data, kept in exactly one place on this side so
 * LocaleContext/useDirection/LanguageStep.tsx never drift apart. Also
 * matches apps/web's own pre-existing LanguageStep.tsx LANGUAGE_OPTIONS
 * dir values for ar/ur/ta/en.
 */
const RTL_LANGUAGES: ReadonlySet<SupportedLanguage> = new Set<SupportedLanguage>(['ar', 'ur']);

/** directionFor returns the TextDirection for a SupportedLanguage. */
export function directionFor(locale: SupportedLanguage): TextDirection {
  return RTL_LANGUAGES.has(locale) ? 'rtl' : 'ltr';
}

/** isRTL reports whether locale is a right-to-left language. */
export function isRTL(locale: SupportedLanguage): boolean {
  return directionFor(locale) === 'rtl';
}
