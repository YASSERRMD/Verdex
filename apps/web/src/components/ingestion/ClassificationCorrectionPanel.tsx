'use client';

import { useState } from 'react';
import clsx from 'clsx';
import { apiFetch } from '@/lib/api';
import { Select } from '@/components/ui/Select';
import type { EvidenceType, PartyRole, SegmentClassification } from '@/types';

export interface ClassificationCorrectionPanelProps {
  classifications: SegmentClassification[];
  /** Called after a correction is successfully submitted. */
  onCorrected?: (updated: SegmentClassification) => void;
  className?: string;
}

const EVIDENCE_TYPE_OPTIONS: { value: EvidenceType; label: string }[] = [
  { value: 'testimony', label: 'Testimony' },
  { value: 'documentary', label: 'Documentary' },
  { value: 'statute_citation', label: 'Statute Citation' },
  { value: 'argument', label: 'Argument' },
  { value: 'other', label: 'Other' },
];

const PARTY_OPTIONS: { value: PartyRole; label: string }[] = [
  { value: 'first_party', label: 'First Party' },
  { value: 'second_party', label: 'Second Party' },
  { value: 'third_party', label: 'Third Party' },
  { value: 'unattributed', label: 'Unattributed' },
];

/**
 * UI-only classification correction control. Mirrors
 * packages/evidence's ManualOverride concept: a reviewer can override the
 * automated evidence-type/party classification for a segment. Calls a stub
 * API handler — persistence wiring is a later phase's concern.
 */
export function ClassificationCorrectionPanel({
  classifications,
  onCorrected,
  className,
}: ClassificationCorrectionPanelProps) {
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});

  if (classifications.length === 0) {
    return (
      <div className={clsx('rounded-lg border border-neutral-200 bg-white px-4 py-6 text-center', className)}>
        <p className="text-sm text-neutral-400">
          No classified segments yet. Classification appears once the ingestion pipeline
          completes.
        </p>
      </div>
    );
  }

  const handleOverride = async (
    segmentId: string,
    field: 'type' | 'party',
    value: string,
  ) => {
    const current = classifications.find((c) => c.segmentId === segmentId);
    if (!current) return;

    setPending((p) => ({ ...p, [segmentId]: true }));
    setErrors((e) => ({ ...e, [segmentId]: '' }));

    const updated: SegmentClassification = {
      ...current,
      type: field === 'type' ? (value as EvidenceType) : current.type,
      party: field === 'party' ? (value as PartyRole) : current.party,
      overridden: true,
    };

    try {
      await apiFetch(`/api/v1/segments/${segmentId}/classification`, {
        method: 'PUT',
        body: JSON.stringify({ type: updated.type, party: updated.party }),
      });
      onCorrected?.(updated);
    } catch (err) {
      setErrors((e) => ({
        ...e,
        [segmentId]: err instanceof Error ? err.message : 'Failed to save correction.',
      }));
    } finally {
      setPending((p) => ({ ...p, [segmentId]: false }));
    }
  };

  return (
    <div className={clsx('space-y-4', className)}>
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">Classification Review</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Correct the evidence type or party attribution for any segment. Corrections take
          precedence over the automated classification and are recorded for audit.
        </p>
      </div>

      <ul className="space-y-3">
        {classifications.map((c) => (
          <li
            key={c.segmentId}
            data-testid={`classification-${c.segmentId}`}
            className="grid grid-cols-1 gap-3 rounded-lg border border-neutral-200 bg-white px-4 py-3 sm:grid-cols-2"
          >
            <Select
              label="Evidence Type"
              value={c.type}
              options={EVIDENCE_TYPE_OPTIONS}
              disabled={!!pending[c.segmentId]}
              onChange={(e) => handleOverride(c.segmentId, 'type', e.target.value)}
            />
            <Select
              label="Party"
              value={c.party}
              options={PARTY_OPTIONS}
              disabled={!!pending[c.segmentId]}
              onChange={(e) => handleOverride(c.segmentId, 'party', e.target.value)}
            />
            <div className="sm:col-span-2 flex items-center justify-between text-xs text-neutral-500">
              <span>
                Confidence: {(c.confidence * 100).toFixed(0)}%
                {c.overridden && <span className="ml-2 font-medium text-primary-DEFAULT">Manually overridden</span>}
              </span>
              {errors[c.segmentId] && (
                <span role="alert" className="text-red-600">
                  {errors[c.segmentId]}
                </span>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
