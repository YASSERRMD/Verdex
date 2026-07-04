'use client';

import { useState } from 'react';
import clsx from 'clsx';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import {
  CASE_STATE_BADGE_CLASSES,
  caseStateLabel,
  canArchive,
  canReopen,
  canTransition,
} from '@/lib/caseLifecycle';
import { useLocale } from '@/lib/i18n/LocaleContext';
import type { CaseState } from '@/types';

export interface StatusActionsBarProps {
  state: CaseState;
  /** Called with the requested target state when a transition is chosen. */
  onTransition?: (toState: CaseState) => void;
  /** Called with a justification string when Reopen is chosen. */
  onReopen?: (justification: string) => void;
  /** Called with an optional reason when Archive is chosen. */
  onArchive?: (reason?: string) => void;
  busy?: boolean;
  className?: string;
}

const NEXT_STATE_LABEL: Record<CaseState, string> = {
  draft: 'Activate',
  active: 'Submit for Review',
  under_review: 'Close Case',
  closed: 'Reopen',
  archived: '',
};

/**
 * Shows the case's current lifecycle state and the state-appropriate
 * actions available against it, mirroring packages/caselifecycle's
 * allowedTransitions/permittedActions semantics exactly (draft -> active
 * -> under_review -> {closed, active}; closed -> archived via the
 * separate Archive operation; archived is terminal).
 */
export function StatusActionsBar({
  state,
  onTransition,
  onReopen,
  onArchive,
  busy = false,
  className,
}: StatusActionsBarProps) {
  const [reopenReason, setReopenReason] = useState('');
  const [showReopenForm, setShowReopenForm] = useState(false);
  const { locale, direction, t } = useLocale();

  const ALL_STATES: CaseState[] = ['draft', 'active', 'under_review', 'closed', 'archived'];
  const forwardTransitions: CaseState[] = ALL_STATES.filter((to) => canTransition(state, to));

  const canSubmitReopen = reopenReason.trim().length > 0;

  return (
    <Card className={clsx(className)} padding="md" dir={direction}>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="text-xs font-medium uppercase tracking-wide text-neutral-500">
            {t('common.status')}
          </span>
          <span
            data-testid="status-bar-state-badge"
            className={clsx(
              'inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-wide',
              CASE_STATE_BADGE_CLASSES[state],
            )}
          >
            {caseStateLabel(state, locale)}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {forwardTransitions.map((to) => (
            <Button
              key={to}
              variant={to === 'closed' ? 'secondary' : 'primary'}
              size="sm"
              loading={busy}
              onClick={() => onTransition?.(to)}
            >
              {to === 'active' && state === 'under_review'
                ? 'Send Back to Active'
                : NEXT_STATE_LABEL[state]}
            </Button>
          ))}

          {canReopen(state) && !showReopenForm && (
            <Button variant="secondary" size="sm" onClick={() => setShowReopenForm(true)}>
              Reopen
            </Button>
          )}

          {canArchive(state) && (
            <Button
              variant="ghost"
              size="sm"
              loading={busy}
              onClick={() => onArchive?.()}
            >
              Archive
            </Button>
          )}

          {forwardTransitions.length === 0 && !canReopen(state) && !canArchive(state) && (
            <span className="text-xs text-neutral-400">{t('common.no_actions_available')}</span>
          )}
        </div>
      </div>

      {showReopenForm && (
        <div className="mt-4 space-y-2 border-t border-neutral-200 pt-4 dark:border-neutral-700">
          <label htmlFor="reopen-justification" className="text-sm font-medium text-neutral-700">
            Justification for reopening (required)
          </label>
          <textarea
            id="reopen-justification"
            data-testid="reopen-justification-input"
            className="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:border-primary-DEFAULT focus:outline-none focus:ring-1 focus:ring-primary-DEFAULT"
            rows={2}
            value={reopenReason}
            onChange={(e) => setReopenReason(e.target.value)}
          />
          <div className="flex justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setShowReopenForm(false);
                setReopenReason('');
              }}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              disabled={!canSubmitReopen}
              loading={busy}
              onClick={() => {
                onReopen?.(reopenReason.trim());
                setShowReopenForm(false);
                setReopenReason('');
              }}
            >
              Confirm Reopen
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}
