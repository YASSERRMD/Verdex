import { AlertTriangleIcon } from 'lucide-react';
import clsx from 'clsx';

interface DisclaimerProps {
  className?: string;
  compact?: boolean;
}

export function Disclaimer({ className, compact = false }: DisclaimerProps) {
  return (
    <div
      role="note"
      aria-label="Non-binding disclaimer"
      className={clsx(
        'flex items-start gap-3 rounded-lg border border-amber-300 bg-amber-50 text-amber-800',
        compact ? 'px-3 py-2' : 'px-4 py-4',
        className,
      )}
    >
      <AlertTriangleIcon
        className={clsx('flex-shrink-0 text-amber-500', compact ? 'h-4 w-4 mt-0.5' : 'h-5 w-5 mt-0.5')}
        aria-hidden="true"
      />
      <div className={clsx('space-y-0.5', compact ? 'text-xs' : 'text-sm')}>
        <p className="font-semibold">Non-Binding Draft Analysis</p>
        <p className={clsx(compact ? 'text-xs' : 'text-sm', 'text-amber-700')}>
          This system produces non-binding draft analyses only. All outputs require review
          and sign-off by a qualified judge before any legal use or publication.
        </p>
      </div>
    </div>
  );
}
