/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LocaleProvider, useLocale } from '@/lib/i18n/LocaleContext';
import { LocaleSwitcher } from '@/components/layout/LocaleSwitcher';
import { directionFor, isRTL } from '@/lib/i18n/direction';

/** Renders the current locale/direction so tests can assert on them via text content. */
function LocaleProbe() {
  const { locale, direction } = useLocale();
  return (
    <div>
      <span data-testid="probe-locale">{locale}</span>
      <span data-testid="probe-direction">{direction}</span>
    </div>
  );
}

afterEach(() => {
  // Clear the persisted cookie between tests so each test starts from
  // the default locale rather than leaking state across cases.
  document.cookie = 'verdex_locale=; path=/; max-age=0';
  document.documentElement.removeAttribute('dir');
  document.documentElement.removeAttribute('lang');
});

describe('direction.ts', () => {
  it('classifies Arabic and Urdu as right-to-left', () => {
    expect(directionFor('ar')).toBe('rtl');
    expect(directionFor('ur')).toBe('rtl');
    expect(isRTL('ar')).toBe(true);
    expect(isRTL('ur')).toBe(true);
  });

  it('classifies Tamil and English as left-to-right', () => {
    expect(directionFor('ta')).toBe('ltr');
    expect(directionFor('en')).toBe('ltr');
    expect(isRTL('ta')).toBe(false);
    expect(isRTL('en')).toBe(false);
  });
});

describe('LocaleProvider / useLocale', () => {
  it('defaults to English/LTR when no cookie is set', () => {
    render(
      <LocaleProvider>
        <LocaleProbe />
      </LocaleProvider>,
    );
    expect(screen.getByTestId('probe-locale')).toHaveTextContent('en');
    expect(screen.getByTestId('probe-direction')).toHaveTextContent('ltr');
  });

  it('accepts an initialLocale override for tests', () => {
    render(
      <LocaleProvider initialLocale="ar">
        <LocaleProbe />
      </LocaleProvider>,
    );
    expect(screen.getByTestId('probe-locale')).toHaveTextContent('ar');
    expect(screen.getByTestId('probe-direction')).toHaveTextContent('rtl');
  });

  it('falls back to sensible defaults when rendered outside a provider', () => {
    render(<LocaleProbe />);
    expect(screen.getByTestId('probe-locale')).toHaveTextContent('en');
    expect(screen.getByTestId('probe-direction')).toHaveTextContent('ltr');
  });

  it('applies dir/lang to the root <html> element as the locale changes', async () => {
    render(
      <LocaleProvider>
        <LocaleSwitcher />
      </LocaleProvider>,
    );

    // Default English/LTR applied on mount.
    expect(document.documentElement.dir).toBe('ltr');
    expect(document.documentElement.lang).toBe('en');

    await userEvent.selectOptions(screen.getByTestId('locale-switcher'), 'ar');

    expect(document.documentElement.dir).toBe('rtl');
    expect(document.documentElement.lang).toBe('ar');
  });

  it('persists the chosen locale to the verdex_locale cookie', async () => {
    render(
      <LocaleProvider>
        <LocaleSwitcher />
      </LocaleProvider>,
    );

    await userEvent.selectOptions(screen.getByTestId('locale-switcher'), 'ur');

    expect(document.cookie).toContain('verdex_locale=ur');
  });
});

describe('LocaleSwitcher', () => {
  it('renders one option per supported locale, defaulting to English selected', () => {
    render(
      <LocaleProvider>
        <LocaleSwitcher />
      </LocaleProvider>,
    );
    const select = screen.getByTestId('locale-switcher') as HTMLSelectElement;
    expect(select.value).toBe('en');

    const optionValues = Array.from(select.options).map((o) => o.value);
    expect(optionValues).toEqual(['ar', 'ur', 'ta', 'en']);
  });

  it('changes the reported locale when a new option is selected', async () => {
    render(
      <LocaleProvider>
        <LocaleSwitcher />
        <LocaleProbe />
      </LocaleProvider>,
    );

    await userEvent.selectOptions(screen.getByTestId('locale-switcher'), 'ta');

    expect(screen.getByTestId('probe-locale')).toHaveTextContent('ta');
    expect(screen.getByTestId('probe-direction')).toHaveTextContent('ltr');
  });
});
