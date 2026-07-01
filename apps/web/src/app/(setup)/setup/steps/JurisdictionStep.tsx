'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Select } from '@/components/ui/Select';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import type { Jurisdiction, JurisdictionSetup } from '@/types';

const COURT_LEVEL_OPTIONS = [
  { value: 'supreme', label: 'Supreme Court' },
  { value: 'appellate', label: 'Appellate Court' },
  { value: 'high', label: 'High Court' },
  { value: 'district', label: 'District Court' },
  { value: 'magistrate', label: 'Magistrate Court' },
  { value: 'family', label: 'Family Court' },
  { value: 'commercial', label: 'Commercial Court' },
  { value: 'other', label: 'Other' },
];

interface JurisdictionStepProps {
  value: JurisdictionSetup;
  onChange: (val: JurisdictionSetup) => void;
  onNext: () => void;
}

export function JurisdictionStep({ value, onChange, onNext }: JurisdictionStepProps) {
  const [jurisdictions, setJurisdictions] = useState<Jurisdiction[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Partial<JurisdictionSetup>>({});

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    apiFetch<{ jurisdictions: Jurisdiction[] }>('/api/v1/jurisdictions')
      .then((data) => {
        if (!cancelled) setJurisdictions(data.jurisdictions);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setFetchError(
            err instanceof Error ? err.message : 'Failed to load jurisdictions.',
          );
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const countryOptions = Array.from(
    new Map(jurisdictions.map((j) => [j.countryCode, j.country])).entries(),
  ).map(([code, name]) => ({ value: code, label: name }));

  const validate = (): boolean => {
    const e: Partial<JurisdictionSetup> = {};
    if (!value.country) e.country = 'Please select a country.';
    if (!value.courtLevel) e.courtLevel = 'Please select a court level.';
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleNext = () => {
    if (validate()) onNext();
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">Jurisdiction</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Select the country and court level you operate in.
        </p>
      </div>

      {loading && (
        <div className="flex items-center gap-2 text-sm text-neutral-500">
          <Spinner size="sm" /> Loading jurisdictions…
        </div>
      )}

      {fetchError && (
        <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700">
          {fetchError} — you may still proceed with manual entry.
        </div>
      )}

      <div className="space-y-4">
        <Select
          label="Country"
          placeholder="Select country"
          value={value.country}
          options={
            countryOptions.length > 0
              ? countryOptions
              : [{ value: value.country, label: value.country || 'Loading…' }]
          }
          error={errors.country}
          onChange={(e) => onChange({ ...value, country: e.target.value })}
          disabled={loading}
        />

        <Select
          label="Court Level"
          placeholder="Select court level"
          value={value.courtLevel}
          options={COURT_LEVEL_OPTIONS}
          error={errors.courtLevel}
          onChange={(e) => onChange({ ...value, courtLevel: e.target.value })}
        />
      </div>

      <div className="flex justify-end">
        <Button onClick={handleNext} variant="primary">
          Continue
        </Button>
      </div>
    </div>
  );
}
