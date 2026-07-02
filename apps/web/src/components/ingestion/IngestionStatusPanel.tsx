'use client';

import clsx from 'clsx';
import { CheckCircle2Icon, AlertCircleIcon } from 'lucide-react';
import { Spinner } from '@/components/ui/Spinner';
import type { IngestionStage, IngestionStatus } from '@/types';

export interface IngestionStatusPanelProps {
  /** Current status snapshot. The caller is responsible for polling/subscribing
   * and passing the latest value in — this component is presentation-only. */
  status: IngestionStatus | null;
  className?: string;
}

const STAGE_LABELS: Record<IngestionStage, string> = {
  intake: 'Receiving files',
  extraction: 'Transcribing / extracting text',
  normalize: 'Normalizing text',
  segment: 'Segmenting',
  classify: 'Classifying evidence',
  complete: 'Processing complete',
  failed: 'Processing failed',
};

const STAGE_ORDER: IngestionStage[] = [
  'intake',
  'extraction',
  'normalize',
  'segment',
  'classify',
  'complete',
];

export function IngestionStatusPanel({ status, className }: IngestionStatusPanelProps) {
  if (!status) {
    return (
      <div className={clsx('rounded-lg border border-neutral-200 bg-white px-4 py-6 text-center', className)}>
        <p className="text-sm text-neutral-400">No ingestion job in progress.</p>
      </div>
    );
  }

  const isFailed = status.stage === 'failed';
  const isComplete = status.stage === 'complete';
  const stageIndex = STAGE_ORDER.indexOf(status.stage);

  return (
    <div
      role="status"
      aria-live="polite"
      data-testid="ingestion-status-panel"
      className={clsx(
        'space-y-4 rounded-lg border px-4 py-5',
        isFailed ? 'border-red-200 bg-red-50' : 'border-neutral-200 bg-white',
        className,
      )}
    >
      <div className="flex items-center gap-3">
        {isComplete ? (
          <CheckCircle2Icon className="h-5 w-5 flex-shrink-0 text-green-600" aria-hidden="true" />
        ) : isFailed ? (
          <AlertCircleIcon className="h-5 w-5 flex-shrink-0 text-red-600" aria-hidden="true" />
        ) : (
          <Spinner size="md" className="flex-shrink-0 text-primary-DEFAULT" />
        )}
        <div>
          <p className="text-sm font-semibold text-neutral-800">
            {STAGE_LABELS[status.stage]}
          </p>
          {!isFailed && (
            <p className="text-xs text-neutral-500">
              This is a background process — extracted content is draft material only and
              will be available for your review once processing completes.
            </p>
          )}
          {isFailed && status.error && (
            <p role="alert" className="text-xs text-red-700">
              {status.error}
            </p>
          )}
        </div>
      </div>

      {!isFailed && (
        <div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-neutral-200">
            <div
              className="h-full rounded-full bg-primary-DEFAULT transition-all duration-300"
              style={{ width: `${Math.min(100, Math.max(0, status.percentComplete))}%` }}
              data-testid="progress-bar-fill"
            />
          </div>
          <p className="mt-1 text-right text-xs text-neutral-500">
            {status.percentComplete}%
          </p>
        </div>
      )}

      {!isFailed && stageIndex >= 0 && (
        <ol className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-neutral-500">
          {STAGE_ORDER.map((stage, idx) => (
            <li
              key={stage}
              className={clsx(
                idx < stageIndex && 'text-primary-DEFAULT',
                idx === stageIndex && 'font-semibold text-primary-DEFAULT',
              )}
            >
              {STAGE_LABELS[stage]}
              {idx < STAGE_ORDER.length - 1 && <span className="ml-4 text-neutral-300">→</span>}
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
