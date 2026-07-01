package accounting

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Repository persists UsageRecord values and provides query helpers for
// building summaries.
type Repository interface {
	// SaveRecord persists a UsageRecord.
	SaveRecord(ctx context.Context, record UsageRecord) error

	// SumByTenant returns an aggregated UsageSummary for all records belonging
	// to tenantID whose CreatedAt falls within [from, to].
	SumByTenant(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (*UsageSummary, error)

	// SumByCase returns an aggregated UsageSummary for all records linked to
	// caseID, regardless of time range.
	SumByCase(ctx context.Context, caseID uuid.UUID) (*UsageSummary, error)

	// ListRecords returns paginated records for tenantID ordered by CreatedAt
	// descending.  limit and offset follow SQL LIMIT/OFFSET semantics.
	ListRecords(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]UsageRecord, error)
}

// InMemoryRepository is a thread-safe in-memory implementation of Repository.
// It is intended for unit tests and local development only.
type InMemoryRepository struct {
	mu      sync.RWMutex
	records []UsageRecord
}

// NewInMemoryRepository returns an initialised InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{}
}

// SaveRecord appends the record to the in-memory store.
func (r *InMemoryRepository) SaveRecord(_ context.Context, record UsageRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

// SumByTenant aggregates all records for tenantID within the given time range.
func (r *InMemoryRepository) SumByTenant(_ context.Context, tenantID uuid.UUID, from, to time.Time) (*UsageSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sum := &UsageSummary{
		TenantID: tenantID,
		Period:   from.UTC().Format("2006-01-02") + "/" + to.UTC().Format("2006-01-02"),
	}
	found := false
	for _, rec := range r.records {
		if rec.TenantID != tenantID {
			continue
		}
		if rec.CreatedAt.Before(from) || rec.CreatedAt.After(to) {
			continue
		}
		found = true
		sum.TotalInputTokens += rec.InputTokens
		sum.TotalOutputTokens += rec.OutputTokens
		sum.TotalTokens += rec.TotalTokens
		if rec.CostUSD != nil {
			sum.EstimatedCostUSD += *rec.CostUSD
		}
		sum.RequestCount++
	}
	if !found {
		return nil, ErrUsageNotFound
	}
	return sum, nil
}

// SumByCase aggregates all records linked to caseID.
func (r *InMemoryRepository) SumByCase(_ context.Context, caseID uuid.UUID) (*UsageSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tenantID uuid.UUID
	sum := &UsageSummary{
		CaseID: &caseID,
	}
	found := false
	for _, rec := range r.records {
		if rec.CaseID == nil || *rec.CaseID != caseID {
			continue
		}
		if !found {
			tenantID = rec.TenantID
		}
		found = true
		sum.TotalInputTokens += rec.InputTokens
		sum.TotalOutputTokens += rec.OutputTokens
		sum.TotalTokens += rec.TotalTokens
		if rec.CostUSD != nil {
			sum.EstimatedCostUSD += *rec.CostUSD
		}
		sum.RequestCount++
	}
	if !found {
		return nil, ErrUsageNotFound
	}
	sum.TenantID = tenantID
	return sum, nil
}

// ListRecords returns paginated records for tenantID ordered by CreatedAt descending.
func (r *InMemoryRepository) ListRecords(_ context.Context, tenantID uuid.UUID, limit, offset int) ([]UsageRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []UsageRecord
	for _, rec := range r.records {
		if rec.TenantID == tenantID {
			filtered = append(filtered, rec)
		}
	}

	// Sort descending by CreatedAt.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	if offset >= len(filtered) {
		return nil, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}

// AllRecords returns a copy of all stored records.  It is used by ReconcileJob.
func (r *InMemoryRepository) AllRecords(_ context.Context) ([]UsageRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]UsageRecord, len(r.records))
	copy(out, r.records)
	return out, nil
}
