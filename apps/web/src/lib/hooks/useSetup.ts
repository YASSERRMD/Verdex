'use client';

import { useState, useCallback } from 'react';
import { apiFetch } from '@/lib/api';
import type { SetupState } from '@/types';

export type SetupStep = 'jurisdiction' | 'language' | 'provider' | 'complete';

const STEP_ORDER: SetupStep[] = ['jurisdiction', 'language', 'provider', 'complete'];

interface UseSetupReturn {
  state: SetupState;
  currentStep: SetupStep;
  currentStepIndex: number;
  totalSteps: number;
  loading: boolean;
  error: string | null;
  canGoBack: boolean;
  canGoForward: boolean;
  goNext: () => void;
  goBack: () => void;
  goToStep: (step: SetupStep) => void;
  updateState: (patch: Partial<SetupState>) => void;
  submitSetup: () => Promise<void>;
  resetSetup: () => void;
}

const initialState: SetupState = {
  jurisdiction: {
    country: '',
    courtLevel: '',
  },
  language: {
    selected: [],
  },
  provider: {
    type: '',
    endpoint: '',
    modelId: '',
  },
};

export function useSetup(): UseSetupReturn {
  const [stepIndex, setStepIndex] = useState(0);
  const [state, setState] = useState<SetupState>(initialState);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const currentStep = STEP_ORDER[stepIndex];

  const goNext = useCallback(() => {
    setStepIndex((i) => Math.min(i + 1, STEP_ORDER.length - 1));
    setError(null);
  }, []);

  const goBack = useCallback(() => {
    setStepIndex((i) => Math.max(i - 1, 0));
    setError(null);
  }, []);

  const goToStep = useCallback((step: SetupStep) => {
    const idx = STEP_ORDER.indexOf(step);
    if (idx !== -1) {
      setStepIndex(idx);
      setError(null);
    }
  }, []);

  const updateState = useCallback((patch: Partial<SetupState>) => {
    setState((prev) => ({ ...prev, ...patch }));
  }, []);

  const submitSetup = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      await apiFetch('/api/v1/setup', {
        method: 'POST',
        body: JSON.stringify(state),
      });
      goToStep('complete');
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : 'Setup submission failed. Please try again.';
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [state, goToStep]);

  const resetSetup = useCallback(() => {
    setStepIndex(0);
    setState(initialState);
    setError(null);
  }, []);

  return {
    state,
    currentStep,
    currentStepIndex: stepIndex,
    totalSteps: STEP_ORDER.length,
    loading,
    error,
    canGoBack: stepIndex > 0,
    canGoForward: stepIndex < STEP_ORDER.length - 1,
    goNext,
    goBack,
    goToStep,
    updateState,
    submitSetup,
    resetSetup,
  };
}
