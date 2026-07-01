import clsx from 'clsx';
import { CheckIcon } from 'lucide-react';

export interface Step {
  id: string;
  label: string;
}

interface StepIndicatorProps {
  steps: Step[];
  currentStepIndex: number;
  className?: string;
}

type StepState = 'complete' | 'current' | 'upcoming';

function getStepState(index: number, currentIndex: number): StepState {
  if (index < currentIndex) return 'complete';
  if (index === currentIndex) return 'current';
  return 'upcoming';
}

export function StepIndicator({ steps, currentStepIndex, className }: StepIndicatorProps) {
  return (
    <nav aria-label="Setup progress" className={clsx('w-full', className)}>
      <ol className="flex items-center justify-between">
        {steps.map((step, index) => {
          const state = getStepState(index, currentStepIndex);
          const isLast = index === steps.length - 1;

          return (
            <li key={step.id} className="relative flex flex-1 flex-col items-center">
              {/* Connector line */}
              {!isLast && (
                <div
                  className={clsx(
                    'absolute left-1/2 top-4 h-0.5 w-full',
                    state === 'complete' ? 'bg-primary-DEFAULT' : 'bg-neutral-200',
                  )}
                  aria-hidden="true"
                />
              )}

              {/* Circle indicator */}
              <div
                data-testid={`step-${step.id}`}
                aria-current={state === 'current' ? 'step' : undefined}
                className={clsx(
                  'relative z-10 flex h-8 w-8 items-center justify-center rounded-full border-2 text-sm font-semibold transition-colors',
                  state === 'complete' && [
                    'border-primary-DEFAULT bg-primary-DEFAULT text-white',
                  ],
                  state === 'current' && [
                    'border-primary-DEFAULT bg-white text-primary-DEFAULT',
                    'ring-2 ring-primary-DEFAULT/20 ring-offset-2',
                  ],
                  state === 'upcoming' && ['border-neutral-300 bg-white text-neutral-400'],
                )}
              >
                {state === 'complete' ? (
                  <CheckIcon className="h-4 w-4" aria-hidden="true" />
                ) : (
                  <span>{index + 1}</span>
                )}
              </div>

              {/* Label */}
              <span
                className={clsx(
                  'mt-2 text-center text-xs font-medium',
                  state === 'complete' && 'text-primary-DEFAULT',
                  state === 'current' && 'text-primary-DEFAULT',
                  state === 'upcoming' && 'text-neutral-400',
                )}
              >
                {step.label}
              </span>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
