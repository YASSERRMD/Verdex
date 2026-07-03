package signoff

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// InMemoryRepository is an in-process Repository implementation for
// tests and for other packages' test fixtures, mirroring
// packages/caselifecycle.InMemoryRepository's convention exactly. It
// is safe for concurrent use.
type InMemoryRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*SignoffRecord // keyed by CaseID
	audit   map[uuid.UUID][]*AuditEntry  // keyed by CaseID
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		records: make(map[uuid.UUID]*SignoffRecord),
		audit:   make(map[uuid.UUID][]*AuditEntry),
	}
}

func cloneRecord(r *SignoffRecord) *SignoffRecord {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}

func cloneAuditEntry(e *AuditEntry) *AuditEntry {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}

// Get implements Repository.
func (repo *InMemoryRepository) Get(_ context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	r, ok := repo.records[caseID]
	if !ok || r.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneRecord(r), nil
}

// Upsert implements Repository.
func (repo *InMemoryRepository) Upsert(_ context.Context, tenantID uuid.UUID, r *SignoffRecord) error {
	if r == nil {
		return wrapf("InMemoryRepository.Upsert", ErrNilRepository)
	}
	if err := requireMatchingTenant(tenantID, r.TenantID); err != nil {
		return err
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()

	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	repo.records[r.CaseID] = cloneRecord(r)
	return nil
}

// AppendAudit implements Repository.
func (repo *InMemoryRepository) AppendAudit(_ context.Context, tenantID uuid.UUID, e *AuditEntry) error {
	if e == nil {
		return wrapf("InMemoryRepository.AppendAudit", ErrNilRepository)
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	repo.audit[e.CaseID] = append(repo.audit[e.CaseID], cloneAuditEntry(e))
	return nil
}

// ListAudit implements Repository.
func (repo *InMemoryRepository) ListAudit(_ context.Context, tenantID, caseID uuid.UUID) ([]*AuditEntry, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	entries := repo.audit[caseID]
	out := make([]*AuditEntry, 0, len(entries))
	for _, e := range entries {
		if e.TenantID != tenantID {
			continue
		}
		out = append(out, cloneAuditEntry(e))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.Before(out[j].OccurredAt)
	})
	return out, nil
}

var _ Repository = (*InMemoryRepository)(nil)
