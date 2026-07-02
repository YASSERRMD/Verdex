'use client';

import { useState } from 'react';
import clsx from 'clsx';
import { Button } from '@/components/ui/Button';
import type { SegmentReview } from '@/types';

export interface ExtractedTextReviewProps {
  segments: SegmentReview[];
  pageSize?: number;
  className?: string;
}

/**
 * Displays extracted/transcribed segments for review, paged, with a
 * source-span reference for each. This content is draft material — it has
 * not been reviewed or signed off, so nothing here should read as a
 * conclusion or finding.
 */
export function ExtractedTextReview({
  segments,
  pageSize = 5,
  className,
}: ExtractedTextReviewProps) {
  const [page, setPage] = useState(0);

  if (segments.length === 0) {
    return (
      <div className={clsx('rounded-lg border border-neutral-200 bg-white px-4 py-6 text-center', className)}>
        <p className="text-sm text-neutral-400">
          No extracted segments yet. They will appear here once processing completes.
        </p>
      </div>
    );
  }

  const totalPages = Math.ceil(segments.length / pageSize);
  const start = page * pageSize;
  const pageSegments = segments.slice(start, start + pageSize);

  return (
    <div className={clsx('space-y-4', className)}>
      <div>
        <h2 className="text-lg font-semibold text-neutral-800">Extracted Text Review</h2>
        <p className="mt-1 text-sm text-neutral-500">
          Draft segments extracted from your uploaded sources. Review each segment against
          its source span before proceeding.
        </p>
      </div>

      <ul className="space-y-3">
        {pageSegments.map((seg) => (
          <li
            key={seg.id}
            data-testid={`segment-${seg.id}`}
            className="rounded-lg border border-neutral-200 bg-white px-4 py-3"
          >
            <p className="text-sm text-neutral-800">{seg.text}</p>
            <p className="mt-2 text-xs text-neutral-400">
              {seg.sourceFileName && <span>{seg.sourceFileName} · </span>}
              Source span [{seg.sourceSpan.start}–{seg.sourceSpan.end}]
            </p>
          </li>
        ))}
      </ul>

      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
          >
            Previous
          </Button>
          <span className="text-xs text-neutral-500">
            Page {page + 1} of {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  );
}
