'use client';

import { useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Button } from '@/components/ui/Button';
import type { CaseCategory, CaseCreationInput } from '@/types';

const CATEGORY_OPTIONS: { value: CaseCategory; label: string }[] = [
  { value: 'civil', label: 'Civil' },
  { value: 'criminal', label: 'Criminal' },
  { value: 'family', label: 'Family' },
  { value: 'commercial', label: 'Commercial' },
  { value: 'administrative', label: 'Administrative' },
  { value: 'other', label: 'Other' },
];

export interface CaseCreationFormProps {
  /** Called with the newly created case ID once the API call succeeds. */
  onCreated?: (caseId: string) => void;
}

const EMPTY_INPUT: CaseCreationInput = {
  category: '',
  firstPartyName: '',
  secondPartyName: '',
};

export function CaseCreationForm({ onCreated }: CaseCreationFormProps) {
  const [value, setValue] = useState<CaseCreationInput>(EMPTY_INPUT);
  const [errors, setErrors] = useState<Partial<Record<keyof CaseCreationInput, string>>>({});
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const validate = (): boolean => {
    const e: Partial<Record<keyof CaseCreationInput, string>> = {};
    if (!value.category) e.category = 'Please select a case category.';
    if (!value.firstPartyName.trim()) e.firstPartyName = 'First party name is required.';
    if (!value.secondPartyName.trim()) e.secondPartyName = 'Second party name is required.';
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setSubmitError(null);
    if (!validate()) return;

    setSubmitting(true);
    try {
      const result = await apiFetch<{ id: string }>('/api/v1/cases', {
        method: 'POST',
        body: JSON.stringify({
          category: value.category,
          firstPartyName: value.firstPartyName.trim(),
          secondPartyName: value.secondPartyName.trim(),
        }),
      });
      onCreated?.(result.id);
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Failed to create case.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6" noValidate>
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">New Case</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Start by identifying the case category and the parties involved. You can attach
          documents and audio in the next step.
        </p>
      </div>

      <div className="space-y-4">
        <Select
          label="Case Category"
          placeholder="Select a category"
          value={value.category}
          options={CATEGORY_OPTIONS}
          error={errors.category}
          onChange={(e) =>
            setValue((v) => ({ ...v, category: e.target.value as CaseCategory }))
          }
        />

        <Input
          label="First Party Name"
          placeholder="e.g. Jane Doe"
          value={value.firstPartyName}
          error={errors.firstPartyName}
          onChange={(e) => setValue((v) => ({ ...v, firstPartyName: e.target.value }))}
        />

        <Input
          label="Second Party Name"
          placeholder="e.g. Acme Corp"
          value={value.secondPartyName}
          error={errors.secondPartyName}
          onChange={(e) => setValue((v) => ({ ...v, secondPartyName: e.target.value }))}
        />
      </div>

      {submitError && (
        <p role="alert" className="text-sm text-red-600">
          {submitError}
        </p>
      )}

      <div className="flex justify-end">
        <Button type="submit" variant="primary" loading={submitting}>
          Create Case
        </Button>
      </div>
    </form>
  );
}
