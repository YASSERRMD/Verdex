import clsx from 'clsx';
import { CASE_STATE_BADGE_CLASSES, CASE_STATE_LABELS } from '@/lib/caseLifecycle';
import type { CaseLifecycle } from '@/types';

export interface CaseHeaderProps {
  caseData: CaseLifecycle;
  className?: string;
}

/**
 * Case workspace header: title, lifecycle state badge, category, and
 * jurisdiction. Sourced directly from a CaseLifecycle record shaped like
 * packages/caselifecycle.Case.
 */
export function CaseHeader({ caseData, className }: CaseHeaderProps) {
  const category = caseData.categoryLabel || caseData.categoryId || 'Uncategorized';
  const jurisdiction = caseData.jurisdictionName || caseData.jurisdictionId;

  return (
    <header className={clsx('space-y-2', className)}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="truncate text-2xl font-bold text-neutral-900 dark:text-white">
            {caseData.title}
          </h1>
          {caseData.reference && (
            <p className="mt-0.5 text-sm text-neutral-500">Ref. {caseData.reference}</p>
          )}
        </div>
        <span
          data-testid="case-state-badge"
          className={clsx(
            'inline-flex flex-shrink-0 items-center rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-wide',
            CASE_STATE_BADGE_CLASSES[caseData.state],
          )}
        >
          {CASE_STATE_LABELS[caseData.state]}
        </span>
      </div>

      <dl className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-neutral-600 dark:text-neutral-300">
        <div className="flex items-center gap-1.5">
          <dt className="font-medium text-neutral-500">Category</dt>
          <dd>
            {category}
            {caseData.subcategoryLabel && <span> / {caseData.subcategoryLabel}</span>}
          </dd>
        </div>
        <div className="flex items-center gap-1.5">
          <dt className="font-medium text-neutral-500">Jurisdiction</dt>
          <dd>{jurisdiction}</dd>
        </div>
      </dl>
    </header>
  );
}
