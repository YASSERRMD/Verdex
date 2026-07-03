import clsx from 'clsx';
import { GitBranchIcon } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';

export interface TreeVisualizationPanelProps {
  /** True while the reasoning tree is being generated or fetched. */
  loading?: boolean;
  className?: string;
}

/**
 * Navigation entry point and empty/loading state for the IRAC reasoning
 * tree view. Phase 065 renders the actual interactive tree/graph into this
 * panel; this phase only wires up the tab and a placeholder state so
 * there is somewhere for that visualization to mount.
 */
export function TreeVisualizationPanel({ loading = false, className }: TreeVisualizationPanelProps) {
  return (
    <Card
      className={clsx(className)}
      header={
        <h2 className="text-base font-semibold text-neutral-800 dark:text-white">
          Reasoning Tree
        </h2>
      }
    >
      <div
        data-testid="tree-visualization-placeholder"
        className="flex flex-col items-center justify-center gap-3 py-16 text-center"
      >
        {loading ? (
          <>
            <Spinner size="lg" className="text-primary-DEFAULT" />
            <p className="text-sm text-neutral-500">Generating the reasoning tree…</p>
          </>
        ) : (
          <>
            <GitBranchIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
            <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">
              No reasoning tree yet
            </p>
            <p className="max-w-sm text-xs text-neutral-400">
              The interactive issue/rule/fact/conclusion tree view will render here once
              reasoning has been generated for this case.
            </p>
          </>
        )}
      </div>
    </Card>
  );
}
