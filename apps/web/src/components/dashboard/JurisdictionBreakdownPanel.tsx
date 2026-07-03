'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import type { AnalyticsMetrics } from '@/types';

/**
 * Per-jurisdiction caseload table, backed by the same
 * GET /api/v1/analytics/caseload response CaseloadPanel reads
 * (packages/analytics.Metrics.ByJurisdiction).
 */
export function JurisdictionBreakdownPanel() {
  const [metrics, setMetrics] = useState<AnalyticsMetrics | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch<AnalyticsMetrics>('/api/v1/analytics/caseload')
      .then((result) => {
        if (!cancelled) setMetrics(result);
      })
      .catch(() => {
        // Leave metrics null; render the empty state.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const jurisdictions = metrics?.byJurisdiction ?? [];

  return (
    <Card padding="md" data-testid="jurisdiction-breakdown-panel">
      <h3 className="mb-4 text-sm font-semibold text-neutral-700 dark:text-neutral-300">
        Cases by Jurisdiction
      </h3>
      {jurisdictions.length === 0 ? (
        <p data-testid="jurisdiction-breakdown-empty" className="text-sm text-neutral-500">
          No jurisdiction data yet.
        </p>
      ) : (
        <table className="w-full text-left text-xs">
          <thead>
            <tr className="text-neutral-500">
              <th className="pb-2 font-medium">Jurisdiction</th>
              <th className="pb-2 font-medium">Total</th>
              <th className="pb-2 font-medium">By State</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-100 dark:divide-neutral-700">
            {jurisdictions.map((j) => (
              <tr key={j.jurisdictionId} data-testid={`jurisdiction-row-${j.jurisdictionId}`}>
                <td className="py-2 font-mono text-[11px] text-neutral-600 dark:text-neutral-400">
                  {j.jurisdictionId}
                </td>
                <td className="py-2 font-semibold text-neutral-800 dark:text-neutral-100">
                  {j.count}
                </td>
                <td className="py-2 text-neutral-500">
                  {j.byState.map((s) => `${s.state}: ${s.count}`).join(', ')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Card>
  );
}
