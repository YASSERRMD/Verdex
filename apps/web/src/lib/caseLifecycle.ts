import type { CaseState, CaseWorkspaceAction, SupportedLanguage } from '@/types';
import { translate, type TranslationKey } from '@/lib/i18n/strings';

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

/**
 * Human-readable label for a CaseState, used in headers and status
 * bars. Kept as a static English-only export for backward
 * compatibility with existing callers; new locale-aware callers should
 * use caseStateLabel(state, locale) instead (Phase 090, task 1/6),
 * which resolves through the same externalized strings catalog these
 * values are copied from (src/lib/i18n/strings.ts).
 */
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

/**
 * Static English-only export kept for backward compatibility; new
 * locale-aware callers should use caseWorkspaceActionLabel(action,
 * locale) instead. See CASE_STATE_LABELS's doc comment.
 */
export const CASE_WORKSPACE_ACTION_LABELS: Record<CaseWorkspaceAction, string> = {
  ingest_evidence: 'Ingest Evidence',
  edit_category: 'Edit Category',
  edit_timeline: 'Edit Timeline',
  generate_reasoning: 'Generate Reasoning',
  review_opinion: 'Review Opinion',
  edit_metadata: 'Edit Metadata',
};

/** Maps each CaseState to its externalized-strings TranslationKey. */
const CASE_STATE_KEYS: Record<CaseState, TranslationKey> = {
  draft: 'case_status.draft',
  active: 'case_status.active',
  under_review: 'case_status.under_review',
  closed: 'case_status.closed',
  archived: 'case_status.archived',
};

/** Maps each CaseWorkspaceAction to its externalized-strings TranslationKey. */
const CASE_WORKSPACE_ACTION_KEYS: Record<CaseWorkspaceAction, TranslationKey> = {
  ingest_evidence: 'action.ingest_evidence',
  edit_category: 'action.edit_category',
  edit_timeline: 'action.edit_timeline',
  generate_reasoning: 'action.generate_reasoning',
  review_opinion: 'action.review_opinion',
  edit_metadata: 'action.edit_metadata',
};

/**
 * caseStateLabel is CASE_STATE_LABELS's locale-aware counterpart
 * (Phase 090): the same label, translated for locale via
 * src/lib/i18n/strings.ts, falling back to English for any locale
 * missing a translation.
 */
export function caseStateLabel(state: CaseState, locale: SupportedLanguage): string {
  return translate(locale, CASE_STATE_KEYS[state]);
}

/**
 * caseWorkspaceActionLabel is CASE_WORKSPACE_ACTION_LABELS's
 * locale-aware counterpart (Phase 090).
 */
export function caseWorkspaceActionLabel(action: CaseWorkspaceAction, locale: SupportedLanguage): string {
  return translate(locale, CASE_WORKSPACE_ACTION_KEYS[action]);
}
