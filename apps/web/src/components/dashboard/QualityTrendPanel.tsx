'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import type { QualityTrend } from '@/types';

/**
 * Reasoning-quality trend by jurisdiction, backed by
 * GET /api/v1/analytics/quality-trend
 * (packages/analytics.Dashboard.QualityTrend, which composes
 * packages/reasoningeval.Dashboard.JurisdictionTrend rather than
 * recomputing quality scores — see
 * packages/analytics/doc/analytics.md).
 */
export function QualityTrendPanel() {
  const [trend, setTrend] = useState<QualityTrend | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch<QualityTrend>('/api/v1/analytics/quality-trend')
      .then((result) => {
        if (!cancelled) setTrend(result);
      })
      .catch(() => {
        // Leave trend null; render the empty state.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const points = trend?.points ?? [];

  return (
    <Card padding="md" data-testid="quality-trend-panel">
      <h3 className="mb-4 text-sm font-semibold text-neutral-700 dark:text-neutral-300">
        Reasoning Quality by Jurisdiction
      </h3>
      {points.length === 0 ? (
        <p data-testid="quality-trend-empty" className="text-sm text-neutral-500">
          No quality scores recorded yet.
        </p>
      ) : (
        <ul className="space-y-2">
          {points.map((p) => (
            <li
              key={p.jurisdictionCode}
              data-testid={`quality-row-${p.jurisdictionCode}`}
              className="flex items-center justify-between text-xs"
            >
              <span className="font-medium text-neutral-700 dark:text-neutral-300">
                {p.jurisdictionCode}
                {p.legalFamily ? ` (${p.legalFamily})` : ''}
              </span>
              <span className="text-neutral-500">
                {(p.avgOverall * 100).toFixed(0)}% avg &middot; {p.count} scored
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
