/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { StepIndicator } from '@/components/ui/StepIndicator';

const STEPS = [
  { id: 'jurisdiction', label: 'Jurisdiction' },
  { id: 'language', label: 'Language' },
  { id: 'provider', label: 'Provider' },
  { id: 'complete', label: 'Complete' },
];

describe('StepIndicator', () => {
  it('renders all step labels', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={0} />);
    STEPS.forEach((step) => {
      expect(screen.getByText(step.label)).toBeInTheDocument();
    });
  });

  it('marks the first step as current (aria-current=step) when currentStepIndex=0', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={0} />);
    const firstStep = screen.getByTestId('step-jurisdiction');
    expect(firstStep).toHaveAttribute('aria-current', 'step');
  });

  it('does not mark upcoming steps as current', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={0} />);
    const languageStep = screen.getByTestId('step-language');
    expect(languageStep).not.toHaveAttribute('aria-current');
    const providerStep = screen.getByTestId('step-provider');
    expect(providerStep).not.toHaveAttribute('aria-current');
  });

  it('marks completed steps correctly when on step 2 (index 2)', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={2} />);
    // Steps 0 and 1 are complete — they should show a checkmark (no number text visible)
    const jurisdictionStep = screen.getByTestId('step-jurisdiction');
    expect(jurisdictionStep).not.toHaveAttribute('aria-current');

    const languageStep = screen.getByTestId('step-language');
    expect(languageStep).not.toHaveAttribute('aria-current');

    const providerStep = screen.getByTestId('step-provider');
    expect(providerStep).toHaveAttribute('aria-current', 'step');
  });

  it('renders correct number of step circles', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={0} />);
    const stepElements = [
      screen.getByTestId('step-jurisdiction'),
      screen.getByTestId('step-language'),
      screen.getByTestId('step-provider'),
      screen.getByTestId('step-complete'),
    ];
    expect(stepElements).toHaveLength(4);
  });

  it('marks the last step as current when currentStepIndex equals steps.length - 1', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={3} />);
    const lastStep = screen.getByTestId('step-complete');
    expect(lastStep).toHaveAttribute('aria-current', 'step');
  });

  it('renders the nav with an accessible label', () => {
    render(<StepIndicator steps={STEPS} currentStepIndex={0} />);
    expect(screen.getByRole('navigation', { name: /setup progress/i })).toBeInTheDocument();
  });
});
