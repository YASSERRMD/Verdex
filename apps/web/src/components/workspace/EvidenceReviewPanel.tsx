'use client';

import { useMemo, useState } from 'react';
import clsx from 'clsx';
import { apiFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { Button } from '@/components/ui/Button';
import type { EvidenceAuditEntry, EvidenceSegment, EvidenceType, PartyRole } from '@/types';

export interface EvidenceReviewPanelProps {
  segments: EvidenceSegment[];
  onSegmentsChange?: (segments: EvidenceSegment[]) => void;
  className?: string;
}

const EVIDENCE_TYPE_LABELS: Record<EvidenceType, string> = {
  testimony: 'Testimony',
  documentary: 'Documentary',
  statute_citation: 'Statute Citation',
  argument: 'Argument',
  other: 'Other',
};

const EVIDENCE_TYPE_BADGE_CLASSES: Record<EvidenceType, string> = {
  testimony: 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  documentary: 'bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
  statute_citation: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  argument: 'bg-teal-50 text-teal-700 dark:bg-teal-900/30 dark:text-teal-300',
  other: 'bg-neutral-100 text-neutral-600 dark:bg-neutral-700 dark:text-neutral-300',
};

const EVIDENCE_TYPE_OPTIONS: { value: EvidenceType; label: string }[] = [
  { value: 'testimony', label: 'Testimony' },
  { value: 'documentary', label: 'Documentary' },
  { value: 'statute_citation', label: 'Statute Citation' },
  { value: 'argument', label: 'Argument' },
  { value: 'other', label: 'Other' },
];

const PARTY_LABELS: Record<PartyRole, string> = {
  first_party: 'First Party',
  second_party: 'Second Party',
  third_party: 'Third Party',
  unattributed: 'Unattributed',
};

const PARTY_OPTIONS: { value: PartyRole; label: string }[] = [
  { value: 'first_party', label: 'First Party' },
  { value: 'second_party', label: 'Second Party' },
  { value: 'third_party', label: 'Third Party' },
  { value: 'unattributed', label: 'Unattributed' },
];

const TYPE_FILTER_OPTIONS = [{ value: '', label: 'All types' }, ...EVIDENCE_TYPE_OPTIONS];
const PARTY_FILTER_OPTIONS = [{ value: '', label: 'All parties' }, ...PARTY_OPTIONS];
const DISPUTE_FILTER_OPTIONS = [
  { value: '', label: 'All segments' },
  { value: 'disputed', label: 'Disputed only' },
  { value: 'undisputed', label: 'Undisputed only' },
];

function newAuditId(): string {
  return `audit-${Math.random().toString(36).slice(2, 10)}`;
}

/**
 * Case-workspace evidence review UI: an ongoing, in-case-lifecycle
 * counterpart to Phase 030's one-time ingestion review
 * (components/ingestion/ClassificationCorrectionPanel and
 * PartyTimelineEditor). Reuses those components' patterns — Select-driven
 * inline correction, a stub apiFetch persistence call, client-side error
 * surfacing — but adds list-level concerns Phase 030 does not need:
 * search/filter, multi-select bulk tagging, dispute tracking, and a visible
 * per-segment change-audit trail. See docs/evidence-review.md.
 */
export function EvidenceReviewPanel({ segments, onSegmentsChange, className }: EvidenceReviewPanelProps) {
  const [items, setItems] = useState<EvidenceSegment[]>(segments);
  const [audit, setAudit] = useState<EvidenceAuditEntry[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [expandedAudit, setExpandedAudit] = useState<Set<string>>(new Set());

  const [searchText, setSearchText] = useState('');
  const [typeFilter, setTypeFilter] = useState<EvidenceType | ''>('');
  const [partyFilter, setPartyFilter] = useState<PartyRole | ''>('');
  const [disputeFilter, setDisputeFilter] = useState<'' | 'disputed' | 'undisputed'>('');

  const [bulkType, setBulkType] = useState<EvidenceType | ''>('');
  const [bulkParty, setBulkParty] = useState<PartyRole | ''>('');

  const filtered = useMemo(() => {
    const query = searchText.trim().toLowerCase();
    return items.filter((seg) => {
      if (typeFilter && seg.type !== typeFilter) return false;
      if (partyFilter && seg.party !== partyFilter) return false;
      if (disputeFilter === 'disputed' && !seg.disputed) return false;
      if (disputeFilter === 'undisputed' && seg.disputed) return false;
      if (query && !seg.text.toLowerCase().includes(query)) return false;
      return true;
    });
  }, [items, searchText, typeFilter, partyFilter, disputeFilter]);

  const hasActiveFilters = !!(searchText || typeFilter || partyFilter || disputeFilter);

  const clearFilters = () => {
    setSearchText('');
    setTypeFilter('');
    setPartyFilter('');
    setDisputeFilter('');
  };

  const updateSegment = (
    segmentId: string,
    patch: Partial<Pick<EvidenceSegment, 'type' | 'party' | 'disputed'>>,
    field: EvidenceAuditEntry['field'],
    previousValue: string,
    newValue: string,
  ) => {
    setItems((prev) => {
      const updated = prev.map((seg) => (seg.id === segmentId ? { ...seg, ...patch } : seg));
      onSegmentsChange?.(updated);
      return updated;
    });
    setAudit((prev) => [
      {
        id: newAuditId(),
        segmentId,
        field,
        previousValue,
        newValue,
        actor: 'Current Reviewer',
        occurredAt: new Date().toISOString(),
      },
      ...prev,
    ]);
  };

  const persistCorrection = async (
    segmentId: string,
    body: Record<string, unknown>,
    onSuccess: () => void,
  ) => {
    setErrors((e) => ({ ...e, [segmentId]: '' }));
    try {
      await apiFetch(`/api/v1/segments/${segmentId}/classification`, {
        method: 'PUT',
        body: JSON.stringify(body),
      });
      onSuccess();
    } catch (err) {
      setErrors((e) => ({
        ...e,
        [segmentId]: err instanceof Error ? err.message : 'Failed to save correction.',
      }));
    }
  };

  const handleTypeChange = (segment: EvidenceSegment, value: EvidenceType) => {
    if (value === segment.type) return;
    void persistCorrection(segment.id, { type: value }, () =>
      updateSegment(segment.id, { type: value }, 'type', segment.type, value),
    );
  };

  const handlePartyChange = (segment: EvidenceSegment, value: PartyRole) => {
    if (value === segment.party) return;
    void persistCorrection(segment.id, { party: value }, () =>
      updateSegment(segment.id, { party: value }, 'party', segment.party, value),
    );
  };

  const handleDisputeToggle = (segment: EvidenceSegment) => {
    const next = !segment.disputed;
    void persistCorrection(segment.id, { disputed: next }, () =>
      updateSegment(
        segment.id,
        { disputed: next },
        'disputed',
        segment.disputed ? 'disputed' : 'undisputed',
        next ? 'disputed' : 'undisputed',
      ),
    );
  };

  const toggleSelected = (segmentId: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(segmentId)) {
        next.delete(segmentId);
      } else {
        next.add(segmentId);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    setSelected((prev) => {
      if (filtered.every((seg) => prev.has(seg.id)) && filtered.length > 0) {
        const next = new Set(prev);
        filtered.forEach((seg) => next.delete(seg.id));
        return next;
      }
      const next = new Set(prev);
      filtered.forEach((seg) => next.add(seg.id));
      return next;
    });
  };

  const applyBulkTagging = async () => {
    const targets = items.filter((seg) => selected.has(seg.id));
    for (const segment of targets) {
      if (bulkType && bulkType !== segment.type) {
        await persistCorrection(segment.id, { type: bulkType }, () =>
          updateSegment(segment.id, { type: bulkType }, 'type', segment.type, bulkType),
        );
      }
      if (bulkParty && bulkParty !== segment.party) {
        await persistCorrection(segment.id, { party: bulkParty }, () =>
          updateSegment(segment.id, { party: bulkParty }, 'party', segment.party, bulkParty),
        );
      }
    }
  };

  const toggleAuditFor = (segmentId: string) => {
    setExpandedAudit((prev) => {
      const next = new Set(prev);
      if (next.has(segmentId)) {
        next.delete(segmentId);
      } else {
        next.add(segmentId);
      }
      return next;
    });
  };

  const allFilteredSelected = filtered.length > 0 && filtered.every((seg) => selected.has(seg.id));
  const disputedCount = items.filter((seg) => seg.disputed).length;

  return (
    <div className={clsx('space-y-6', className)}>
      <Card
        header={<h2 className="text-base font-semibold text-neutral-800 dark:text-white">Evidence Review</h2>}
      >
        <div className="space-y-4">
          <p className="text-sm text-neutral-500">
            Review and correct extracted evidence segments for this case. Corrections take
            precedence over the automated classification and are recorded below for audit.
          </p>

          {items.length > 0 && (
            <p data-testid="evidence-summary" className="text-xs font-medium text-neutral-500">
              {items.length} segment{items.length === 1 ? '' : 's'} total
              {disputedCount > 0 && (
                <span className="ml-1 text-red-600">
                  · {disputedCount} disputed
                </span>
              )}
            </p>
          )}

          <div data-testid="evidence-filters" className="grid grid-cols-1 gap-3 sm:grid-cols-4">
            <Input
              label="Search"
              placeholder="Search segment text…"
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              aria-label="Search evidence segments"
            />
            <Select
              id="filter-type"
              label="Type"
              value={typeFilter}
              options={TYPE_FILTER_OPTIONS}
              onChange={(e) => setTypeFilter(e.target.value as EvidenceType | '')}
            />
            <Select
              id="filter-party"
              label="Party"
              value={partyFilter}
              options={PARTY_FILTER_OPTIONS}
              onChange={(e) => setPartyFilter(e.target.value as PartyRole | '')}
            />
            <Select
              id="filter-disputed"
              label="Dispute status"
              value={disputeFilter}
              options={DISPUTE_FILTER_OPTIONS}
              onChange={(e) => setDisputeFilter(e.target.value as '' | 'disputed' | 'undisputed')}
            />
          </div>

          {hasActiveFilters && (
            <div className="flex items-center justify-between text-xs text-neutral-500">
              <span>
                Showing {filtered.length} of {items.length} segments
              </span>
              <button
                type="button"
                onClick={clearFilters}
                className="font-medium text-primary-DEFAULT hover:underline"
              >
                Clear filters
              </button>
            </div>
          )}

          {items.length === 0 ? (
            <p className="text-sm text-neutral-400">No evidence segments recorded for this case yet.</p>
          ) : (
            <>
              <div
                data-testid="bulk-tagging-bar"
                className="flex flex-wrap items-end gap-3 rounded-lg border border-neutral-200 bg-neutral-50 px-4 py-3 dark:border-neutral-700 dark:bg-neutral-900/40"
              >
                <label className="flex items-center gap-2 text-sm text-neutral-600 dark:text-neutral-300">
                  <input
                    type="checkbox"
                    aria-label="Select all filtered segments"
                    checked={allFilteredSelected}
                    onChange={toggleSelectAll}
                    className="h-4 w-4 rounded border-neutral-300"
                  />
                  Select all ({selected.size} selected)
                </label>
                <Select
                  label="Bulk set type"
                  value={bulkType}
                  options={[{ value: '', label: 'No change' }, ...EVIDENCE_TYPE_OPTIONS]}
                  onChange={(e) => setBulkType(e.target.value as EvidenceType | '')}
                  className="min-w-[9rem]"
                />
                <Select
                  label="Bulk set party"
                  value={bulkParty}
                  options={[{ value: '', label: 'No change' }, ...PARTY_OPTIONS]}
                  onChange={(e) => setBulkParty(e.target.value as PartyRole | '')}
                  className="min-w-[9rem]"
                />
                <Button
                  type="button"
                  size="sm"
                  disabled={selected.size === 0 || (!bulkType && !bulkParty)}
                  onClick={() => void applyBulkTagging()}
                >
                  Apply to {selected.size} selected
                </Button>
              </div>

              {filtered.length === 0 ? (
                <p className="text-sm text-neutral-400">No segments match the current filters.</p>
              ) : (
                <ul className="space-y-3">
                  {filtered.map((seg) => {
                    const segmentAudit = audit.filter((a) => a.segmentId === seg.id);
                    const auditOpen = expandedAudit.has(seg.id);

                    return (
                      <li
                        key={seg.id}
                        data-testid={`evidence-review-segment-${seg.id}`}
                        className={clsx(
                          'rounded-lg border px-4 py-3',
                          seg.disputed
                            ? 'border-red-300 bg-red-50/40 dark:border-red-800 dark:bg-red-900/10'
                            : 'border-neutral-200 dark:border-neutral-700',
                        )}
                      >
                        <div className="flex items-start gap-3">
                          <input
                            type="checkbox"
                            aria-label={`Select segment ${seg.id}`}
                            checked={selected.has(seg.id)}
                            onChange={() => toggleSelected(seg.id)}
                            className="mt-1 h-4 w-4 flex-shrink-0 rounded border-neutral-300"
                          />
                          <div className="min-w-0 flex-1 space-y-3">
                            <div className="flex items-start justify-between gap-3">
                              <p className="text-sm text-neutral-800 dark:text-neutral-200">{seg.text}</p>
                              <span
                                data-testid={`evidence-type-badge-${seg.id}`}
                                className={clsx(
                                  'flex-shrink-0 rounded-full px-2.5 py-1 text-xs font-medium',
                                  EVIDENCE_TYPE_BADGE_CLASSES[seg.type],
                                )}
                              >
                                {EVIDENCE_TYPE_LABELS[seg.type]}
                              </span>
                            </div>

                            <p className="text-xs text-neutral-400">
                              {seg.sourceFileName && <span>{seg.sourceFileName} · </span>}
                              Source span [{seg.sourceSpan.start}–{seg.sourceSpan.end}] · Confidence{' '}
                              {Math.round(seg.confidence * 100)}%
                            </p>

                            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                              <Select
                                id={`evidence-type-${seg.id}`}
                                label="Evidence Type"
                                value={seg.type}
                                options={EVIDENCE_TYPE_OPTIONS}
                                onChange={(e) => handleTypeChange(seg, e.target.value as EvidenceType)}
                              />
                              <Select
                                id={`evidence-party-${seg.id}`}
                                label="Party"
                                value={seg.party}
                                options={PARTY_OPTIONS}
                                onChange={(e) => handlePartyChange(seg, e.target.value as PartyRole)}
                              />
                              <div className="flex flex-col justify-end gap-1">
                                <span className="text-sm font-medium text-neutral-700 dark:text-neutral-300">
                                  Dispute status
                                </span>
                                <Button
                                  type="button"
                                  variant={seg.disputed ? 'danger' : 'secondary'}
                                  size="sm"
                                  onClick={() => handleDisputeToggle(seg)}
                                >
                                  {seg.disputed ? 'Disputed' : 'Undisputed'}
                                </Button>
                              </div>
                            </div>

                            <div className="flex items-center justify-between text-xs">
                              <span className="text-neutral-500">
                                Attributed to {PARTY_LABELS[seg.party]}
                              </span>
                              {errors[seg.id] && (
                                <span role="alert" className="text-red-600">
                                  {errors[seg.id]}
                                </span>
                              )}
                            </div>

                            <div>
                              <button
                                type="button"
                                onClick={() => toggleAuditFor(seg.id)}
                                className="text-xs font-medium text-primary-DEFAULT hover:underline"
                                aria-expanded={auditOpen}
                              >
                                {auditOpen ? 'Hide' : 'Show'} change history ({segmentAudit.length})
                              </button>
                              {auditOpen && (
                                <ul
                                  data-testid={`audit-history-${seg.id}`}
                                  className="mt-2 space-y-1 border-l border-neutral-200 pl-3 text-xs text-neutral-500 dark:border-neutral-700"
                                >
                                  {segmentAudit.length === 0 ? (
                                    <li>No corrections recorded yet.</li>
                                  ) : (
                                    segmentAudit.map((entry) => (
                                      <li key={entry.id}>
                                        {entry.actor} changed {entry.field} from &ldquo;
                                        {entry.previousValue}&rdquo; to &ldquo;{entry.newValue}&rdquo; at{' '}
                                        {new Date(entry.occurredAt).toLocaleString()}
                                      </li>
                                    ))
                                  )}
                                </ul>
                              )}
                            </div>
                          </div>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              )}
            </>
          )}
        </div>
      </Card>
    </div>
  );
}
