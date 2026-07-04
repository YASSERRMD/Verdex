import { useLocale } from '@/lib/i18n/LocaleContext';
import type { TextDirection } from '@/lib/i18n/direction';

/**
 * useDirection is task 3's "locale-to-dir-attribute helper/hook":
 * returns the current locale's TextDirection ('ltr' | 'rtl') from the
 * nearest LocaleProvider, for a component that needs to apply
 * dir={direction} conditionally on an element it renders (see
 * RootLayout in src/app/layout.tsx for the top-level application, and
 * LocaleSwitcher.tsx for a component-level one).
 */
export function useDirection(): TextDirection {
  return useLocale().direction;
}
