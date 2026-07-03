import type { CaseState, CaseWorkspaceAction } from '@/types';

/**
 * Client-side mirror of packages/caselifecycle's allowedTransitions map
 * (see packages/caselifecycle/transition.go). Ordinary Transition moves
 * only — Reopen (closed -> active) and Archive (closed -> archived) are
 * deliberately separate, audited operations in that package, and are
 * modeled the same way here via REOPEN_FROM / ARCHIVE_FROM below.
 */
const ALLOWED_TRANSITIONS: Record<CaseState, CaseState[]> = {
  draft: ['active'],
  active: ['under_review'],
  under_review: ['closed', 'active'],
  closed: [],
  archived: [],
};

/** States Reopen is permitted from (packages/caselifecycle/reopen.go). */
const REOPEN_FROM: CaseState[] = ['closed'];

/** States Archive is permitted from (packages/caselifecycle/archive.go). */
const ARCHIVE_FROM: CaseState[] = ['closed'];

export function canTransition(from: CaseState, to: CaseState): boolean {
  return ALLOWED_TRANSITIONS[from]?.includes(to) ?? false;
}

export function canReopen(from: CaseState): boolean {
  return REOPEN_FROM.includes(from);
}

export function canArchive(from: CaseState): boolean {
  return ARCHIVE_FROM.includes(from);
}

/**
 * Client-side mirror of packages/caselifecycle's permittedActions map
 * (see packages/caselifecycle/actions.go), keyed the same way: which
 * Actions are available against a case in a given State.
 */
const PERMITTED_ACTIONS: Record<CaseState, CaseWorkspaceAction[]> = {
  draft: ['ingest_evidence', 'edit_category', 'edit_timeline', 'edit_metadata'],
  active: [
    'ingest_evidence',
    'edit_category',
    'edit_timeline',
    'generate_reasoning',
    'review_opinion',
    'edit_metadata',
  ],
  under_review: ['review_opinion', 'edit_metadata'],
  closed: [],
  archived: [],
};

export function permittedActions(state: CaseState): CaseWorkspaceAction[] {
  return PERMITTED_ACTIONS[state] ?? [];
}

export function isActionPermitted(state: CaseState, action: CaseWorkspaceAction): boolean {
  return permittedActions(state).includes(action);
}

/** Human-readable label for a CaseState, used in headers and status bars. */
export const CASE_STATE_LABELS: Record<CaseState, string> = {
  draft: 'Draft',
  active: 'Active',
  under_review: 'Under Review',
  closed: 'Closed',
  archived: 'Archived',
};

/** Tailwind badge color classes per CaseState. */
export const CASE_STATE_BADGE_CLASSES: Record<CaseState, string> = {
  draft: 'bg-neutral-100 text-neutral-700 dark:bg-neutral-700 dark:text-neutral-200',
  active: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200',
  under_review: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200',
  closed: 'bg-neutral-200 text-neutral-700 dark:bg-neutral-600 dark:text-neutral-100',
  archived: 'bg-neutral-300 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400',
};

export const CASE_WORKSPACE_ACTION_LABELS: Record<CaseWorkspaceAction, string> = {
  ingest_evidence: 'Ingest Evidence',
  edit_category: 'Edit Category',
  edit_timeline: 'Edit Timeline',
  generate_reasoning: 'Generate Reasoning',
  review_opinion: 'Review Opinion',
  edit_metadata: 'Edit Metadata',
};
