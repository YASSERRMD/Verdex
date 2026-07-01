package accounting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// allRecordsFetcher is a subset of InMemoryRepository used by ReconcileJob so
// that the job only needs to know about the fetch method.
type allRecordsFetcher interface {
	AllRecords(ctx context.Context) ([]UsageRecord, error)
}

// ReconcileJob rebuilds the in-memory budget state from the persistent record
// store.  It is idempotent: running it multiple times produces the same result
// as running it once.
//
// Use it on startup (to restore state after a restart) or periodically (to
// correct any drift caused by missed RecordUsage calls).
type ReconcileJob struct {
	fetcher allRecordsFetcher
	checker *InMemoryBudgetChecker
}

// NewReconcileJob constructs a ReconcileJob.  repo must implement
// AllRecords (e.g. *InMemoryRepository); checker is the budget state to
// overwrite.
func NewReconcileJob(repo allRecordsFetcher, checker *InMemoryBudgetChecker) *ReconcileJob {
	return &ReconcileJob{fetcher: repo, checker: checker}
}

// Run re-aggregates all persisted records into the in-memory budget checker
// and returns the total count of records processed.
//
// It is safe to call concurrently, though concurrent calls will each do a
// full pass; the last one to finish wins the state write.
func (j *ReconcileJob) Run(ctx context.Context) (int, error) {
	records, err := j.fetcher.AllRecords(ctx)
	if err != nil {
		return 0, fmt.Errorf("reconcile: fetch records: %w", err)
	}

	// Group by tenant → period.
	type dailyKey struct {
		tenantID uuid.UUID
		day      string
	}
	type monthlyKey struct {
		tenantID uuid.UUID
		month    string
	}

	dailyAgg := make(map[dailyKey]periodUsage)
	monthlyAgg := make(map[monthlyKey]periodUsage)

	for _, r := range records {
		day := r.CreatedAt.UTC().Format("2006-01-02")
		month := r.CreatedAt.UTC().Format("2006-01")

		cost := 0.0
		if r.CostUSD != nil {
			cost = *r.CostUSD
		}

		dk := dailyKey{tenantID: r.TenantID, day: day}
		d := dailyAgg[dk]
		d.tokens += r.TotalTokens
		d.costUSD += cost
		dailyAgg[dk] = d

		mk := monthlyKey{tenantID: r.TenantID, month: month}
		m := monthlyAgg[mk]
		m.tokens += r.TotalTokens
		m.costUSD += cost
		monthlyAgg[mk] = m
	}

	// Collect affected tenants.
	tenants := make(map[uuid.UUID]struct{})
	for k := range dailyAgg {
		tenants[k.tenantID] = struct{}{}
	}

	// Write per-tenant state into the checker.
	for tenantID := range tenants {
		daily := make(map[string]periodUsage)
		for dk, v := range dailyAgg {
			if dk.tenantID == tenantID {
				daily[dk.day] = v
			}
		}
		monthly := make(map[string]periodUsage)
		for mk, v := range monthlyAgg {
			if mk.tenantID == tenantID {
				monthly[mk.month] = v
			}
		}
		j.checker.ResetState(tenantID, daily, monthly)
	}

	return len(records), nil
}

// RunWithTimeout is a convenience wrapper that cancels Run after the given duration.
func (j *ReconcileJob) RunWithTimeout(timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return j.Run(ctx)
}
