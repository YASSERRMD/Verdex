import { AlertCircleIcon } from 'lucide-react';
import { Button } from '@/components/ui/Button';

export interface WorkspaceErrorProps {
  message: string;
  onRetry?: () => void;
}

/**
 * Full-panel error state for the case workspace, covering both "case not
 * found" and generic fetch failures.
 */
export function WorkspaceError({ message, onRetry }: WorkspaceErrorProps) {
  return (
    <div
      role="alert"
      data-testid="workspace-error"
      className="flex flex-col items-center justify-center gap-3 py-24 text-center"
    >
      <AlertCircleIcon className="h-10 w-10 text-red-400" aria-hidden="true" />
      <p className="text-sm font-medium text-neutral-700 dark:text-neutral-200">{message}</p>
      {onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  );
}
