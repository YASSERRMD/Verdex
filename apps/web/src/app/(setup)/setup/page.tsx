'use client';

import { useSetup } from '@/lib/hooks/useSetup';
import { StepIndicator } from '@/components/ui/StepIndicator';
import { Card } from '@/components/ui/Card';
import { JurisdictionStep } from './steps/JurisdictionStep';
import { LanguageStep } from './steps/LanguageStep';
import { ProviderStep } from './steps/ProviderStep';
import { CompleteStep } from './steps/CompleteStep';

const WIZARD_STEPS = [
  { id: 'jurisdiction', label: 'Jurisdiction' },
  { id: 'language', label: 'Language' },
  { id: 'provider', label: 'Provider' },
  { id: 'complete', label: 'Complete' },
];

export default function SetupPage() {
  const {
    state,
    currentStep,
    currentStepIndex,
    loading,
    updateState,
    goNext,
    goBack,
    submitSetup,
  } = useSetup();

  const handleProviderNext = async () => {
    try {
      await submitSetup();
    } catch {
      // Error displayed inside the hook
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-br from-primary-900 via-primary-800 to-primary-700 px-4 py-12">
      <div className="w-full max-w-xl space-y-6">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-accent-DEFAULT">
            <span className="text-lg font-bold text-primary-900">V</span>
          </div>
          <h1 className="text-2xl font-bold text-white">Verdex Setup</h1>
          <p className="mt-1 text-sm text-primary-200">Configure your judicial workspace</p>
        </div>

        {/* Step Indicator */}
        <div className="rounded-xl bg-white/10 px-6 py-4">
          <StepIndicator steps={WIZARD_STEPS} currentStepIndex={currentStepIndex} />
        </div>

        {/* Step Card */}
        <Card>
          {currentStep === 'jurisdiction' && (
            <JurisdictionStep
              value={state.jurisdiction}
              onChange={(jurisdiction) => updateState({ jurisdiction })}
              onNext={goNext}
            />
          )}
          {currentStep === 'language' && (
            <LanguageStep
              value={state.language}
              onChange={(language) => updateState({ language })}
              onNext={goNext}
              onBack={goBack}
            />
          )}
          {currentStep === 'provider' && (
            <ProviderStep
              value={state.provider}
              onChange={(provider) => updateState({ provider })}
              onNext={handleProviderNext}
              onBack={goBack}
              loading={loading}
            />
          )}
          {currentStep === 'complete' && <CompleteStep />}
        </Card>
      </div>
    </div>
  );
}
