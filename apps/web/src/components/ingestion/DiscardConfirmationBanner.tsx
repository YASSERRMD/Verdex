import { ShieldCheckIcon } from 'lucide-react';
import clsx from 'clsx';

export interface DiscardConfirmationBannerProps {
  className?: string;
  compact?: boolean;
}

/**
 * Explains, factually and without overstatement, that uploaded source
 * binaries are cryptographically hashed for provenance and then discarded
 * once their text/transcript has been extracted — matching the
 * transcribe-and-discard guarantee implemented in packages/provenance and
 * packages/ingestion. Visually mirrors Disclaimer.tsx so the two banners
 * read as part of the same guardrail family.
 */
export function DiscardConfirmationBanner({
  className,
  compact = false,
}: DiscardConfirmationBannerProps) {
  return (
    <div
      role="note"
      aria-label="Source file discard notice"
      className={clsx(
        'flex items-start gap-3 rounded-lg border border-primary-200 bg-primary-50 text-primary-800',
        compact ? 'px-3 py-2' : 'px-4 py-4',
        className,
      )}
    >
      <ShieldCheckIcon
        className={clsx('flex-shrink-0 text-primary-500', compact ? 'h-4 w-4 mt-0.5' : 'h-5 w-5 mt-0.5')}
        aria-hidden="true"
      />
      <div className={clsx('space-y-0.5', compact ? 'text-xs' : 'text-sm')}>
        <p className="font-semibold">Source Files Are Hashed, Then Discarded</p>
        <p className={clsx(compact ? 'text-xs' : 'text-sm', 'text-primary-700')}>
          Each uploaded document or recording is cryptographically hashed for provenance
          before any processing begins. Once its text has been transcribed or extracted, the
          original binary is discarded — only the hash, the extracted text, and the
          chain-of-custody record are retained. This extracted text is draft material only
          and has not yet been reviewed.
        </p>
      </div>
    </div>
  );
}
