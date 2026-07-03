import clsx from 'clsx';
import { Card } from '@/components/ui/Card';
import type { EvidenceSegment, TimelineEvent } from '@/types';

export interface EvidenceTimelinePanelProps {
  segments: EvidenceSegment[];
  events: TimelineEvent[];
  className?: string;
}

const EVIDENCE_TYPE_LABELS: Record<EvidenceSegment['type'], string> = {
  testimony: 'Testimony',
  documentary: 'Documentary',
  statute_citation: 'Statute Citation',
  argument: 'Argument',
  other: 'Other',
};

function sortByOccurredAt(events: TimelineEvent[]): TimelineEvent[] {
  return [...events].sort((a, b) => {
    if (!a.occurredAt && !b.occurredAt) return 0;
    if (!a.occurredAt) return 1;
    if (!b.occurredAt) return -1;
    return a.occurredAt.localeCompare(b.occurredAt);
  });
}

/**
 * Case workspace panel combining the evidence segment list with a
 * chronological timeline of events. Draft material only — segments and
 * events here have not been reviewed or signed off (see Phase 066 for the
 * full evidence review UI).
 */
export function EvidenceTimelinePanel({ segments, events, className }: EvidenceTimelinePanelProps) {
  const orderedEvents = sortByOccurredAt(events);

  return (
    <div className={clsx('space-y-6', className)}>
      <Card
        header={<h2 className="text-base font-semibold text-neutral-800 dark:text-white">Evidence</h2>}
      >
        {segments.length === 0 ? (
          <p className="text-sm text-neutral-400">No evidence segments recorded for this case yet.</p>
        ) : (
          <ul className="space-y-3">
            {segments.map((seg) => (
              <li
                key={seg.id}
                data-testid={`evidence-segment-${seg.id}`}
                className="rounded-lg border border-neutral-200 px-4 py-3 dark:border-neutral-700"
              >
                <div className="flex items-start justify-between gap-3">
                  <p className="text-sm text-neutral-800 dark:text-neutral-200">{seg.text}</p>
                  <span className="flex-shrink-0 rounded-full bg-neutral-100 px-2.5 py-1 text-xs font-medium text-neutral-600 dark:bg-neutral-700 dark:text-neutral-300">
                    {EVIDENCE_TYPE_LABELS[seg.type]}
                  </span>
                </div>
                <p className="mt-2 text-xs text-neutral-400">
                  {seg.sourceFileName && <span>{seg.sourceFileName} · </span>}
                  Source span [{seg.sourceSpan.start}–{seg.sourceSpan.end}] · Confidence{' '}
                  {Math.round(seg.confidence * 100)}%
                </p>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card
        header={<h2 className="text-base font-semibold text-neutral-800 dark:text-white">Timeline</h2>}
      >
        {orderedEvents.length === 0 ? (
          <p className="text-sm text-neutral-400">No timeline events recorded for this case yet.</p>
        ) : (
          <ol className="relative space-y-6 border-l border-neutral-200 pl-6 dark:border-neutral-700">
            {orderedEvents.map((event) => (
              <li key={event.id} data-testid={`timeline-event-${event.id}`} className="relative">
                <span
                  className="absolute -left-[1.6rem] top-1 h-3 w-3 rounded-full border-2 border-white bg-primary-DEFAULT dark:border-neutral-800"
                  aria-hidden="true"
                />
                <p className="text-xs font-medium uppercase tracking-wide text-neutral-500">
                  {event.occurredAt ?? 'Date unknown'}
                </p>
                <p className="mt-0.5 text-sm text-neutral-800 dark:text-neutral-200">
                  {event.description || 'Untitled event'}
                </p>
              </li>
            ))}
          </ol>
        )}
      </Card>
    </div>
  );
}
