/**
 * @jest-environment jsdom
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { HistoryPanel } from '@/components/workspace/HistoryPanel';
import type { SnapshotDiff, SnapshotEntry } from '@/types';

const METADATA_BEFORE: SnapshotEntry = {
  id: 'snap-1',
  caseId: 'case-1',
  artifactKind: 'case-metadata',
  artifactRevisionRef: '1',
  createdBy: 'user-1',
  createdByName: 'Judge Doe',
  reason: 'initial',
  label: 'Original',
  createdAt: '2026-01-01T10:00:00Z',
};

const METADATA_AFTER: SnapshotEntry = {
  id: 'snap-2',
  caseId: 'case-1',
  artifactKind: 'case-metadata',
  artifactRevisionRef: '2',
  createdBy: 'user-1',
  createdByName: 'Judge Doe',
  reason: 'manual edit',
  label: 'Amended',
  createdAt: '2026-01-02T10:00:00Z',
};

const TREE_SNAPSHOT: SnapshotEntry = {
  id: 'snap-3',
  caseId: 'case-1',
  artifactKind: 'tree',
  artifactRevisionRef: '3',
  createdBy: 'user-2',
  createdByName: 'Clerk Smith',
  reason: 'tree assembled',
  createdAt: '2026-01-03T10:00:00Z',
};

describe('HistoryPanel', () => {
  it('shows an empty state when there are no snapshots', () => {
    render(<HistoryPanel snapshots={[]} onDiff={jest.fn()} onRestore={jest.fn()} />);
    expect(screen.getByTestId('history-empty-state')).toBeInTheDocument();
  });

  it('renders every snapshot newest-first with its artifact kind and attribution', () => {
    render(
      <HistoryPanel
        snapshots={[METADATA_BEFORE, METADATA_AFTER, TREE_SNAPSHOT]}
        onDiff={jest.fn()}
        onRestore={jest.fn()}
      />,
    );

    const timeline = screen.getByTestId('history-timeline');
    const items = within(timeline).getAllByRole('listitem');
    expect(items).toHaveLength(3);
    // Newest first: TREE_SNAPSHOT (Jan 3), then METADATA_AFTER (Jan 2),
    // then METADATA_BEFORE (Jan 1).
    expect(items[0]).toHaveAttribute('data-testid', `history-snapshot-${TREE_SNAPSHOT.id}`);
    expect(items[2]).toHaveAttribute('data-testid', `history-snapshot-${METADATA_BEFORE.id}`);

    expect(within(items[0]).getByText('Reasoning tree')).toBeInTheDocument();
    expect(within(items[0]).getByText('Clerk Smith')).toBeInTheDocument();
    expect(within(items[1]).getByText('Amended')).toBeInTheDocument();
  });

  it('computes and displays a field-level diff against the previous snapshot', async () => {
    const diff: SnapshotDiff = {
      caseId: 'case-1',
      artifactKind: 'case-metadata',
      snapshotAId: METADATA_BEFORE.id,
      snapshotBId: METADATA_AFTER.id,
      fieldChanges: [{ field: 'title', before: 'Doe v. Acme', after: 'Doe v. Acme (Amended)' }],
      revisionRefChanged: true,
      revisionRefBefore: '1',
      revisionRefAfter: '2',
      identical: false,
    };
    const onDiff = jest.fn().mockResolvedValue(diff);

    render(
      <HistoryPanel snapshots={[METADATA_BEFORE, METADATA_AFTER]} onDiff={onDiff} onRestore={jest.fn()} />,
    );

    const newestRow = screen.getByTestId(`history-snapshot-${METADATA_AFTER.id}`);
    fireEvent.click(within(newestRow).getByRole('button', { name: /diff vs\. previous/i }));

    await waitFor(() => expect(onDiff).toHaveBeenCalledWith(METADATA_BEFORE.id, METADATA_AFTER.id));
    await waitFor(() =>
      expect(within(newestRow).getByTestId('history-diff-fields')).toBeInTheDocument(),
    );
    expect(within(newestRow).getByText('title')).toBeInTheDocument();
    expect(within(newestRow).getByText('Doe v. Acme')).toBeInTheDocument();
    expect(within(newestRow).getByText('Doe v. Acme (Amended)')).toBeInTheDocument();
  });

  it('offers Restore only for case-metadata snapshots, not tree/evidence/opinion', () => {
    render(
      <HistoryPanel
        snapshots={[METADATA_BEFORE, TREE_SNAPSHOT]}
        onDiff={jest.fn()}
        onRestore={jest.fn()}
      />,
    );

    const metadataRow = screen.getByTestId(`history-snapshot-${METADATA_BEFORE.id}`);
    const treeRow = screen.getByTestId(`history-snapshot-${TREE_SNAPSHOT.id}`);

    expect(within(metadataRow).getByRole('button', { name: /restore this version/i })).toBeInTheDocument();
    expect(
      within(treeRow).queryByRole('button', { name: /restore this version/i }),
    ).not.toBeInTheDocument();
  });

  it('calls onRestore with the snapshot id when Restore is clicked', async () => {
    const onRestore = jest.fn().mockResolvedValue(undefined);
    render(<HistoryPanel snapshots={[METADATA_BEFORE]} onDiff={jest.fn()} onRestore={onRestore} />);

    const row = screen.getByTestId(`history-snapshot-${METADATA_BEFORE.id}`);
    fireEvent.click(within(row).getByRole('button', { name: /restore this version/i }));

    await waitFor(() => expect(onRestore).toHaveBeenCalledWith(METADATA_BEFORE.id));
  });

  it('does not offer Restore on a snapshot that is itself the record of a prior restore', () => {
    const restoreSnapshot: SnapshotEntry = {
      ...METADATA_AFTER,
      id: 'snap-restore',
      restoredFromId: METADATA_BEFORE.id,
    };
    render(<HistoryPanel snapshots={[restoreSnapshot]} onDiff={jest.fn()} onRestore={jest.fn()} />);

    const row = screen.getByTestId(`history-snapshot-${restoreSnapshot.id}`);
    expect(within(row).getByTestId(`history-restore-badge-${restoreSnapshot.id}`)).toBeInTheDocument();
    expect(within(row).queryByRole('button', { name: /restore this version/i })).not.toBeInTheDocument();
  });
});
