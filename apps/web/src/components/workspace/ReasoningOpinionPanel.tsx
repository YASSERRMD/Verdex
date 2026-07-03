import clsx from 'clsx';
import { ScaleIcon } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Disclaimer } from '@/components/Disclaimer';

export interface ReasoningOpinionPanelProps {
  /** True while a draft opinion is being synthesized. */
  loading?: boolean;
  /** True once a draft opinion exists to review. Phase 067 renders it. */
  hasDraftOpinion?: boolean;
  className?: string;
}

/**
 * Entry point for the reasoned-opinion review panel. Always renders the
 * non-binding disclaimer before any draft content, per the Phase 057
 * guardrail: reasoning/opinion output must never be presented without it.
 * Phase 067 fills in the full per-issue analysis; this phase only wires
 * up the tab and a placeholder for the opinion content.
 */
export function ReasoningOpinionPanel({
  loading = false,
  hasDraftOpinion = false,
  className,
}: ReasoningOpinionPanelProps) {
  return (
    <div className={clsx('space-y-4', className)}>
      <Disclaimer />

      <Card
        header={
          <h2 className="text-base font-semibold text-neutral-800 dark:text-white">
            Draft Reasoned Opinion
          </h2>
        }
      >
        <div
          data-testid="reasoning-opinion-placeholder"
          className="flex flex-col items-center justify-center gap-3 py-16 text-center"
        >
          <ScaleIcon className="h-10 w-10 text-neutral-300" aria-hidden="true" />
          {loading ? (
            <p className="text-sm text-neutral-500">Synthesizing draft analysis…</p>
          ) : hasDraftOpinion ? (
            <p className="text-sm text-neutral-500">
              A draft opinion is available. The full review interface is not yet available in
              this build.
            </p>
          ) : (
            <>
              <p className="text-sm font-medium text-neutral-600 dark:text-neutral-300">
                No draft opinion yet
              </p>
              <p className="max-w-sm text-xs text-neutral-400">
                Once reasoning has been generated for this case, the per-issue draft analysis
                with full evidence traceability will render here for review.
              </p>
            </>
          )}
        </div>
      </Card>
    </div>
  );
}
