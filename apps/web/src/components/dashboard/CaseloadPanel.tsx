'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import type { AnalyticsMetrics } from '@/types';

const STATE_LABEL: Record<string, string> = {
  draft: 'Draft',
  active: 'Active',
  under_review: 'Under Review',
  closed: 'Closed',
  archived: 'Archived',
};

/**
 * Top-of-dashboard caseload summary: total case count plus a
 * by-state breakdown, backed by GET /api/v1/analytics/caseload
 * (packages/analytics.Dashboard.Caseload). See
 * packages/analytics/doc/analytics.md, "Web dashboard".
 */
export function CaseloadPanel() {
  const [metrics, setMetrics] = useState<AnalyticsMetrics | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    apiFetch<AnalyticsMetrics>('/api/v1/analytics/caseload')
      .then((result) => {
        if (!cancelled) setMetrics(result);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <Card padding="md" data-testid="caseload-panel-error">
        <p className="text-sm text-neutral-500">Caseload metrics are unavailable right now.</p>
      </Card>
    );
  }

  const stats = [
    { label: 'Total Cases', value: metrics?.totalCases },
    ...['active', 'under_review', 'closed'].map((state) => ({
      label: STATE_LABEL[state],
      value: metrics?.byState.find((s) => s.state === state)?.count,
    })),
  ];

  return (
    <div data-testid="caseload-panel" className="grid grid-cols-1 gap-4 sm:grid-cols-4">
      {stats.map((stat) => (
        <Card key={stat.label} padding="md">
          <p className="text-xs font-medium uppercase tracking-wider text-neutral-500">
            {stat.label}
          </p>
          <p className="mt-2 text-3xl font-bold text-primary-DEFAULT">
            {stat.value ?? (metrics ? 0 : '—')}
          </p>
        </Card>
      ))}
    </div>
  );
}
