'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Card } from '@/components/ui/Card';
import type { Role, UsageDashboard } from '@/types';

const AUTHORIZED_ROLES: Role[] = ['admin', 'judge'];

interface UsageCostPanelProps {
  roles: Role[];
}

/**
 * Token usage / estimated cost view, backed by
 * GET /api/v1/analytics/usage
 * (packages/analytics.Dashboard.UsageView, which composes
 * packages/accounting.DashboardAPI.GetTenantDashboard). Only rendered
 * for roles that hold identity.PermAuditRead server-side (judge,
 * admin, auditor per identity.PermissionMatrix) — the client checks
 * `roles` here purely as a UX nicety to avoid an unnecessary request
 * and a flash of a 403 state; the server enforces
 * packages/analytics.RequireCostPermission regardless of what the
 * client sends. `auditor` is not currently a web `Role` value, so this
 * panel is shown for `admin`/`judge` only.
 */
export function UsageCostPanel({ roles }: UsageCostPanelProps) {
  const [usage, setUsage] = useState<UsageDashboard | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const authorized = roles.some((r) => AUTHORIZED_ROLES.includes(r));

  useEffect(() => {
    if (!authorized) return;
    let cancelled = false;
    apiFetch<UsageDashboard>('/api/v1/analytics/usage')
      .then((result) => {
        if (!cancelled) setUsage(result);
      })
      .catch(() => {
        if (!cancelled) setForbidden(true);
      });
    return () => {
      cancelled = true;
    };
  }, [authorized]);

  if (!authorized) {
    return null;
  }

  if (forbidden) {
    return (
      <Card padding="md" data-testid="usage-cost-panel-error">
        <p className="text-sm text-neutral-500">Usage and cost data is unavailable right now.</p>
      </Card>
    );
  }

  return (
    <Card padding="md" data-testid="usage-cost-panel">
      <h3 className="mb-4 text-sm font-semibold text-neutral-700 dark:text-neutral-300">
        Usage &amp; Cost
      </h3>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div>
          <p className="text-xs uppercase tracking-wider text-neutral-500">Total Tokens</p>
          <p className="mt-1 text-xl font-bold text-primary-DEFAULT">
            {usage ? usage.totalTokens.toLocaleString() : '—'}
          </p>
        </div>
        <div>
          <p className="text-xs uppercase tracking-wider text-neutral-500">Estimated Cost</p>
          <p className="mt-1 text-xl font-bold text-primary-DEFAULT">
            {usage ? `$${usage.estimatedCostUsd.toFixed(2)}` : '—'}
          </p>
        </div>
        <div>
          <p className="text-xs uppercase tracking-wider text-neutral-500">Requests</p>
          <p className="mt-1 text-xl font-bold text-primary-DEFAULT">
            {usage ? usage.requestCount.toLocaleString() : '—'}
          </p>
        </div>
      </div>
      {usage && usage.byProvider.length > 0 && (
        <ul className="mt-4 space-y-1 border-t border-neutral-200 pt-3 text-xs dark:border-neutral-700">
          {usage.byProvider.map((p) => (
            <li key={p.providerId} data-testid={`usage-provider-${p.providerId}`} className="flex justify-between">
              <span className="text-neutral-600 dark:text-neutral-400">{p.providerId}</span>
              <span className="text-neutral-500">
                {p.totalTokens.toLocaleString()} tokens &middot; ${p.estimatedCostUsd.toFixed(2)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
