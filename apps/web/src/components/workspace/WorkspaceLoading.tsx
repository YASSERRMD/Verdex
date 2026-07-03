import { Spinner } from '@/components/ui/Spinner';

/** Full-panel loading state shown while a case record is being fetched. */
export function WorkspaceLoading() {
  return (
    <div
      data-testid="workspace-loading"
      className="flex flex-col items-center justify-center gap-3 py-24 text-center"
    >
      <Spinner size="lg" className="text-primary-DEFAULT" />
      <p className="text-sm text-neutral-500">Loading case…</p>
    </div>
  );
}
