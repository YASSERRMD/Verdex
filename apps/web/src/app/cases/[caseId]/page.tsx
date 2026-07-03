'use client';

import { useCallback, useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getSession } from '@/lib/auth';
import { apiFetch, ApiError } from '@/lib/api';
import { AppShell } from '@/components/layout/AppShell';
import { CaseHeader } from '@/components/workspace/CaseHeader';
import { PartiesCategoryPanel } from '@/components/workspace/PartiesCategoryPanel';
import { EvidenceTimelinePanel } from '@/components/workspace/EvidenceTimelinePanel';
import { TreeVisualizationPanel } from '@/components/workspace/TreeVisualizationPanel';
import { ReasoningOpinionPanel } from '@/components/workspace/ReasoningOpinionPanel';
import { StatusActionsBar } from '@/components/workspace/StatusActionsBar';
import { WorkspaceTabs, type WorkspaceTabId } from '@/components/workspace/WorkspaceTabs';
import { WorkspaceLoading } from '@/components/workspace/WorkspaceLoading';
import { WorkspaceError } from '@/components/workspace/WorkspaceError';
import type { CaseLifecycle, CaseParty, CaseState, EvidenceSegment, Role, TimelineEvent } from '@/types';

interface CaseWorkspaceData {
  caseData: CaseLifecycle;
  parties: CaseParty[];
  evidence: EvidenceSegment[];
  events: TimelineEvent[];
}

export default function CaseWorkspacePage() {
  const router = useRouter();
  const params = useParams<{ caseId: string }>();
  const caseId = params?.caseId;
  const session = getSession();

  const [activeTab, setActiveTab] = useState<WorkspaceTabId>('overview');
  const [data, setData] = useState<CaseWorkspaceData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [transitionBusy, setTransitionBusy] = useState(false);

  const loadCase = useCallback(async () => {
    if (!caseId) return;
    setLoading(true);
    setError(null);
    try {
      const result = await apiFetch<CaseWorkspaceData>(`/api/v1/cases/${caseId}`);
      setData(result);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setError('Case not found.');
      } else {
        setError(err instanceof Error ? err.message : 'Failed to load case.');
      }
    } finally {
      setLoading(false);
    }
  }, [caseId]);

  const hasSession = !!session;
  useEffect(() => {
    if (!hasSession) {
      router.replace('/login');
      return;
    }
    loadCase();
    // router intentionally omitted: Next.js's useRouter() return value is
    // stable, and including it here would re-run this effect (and re-fetch
    // the case) on every render where a new router-like object is handed
    // back by a test double.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasSession, loadCase]);

  const applyStateChange = async (mutate: () => Promise<CaseLifecycle>) => {
    setTransitionBusy(true);
    try {
      const updated = await mutate();
      setData((prev) => (prev ? { ...prev, caseData: updated } : prev));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update case state.');
    } finally {
      setTransitionBusy(false);
    }
  };

  const handleTransition = (toState: CaseState) =>
    applyStateChange(() =>
      apiFetch<CaseLifecycle>(`/api/v1/cases/${caseId}/transition`, {
        method: 'POST',
        body: JSON.stringify({ toState }),
      }),
    );

  const handleReopen = (justification: string) =>
    applyStateChange(() =>
      apiFetch<CaseLifecycle>(`/api/v1/cases/${caseId}/reopen`, {
        method: 'POST',
        body: JSON.stringify({ justification }),
      }),
    );

  const handleArchive = (reason?: string) =>
    applyStateChange(() =>
      apiFetch<CaseLifecycle>(`/api/v1/cases/${caseId}/archive`, {
        method: 'POST',
        body: JSON.stringify({ reason: reason ?? '' }),
      }),
    );

  if (!session) return null;

  const roles = (session.user.roles ?? []) as Role[];

  return (
    <AppShell user={session.user} roles={roles}>
      <div className="mx-auto max-w-5xl space-y-6">
        {loading && <WorkspaceLoading />}

        {!loading && error && <WorkspaceError message={error} onRetry={loadCase} />}

        {!loading && !error && data && (
          <>
            <CaseHeader caseData={data.caseData} />

            <StatusActionsBar
              state={data.caseData.state}
              busy={transitionBusy}
              onTransition={handleTransition}
              onReopen={handleReopen}
              onArchive={handleArchive}
            />

            <WorkspaceTabs activeTab={activeTab} onTabChange={setActiveTab} />

            <div
              role="tabpanel"
              id={`workspace-tabpanel-${activeTab}`}
              aria-labelledby={`workspace-tab-${activeTab}`}
            >
              {activeTab === 'overview' && (
                <PartiesCategoryPanel caseData={data.caseData} parties={data.parties} />
              )}
              {activeTab === 'evidence' && (
                <EvidenceTimelinePanel segments={data.evidence} events={data.events} />
              )}
              {activeTab === 'tree' && <TreeVisualizationPanel />}
              {activeTab === 'reasoning' && <ReasoningOpinionPanel />}
            </div>
          </>
        )}
      </div>
    </AppShell>
  );
}
