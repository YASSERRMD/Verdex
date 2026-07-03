package reportexport

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryAuditRepository is an in-process AuditRepository
// implementation for tests and other packages' test fixtures,
// mirroring packages/notifications.InMemoryRepository's convention
// exactly. It is safe for concurrent use.
type InMemoryAuditRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*AuditRecord
}

// NewInMemoryAuditRepository builds an empty InMemoryAuditRepository.
func NewInMemoryAuditRepository() *InMemoryAuditRepository {
	return &InMemoryAuditRepository{records: make(map[uuid.UUID]*AuditRecord)}
}

func cloneAuditRecord(a *AuditRecord) *AuditRecord {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

// Create implements AuditRepository.
func (r *InMemoryAuditRepository) Create(_ context.Context, tenantID uuid.UUID, rec *AuditRecord) error {
	if rec == nil {
		return wrapf("InMemoryAuditRepository.Create", ErrNilRecord)
	}
	if rec.TenantID == uuid.Nil {
		rec.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	if rec.ExportedAt.IsZero() {
		rec.ExportedAt = time.Now().UTC()
	}

	r.records[rec.ID] = cloneAuditRecord(rec)
	return nil
}

// List implements AuditRepository.
func (r *InMemoryAuditRepository) List(_ context.Context, tenantID uuid.UUID, filter AuditFilter) ([]*AuditRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*AuditRecord, 0)
	for _, rec := range r.records {
		if rec.TenantID != tenantID {
			continue
		}
		if filter.CaseID != nil && rec.CaseID != *filter.CaseID {
			continue
		}
		if filter.ActorID != nil && rec.ActorID != *filter.ActorID {
			continue
		}
		if filter.Format != "" && rec.Format != filter.Format {
			continue
		}
		if !filter.Since.IsZero() && rec.ExportedAt.Before(filter.Since) {
			continue
		}
		out = append(out, cloneAuditRecord(rec))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExportedAt.After(out[j].ExportedAt)
	})
	return out, nil
}

var _ AuditRepository = (*InMemoryAuditRepository)(nil)
