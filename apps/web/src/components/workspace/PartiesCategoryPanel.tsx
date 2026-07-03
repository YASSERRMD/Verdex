import clsx from 'clsx';
import { Card } from '@/components/ui/Card';
import type { CaseLifecycle, CaseParty } from '@/types';

export interface PartiesCategoryPanelProps {
  caseData: CaseLifecycle;
  parties: CaseParty[];
  className?: string;
}

const ROLE_LABELS: Record<CaseParty['role'], string> = {
  first_party: 'First Party',
  second_party: 'Second Party',
  third_party: 'Third Party',
};

/**
 * Renders the parties attached to a case, and its category/subcategory
 * classification, sourced from CaseLifecycle- and CaseParty-shaped data.
 */
export function PartiesCategoryPanel({ caseData, parties, className }: PartiesCategoryPanelProps) {
  return (
    <div className={clsx('space-y-6', className)}>
      <Card header={<h2 className="text-base font-semibold text-neutral-800 dark:text-white">Category</h2>}>
        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-neutral-500">Category</dt>
            <dd className="mt-1 text-sm text-neutral-800 dark:text-neutral-200">
              {caseData.categoryLabel || caseData.categoryId || 'Not yet assigned'}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-neutral-500">Subcategory</dt>
            <dd className="mt-1 text-sm text-neutral-800 dark:text-neutral-200">
              {caseData.subcategoryLabel || '—'}
            </dd>
          </div>
        </dl>
      </Card>

      <Card header={<h2 className="text-base font-semibold text-neutral-800 dark:text-white">Parties</h2>}>
        {parties.length === 0 ? (
          <p className="text-sm text-neutral-400">No parties recorded for this case yet.</p>
        ) : (
          <ul className="space-y-3">
            {parties.map((party) => (
              <li
                key={party.id}
                data-testid={`workspace-party-${party.id}`}
                className="flex items-center justify-between gap-3 rounded-lg border border-neutral-200 px-4 py-3 dark:border-neutral-700"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-neutral-800 dark:text-neutral-100">
                    {party.name || 'Unnamed party'}
                  </p>
                  {party.counsel && (
                    <p className="mt-0.5 truncate text-xs text-neutral-500">Counsel: {party.counsel}</p>
                  )}
                </div>
                <span className="flex-shrink-0 rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-DEFAULT dark:bg-primary-900/30">
                  {ROLE_LABELS[party.role]}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
