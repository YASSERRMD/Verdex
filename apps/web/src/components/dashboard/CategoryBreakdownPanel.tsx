'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import type { AnalyticsMetrics } from '@/types';

/**
 * Case-by-category bar list, backed by the same
 * GET /api/v1/analytics/caseload response CaseloadPanel reads
 * (packages/analytics.Metrics.ByCategory).
 */
export function CategoryBreakdownPanel() {
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

  const categories = metrics?.byCategory ?? [];
  const maxCount = Math.max(1, ...categories.map((c) => c.count));

  return (
    <Card padding="md" data-testid="category-breakdown-panel">
      <h3 className="mb-4 text-sm font-semibold text-neutral-700 dark:text-neutral-300">
        Cases by Category
      </h3>
      {categories.length === 0 ? (
        <p data-testid="category-breakdown-empty" className="text-sm text-neutral-500">
          No categorized cases yet.
        </p>
      ) : (
        <ul className="space-y-2">
          {categories.map((c) => (
            <li key={c.categoryId || 'uncategorized'} data-testid={`category-row-${c.categoryId || 'uncategorized'}`}>
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium text-neutral-700 dark:text-neutral-300">
                  {c.categoryId || 'Uncategorized'}
                </span>
                <span className="text-neutral-500">{c.count}</span>
              </div>
              <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-700">
                <div
                  className="h-full rounded-full bg-primary-DEFAULT"
                  style={{ width: `${(c.count / maxCount) * 100}%` }}
                />
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
