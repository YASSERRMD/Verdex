'use client';

import { useState } from 'react';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Button } from '@/components/ui/Button';
import type { ProviderSetup } from '@/types';

const PROVIDER_TYPES = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'azure-openai', label: 'Azure OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama (self-hosted)' },
  { value: 'custom', label: 'Custom / Other' },
];

interface ProviderErrors {
  type?: string;
  endpoint?: string;
  modelId?: string;
}

interface ProviderStepProps {
  value: ProviderSetup;
  onChange: (val: ProviderSetup) => void;
  onNext: () => void;
  onBack: () => void;
  loading?: boolean;
}

export function ProviderStep({
  value,
  onChange,
  onNext,
  onBack,
  loading = false,
}: ProviderStepProps) {
  const [errors, setErrors] = useState<ProviderErrors>({});

  const validate = (): boolean => {
    const e: ProviderErrors = {};
    if (!value.type) e.type = 'Please select a provider type.';
    if (!value.endpoint.trim()) e.endpoint = 'Endpoint URL is required.';
    else {
      try {
        new URL(value.endpoint);
      } catch {
        e.endpoint = 'Enter a valid URL.';
      }
    }
    if (!value.modelId.trim()) e.modelId = 'Model ID is required.';
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleNext = () => {
    if (validate()) onNext();
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">AI Provider</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Configure the language model provider for generating draft analyses.
        </p>
      </div>

      <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700">
        Provider credentials are stored encrypted and only used to generate non-binding
        draft analyses. All outputs must be reviewed by a qualified judge before use.
      </div>

      <div className="space-y-4">
        <Select
          label="Provider Type"
          placeholder="Select provider"
          value={value.type}
          options={PROVIDER_TYPES}
          error={errors.type}
          onChange={(e) => onChange({ ...value, type: e.target.value })}
        />

        <Input
          label="Endpoint URL"
          type="url"
          placeholder="https://api.openai.com/v1"
          value={value.endpoint}
          error={errors.endpoint}
          helperText="Base URL for the API endpoint"
          onChange={(e) => onChange({ ...value, endpoint: e.target.value })}
        />

        <Input
          label="Model ID"
          type="text"
          placeholder="gpt-4o"
          value={value.modelId}
          error={errors.modelId}
          helperText="The model identifier (e.g. gpt-4o, claude-3-5-sonnet-20241022)"
          onChange={(e) => onChange({ ...value, modelId: e.target.value })}
        />
      </div>

      <div className="flex justify-between">
        <Button variant="secondary" onClick={onBack} disabled={loading}>
          Back
        </Button>
        <Button variant="primary" onClick={handleNext} loading={loading}>
          Finish Setup
        </Button>
      </div>
    </div>
  );
}
