'use client';

import { useState } from 'react';
import clsx from 'clsx';
import {
  ClockIcon,
  GitBranchIcon,
  HistoryIcon,
  RotateCcwIcon,
  ScaleIcon,
  ScrollTextIcon,
} from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import type { SnapshotArtifactKind, SnapshotDiff, SnapshotEntry } from '@/types';

export interface HistoryPanelProps {
  /** Every snapshot loaded for the case, oldest first. */
  snapshots: SnapshotEntry[];
  /**
   * Called to compute the diff between two snapshot IDs. Rejects/throws
   * are surfaced as an inline error by this panel.
   */
  onDiff: (snapshotAId: string, snapshotBId: string) => Promise<SnapshotDiff>;
  /**
   * Called to restore the case's metadata to a prior snapshot. Only
   * offered for 'case-metadata' snapshots — see
   * packages/caseversioning.ErrNotRestorable.
   */
  onRestore: (snapshotId: string) => Promise<void> | void;
  className?: string;
}

const ARTIFACT_LABELS: Record<SnapshotArtifactKind, string> = {
  'case-metadata': 'Case metadata',
  tree: 'Reasoning tree',
  evidence: 'Evidence',
  opinion: 'Draft opinion',
};

const ARTIFACT_ICONS: Record<SnapshotArtifactKind, typeof ScrollTextIcon> = {
  'case-metadata': ScrollTextIcon,
  tree: GitBranchIcon,
  evidence: ScaleIcon,
  opinion: HistoryIcon,
};

const ARTIFACT_BADGE_CLASSES: Record<SnapshotArtifactKind, string> = {
  'case-metadata': 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  tree: 'bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
  evidence: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  opinion: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
};

function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

function DiffView({ diff }: { diff: SnapshotDiff }) {
  if (diff.identical) {
    return (
      <p className="text-xs text-neutral-500" data-testid="history-diff-identical">
        No differences between these versions.
      </p>
    );
  }

  if (diff.fieldChanges && diff.fieldChanges.length > 0) {
    return (
      <table className="w-full text-left text-xs" data-testid="history-diff-fields">
        <thead>
          <tr className="text-neutral-400">
            <th className="pb-1 pr-3 font-medium">Field</th>
            <th className="pb-1 pr-3 font-medium">Before</th>
            <th className="pb-1 font-medium">After</th>
          </tr>
        </thead>
        <tbody>
          {diff.fieldChanges.map((fc) => (
            <tr key={fc.field} className="border-t border-neutral-100 dark:border-neutral-700">
              <td className="py-1 pr-3 font-mono text-neutral-600 dark:text-neutral-300">{fc.field}</td>
              <td className="py-1 pr-3 text-red-600 dark:text-red-400">{fc.before || '—'}</td>
              <td className="py-1 text-emerald-600 dark:text-emerald-400">{fc.after || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    );
  }

  return (
    <p className="text-xs text-neutral-600 dark:text-neutral-300" data-testid="history-diff-reference">
      Revision reference changed: <span className="font-mono">{diff.revisionRefBefore || '—'}</span>{' '}
      &rarr; <span className="font-mono">{diff.revisionRefAfter || '—'}</span>
    </p>
  );
}

function SnapshotRow({
  snapshot,
  previous,
  onDiff,
  onRestore,
}: {
  snapshot: SnapshotEntry;
  previous?: SnapshotEntry;
  onDiff: (snapshotAId: string, snapshotBId: string) => Promise<SnapshotDiff>;
  onRestore: (snapshotId: string) => Promise<void> | void;
}) {
  const [diff, setDiff] = useState<SnapshotDiff | null>(null);
  const [diffBusy, setDiffBusy] = useState(false);
  const [diffError, setDiffError] = useState<string | null>(null);
  const [restoreBusy, setRestoreBusy] = useState(false);
  const [restoreError, setRestoreError] = useState<string | null>(null);

  const Icon = ARTIFACT_ICONS[snapshot.artifactKind];

  const handleDiff = async () => {
    if (!previous) return;
    setDiffBusy(true);
    setDiffError(null);
    try {
      const result = await onDiff(previous.id, snapshot.id);
      setDiff(result);
    } catch (err) {
      setDiffError(err instanceof Error ? err.message : 'Failed to compute diff.');
    } finally {
      setDiffBusy(false);
    }
  };

  const handleRestore = async () => {
    setRestoreBusy(true);
    setRestoreError(null);
    try {
      await onRestore(snapshot.id);
    } catch (err) {
      setRestoreError(err instanceof Error ? err.message : 'Failed to restore this version.');
    } finally {
      setRestoreBusy(false);
    }
  };

  const canRestore = snapshot.artifactKind === 'case-metadata' && !snapshot.restoredFromId;

  return (
    <li
      data-testid={`history-snapshot-${snapshot.id}`}
      className="rounded-lg border border-neutral-200 px-3 py-2.5 text-sm dark:border-neutral-700"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={clsx(
            'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium',
            ARTIFACT_BADGE_CLASSES[snapshot.artifactKind],
          )}
        >
          <Icon className="h-3 w-3" aria-hidden="true" />
          {ARTIFACT_LABELS[snapshot.artifactKind]}
        </span>
        {snapshot.label && (
          <span className="text-sm font-medium text-neutral-800 dark:text-white">{snapshot.label}</span>
        )}
        {snapshot.restoredFromId && (
          <span
            data-testid={`history-restore-badge-${snapshot.id}`}
            className="inline-flex items-center gap-1 rounded-full bg-neutral-100 px-2 py-0.5 text-xs font-medium text-neutral-600 dark:bg-neutral-700 dark:text-neutral-300"
          >
            <RotateCcwIcon className="h-3 w-3" aria-hidden="true" />
            Restore
          </span>
        )}
      </div>

      <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-neutral-500">
        <ClockIcon className="h-3 w-3" aria-hidden="true" />
        <span>{formatTimestamp(snapshot.createdAt)}</span>
        <span aria-hidden="true">·</span>
        <span>{snapshot.createdByName ?? snapshot.createdBy}</span>
        {snapshot.reason && (
          <>
            <span aria-hidden="true">·</span>
            <span>{snapshot.reason}</span>
          </>
        )}
        {snapshot.artifactRevisionRef && (
          <>
            <span aria-hidden="true">·</span>
            <span className="font-mono">rev {snapshot.artifactRevisionRef}</span>
          </>
        )}
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-2">
        {previous && (
          <Button type="button" variant="ghost" size="sm" loading={diffBusy} onClick={handleDiff}>
            Diff vs. previous
          </Button>
        )}
        {canRestore && (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            loading={restoreBusy}
            leftIcon={<RotateCcwIcon className="h-3.5 w-3.5" />}
            onClick={handleRestore}
          >
            Restore this version
          </Button>
        )}
      </div>

      {diffError && (
        <p role="alert" className="mt-2 text-xs text-red-600 dark:text-red-400">
          {diffError}
        </p>
      )}
      {restoreError && (
        <p role="alert" className="mt-2 text-xs text-red-600 dark:text-red-400">
          {restoreError}
        </p>
      )}
      {diff && (
        <div className="mt-3 rounded-md bg-neutral-50 p-2.5 dark:bg-neutral-900/40" data-testid={`history-diff-${snapshot.id}`}>
          <DiffView diff={diff} />
        </div>
      )}
    </li>
  );
}

/**
 * The case-level "History" panel: a chronological timeline of every
 * version snapshot recorded for the case (case metadata, reasoning
 * tree, evidence, and draft opinion), each with a "Diff vs. previous"
 * action and, for case-metadata snapshots, a "Restore this version"
 * action. Reachable from the case workspace as its own tab (see
 * WorkspaceTabs). Backed by packages/caseversioning — see that
 * package's doc/case-versioning.md for the full data model and how it
 * composes (rather than duplicates) tree-revision and
 * metadata-version tracking that already exists elsewhere. See
 * apps/web/docs/case-history-ui.md for the UI-specific data shape.
 */
export function HistoryPanel({ snapshots, onDiff, onRestore, className }: HistoryPanelProps) {
  const ordered = [...snapshots].sort(
    (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
  );

  return (
    <div className={clsx('space-y-4', className)}>
      <Card
        header={
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold text-neutral-800 dark:text-white">History</h2>
            <span className="text-xs text-neutral-500">
              {ordered.length} {ordered.length === 1 ? 'version' : 'versions'}
            </span>
          </div>
        }
      >
        {ordered.length === 0 ? (
          <div
            data-testid="history-empty-state"
            className="flex flex-col items-center justify-center gap-3 py-16 text-center"
          >
            <HistoryIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
            <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">No version history yet</p>
            <p className="max-w-sm text-xs text-neutral-400">
              Every change to this case&apos;s metadata, reasoning tree, evidence, and draft opinion will
              appear here as it happens.
            </p>
          </div>
        ) : (
          <ul data-testid="history-timeline" className="space-y-3">
            {ordered
              .slice()
              .reverse()
              .map((snapshot, idx) => {
                // ordered is oldest-first; reversed for display
                // (newest-first), so the "previous" version for a diff
                // is the next element after reversal.
                const previous = ordered[ordered.length - 1 - idx - 1];
                return (
                  <SnapshotRow
                    key={snapshot.id}
                    snapshot={snapshot}
                    previous={previous}
                    onDiff={onDiff}
                    onRestore={onRestore}
                  />
                );
              })}
          </ul>
        )}
      </Card>
    </div>
  );
}
