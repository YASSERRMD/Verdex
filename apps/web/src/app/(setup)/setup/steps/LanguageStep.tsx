'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import type { LanguageSetup, SupportedLanguage } from '@/types';

interface LanguageOption {
  code: SupportedLanguage;
  label: string;
  nativeLabel: string;
  dir: 'ltr' | 'rtl';
}

const LANGUAGE_OPTIONS: LanguageOption[] = [
  { code: 'ar', label: 'Arabic', nativeLabel: 'العربية', dir: 'rtl' },
  { code: 'ur', label: 'Urdu', nativeLabel: 'اردو', dir: 'rtl' },
  { code: 'ta', label: 'Tamil', nativeLabel: 'தமிழ்', dir: 'ltr' },
  { code: 'en', label: 'English', nativeLabel: 'English', dir: 'ltr' },
];

interface LanguageStepProps {
  value: LanguageSetup;
  onChange: (val: LanguageSetup) => void;
  onNext: () => void;
  onBack: () => void;
}

export function LanguageStep({ value, onChange, onNext, onBack }: LanguageStepProps) {
  const [error, setError] = useState<string | null>(null);

  const toggle = (code: SupportedLanguage) => {
    const selected = value.selected.includes(code)
      ? value.selected.filter((c) => c !== code)
      : [...value.selected, code];
    onChange({ selected });
    if (selected.length > 0) setError(null);
  };

  const handleNext = () => {
    if (value.selected.length === 0) {
      setError('Please select at least one language.');
      return;
    }
    onNext();
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">Languages</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Select the document languages your court works with.
        </p>
      </div>

      <fieldset>
        <legend className="sr-only">Select languages</legend>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {LANGUAGE_OPTIONS.map((lang) => {
            const checked = value.selected.includes(lang.code);
            return (
              <label
                key={lang.code}
                className={`flex cursor-pointer items-center gap-3 rounded-lg border-2 px-4 py-3 transition-colors ${
                  checked
                    ? 'border-primary-DEFAULT bg-primary-50'
                    : 'border-neutral-200 bg-white hover:border-primary-300'
                }`}
              >
                <input
                  type="checkbox"
                  name="language"
                  value={lang.code}
                  checked={checked}
                  onChange={() => toggle(lang.code)}
                  className="h-4 w-4 rounded border-neutral-300 text-primary-DEFAULT focus:ring-primary-DEFAULT/30"
                />
                <span className="flex flex-col">
                  <span className="text-sm font-medium text-neutral-800">{lang.label}</span>
                  <span
                    dir={lang.dir}
                    className="text-xs text-neutral-500"
                    lang={lang.code}
                  >
                    {lang.nativeLabel}
                  </span>
                </span>
              </label>
            );
          })}
        </div>
        {error && (
          <p role="alert" className="mt-2 text-xs text-red-600">
            {error}
          </p>
        )}
      </fieldset>

      <div className="flex justify-between">
        <Button variant="secondary" onClick={onBack}>
          Back
        </Button>
        <Button variant="primary" onClick={handleNext}>
          Continue
        </Button>
      </div>
    </div>
  );
}
