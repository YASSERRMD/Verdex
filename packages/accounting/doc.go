// Package accounting provides per-case, per-tenant LLM token usage tracking
// and budget enforcement for the Verdex judicial reasoning platform.
//
// Architecture overview:
//
//	AccountingHook (provider.TokenAccountingHook)
//	       |
//	       v
//	AccountingService
//	  |- Repository         (persist UsageRecord)
//	  |- InMemoryBudgetChecker (enforce token/cost limits)
//	  |- AlertSink          (send budget warnings/exceeded events)
//
// Key concepts:
//
//   - UsageRecord: a single LLM call's token counts and estimated cost,
//     scoped to a tenant and optionally to a case.
//
//   - UsageSummary: aggregated totals for a tenant or case over a period.
//
//   - BudgetConfig: daily/monthly token and cost limits per tenant.
//
//   - AlertEvent: fired when usage crosses a warning or hard-stop threshold.
//
//   - ReconcileJob: re-aggregates all persisted records to repair any
//     in-memory state that diverged (e.g. after a restart).
//
//   - DashboardAPI: returns a per-provider / per-task summary with a
//     7-day trend, suitable for JSON serialisation.
package accounting
