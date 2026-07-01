# Token Accounting & Budget Model

## Overview

The `accounting` package tracks per-case, per-tenant LLM token consumption for
the Verdex judicial reasoning platform.  It is designed to be:

- **Non-blocking**: the `AccountingHook` forwards usage events to the service
  without blocking the LLM request path.
- **Idempotent**: `ReconcileJob` can rebuild the in-memory budget state from the
  persistent record store at any time.
- **Auditable**: every LLM call produces a `UsageRecord` that is persisted
  before any budget decision is made.

---

## Data Model

### UsageRecord

```
UsageRecord {
  ID           uuid        -- unique record identifier
  TenantID     uuid        -- owning tenant (required)
  CaseID       *uuid       -- optional judicial case
  ProviderID   string      -- LLM provider (e.g. "anthropic")
  TaskType     string      -- "chat" | "embed" | "reason" | "extract"
  InputTokens  int         -- prompt tokens
  OutputTokens int         -- completion tokens
  TotalTokens  int         -- InputTokens + OutputTokens
  CostUSD      *float64    -- estimated cost (nil if pricing unavailable)
  RequestID    string      -- provider-assigned completion ID
  CreatedAt    time.Time   -- UTC creation timestamp
}
```

### UsageSummary

Aggregated totals for a `(TenantID, Period)` or `(CaseID)` group:

```
UsageSummary {
  TenantID          uuid
  CaseID            *uuid
  Period            string      -- "YYYY-MM-DD" or "YYYY-MM"
  TotalInputTokens  int
  TotalOutputTokens int
  TotalTokens       int
  EstimatedCostUSD  float64
  RequestCount      int
}
```

---

## Budget Enforcement

Budget limits are configured per tenant via `BudgetConfig`:

| Field                | Type      | Description                                   |
|----------------------|-----------|-----------------------------------------------|
| `DailyTokenLimit`    | `*int`    | Maximum total tokens per calendar day         |
| `MonthlyTokenLimit`  | `*int`    | Maximum total tokens per calendar month       |
| `DailyCostLimitUSD`  | `*float64`| Maximum estimated cost per calendar day       |
| `MonthlyCostLimitUSD`| `*float64`| Maximum estimated cost per calendar month     |
| `HardStop`           | `bool`    | Block the request when any limit is breached  |
| `AlertThresholdPct`  | `float64` | Percentage at which to send a warning alert   |

### Enforcement flow

```
LLM call completes
    │
    ▼
provider.HookedProvider
    │
    ▼ RecordUsage(ctx, providerID, usage, task)
AccountingHook
    │
    ▼ RecordUsage(ctx, UsageRecord)
AccountingService
    ├── repo.SaveRecord(record)          ← always persisted first
    ├── checker.RecordUsage(...)         ← update in-memory totals
    ├── checker.Check(ctx, tenantID, usage)
    │       ├── allowed=true, alert=false  → return nil
    │       ├── allowed=true, alert=true   → send AlertTypeBudgetWarning
    │       └── allowed=false              → send AlertTypeBudgetExceeded
    │                                        return ErrBudgetExceeded (if HardStop)
    └── return
```

### Alert types

| AlertType           | When fired                                      |
|---------------------|-------------------------------------------------|
| `budget_warning`    | Usage ≥ `AlertThresholdPct` % of any limit      |
| `budget_exceeded`   | Usage ≥ 100 % of any limit                     |

---

## Aggregation

Two aggregation helpers are provided:

- **`AggregateByCasePeriod`**: groups records by `(CaseID, YYYY-MM-DD)`.
- **`AggregateByTenantPeriod`**: groups records by `(TenantID, period)` where
  period is either `"daily"` (YYYY-MM-DD) or `"monthly"` (YYYY-MM).

---

## Cost Estimation

Use `CostEstimate(inputTokens, outputTokens int, pricing PricingConfig) float64`
to compute estimated cost:

```
cost = (inputTokens / 1_000_000) * InputPricePer1M
     + (outputTokens / 1_000_000) * OutputPricePer1M
```

Assign the result to `UsageRecord.CostUSD` before calling `RecordUsage`.

---

## Reconciliation

`ReconcileJob.Run(ctx)` re-aggregates **all** persisted records into the
in-memory budget checker state.  It is:

- **Idempotent**: calling it N times is equivalent to calling it once.
- **Safe to run concurrently**: the last run to finish wins the state write.

Run it on service startup and periodically (e.g. every hour) to keep
in-memory state consistent with the persisted record store.

---

## Dashboard

`DashboardAPI.GetTenantDashboard(ctx, tenantID)` returns a `TenantDashboard`
containing:

- **`ByProvider`**: per-provider token and cost totals.
- **`ByTaskType`**: per-task-type totals.
- **`Last7DaysTrend`**: daily totals for the past 7 calendar days (zero-filled
  for days with no activity).

The dashboard is JSON-serialisable and suitable for a REST API response.

---

## Integration

```go
// 1. Build the accounting stack.
repo    := accounting.NewInMemoryRepository()
checker := accounting.NewInMemoryBudgetChecker(budgetConfigs)
alerts  := &accounting.LoggingAlertSink{}
svc     := accounting.NewAccountingService(repo, checker, alerts)
hook    := accounting.NewAccountingHook(svc, defaultTenantID)

// 2. Wrap the LLM provider.
p := provider.HookedProvider(baseProvider, hook)

// 3. Inject tenant/case context before each call.
ctx = accounting.WithTenantID(ctx, tenantID)
ctx = accounting.WithCaseID(ctx, caseID)
resp, err := p.Chat(ctx, req)

// 4. Reconcile on startup.
job := accounting.NewReconcileJob(repo, checker)
n, err := job.Run(ctx)
```
