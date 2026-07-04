package securitytesting

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryFindingRepository is a process-local FindingRepository backed
// by a map guarded by a mutex, intended for tests and other packages'
// fixtures -- never for production use, mirroring
// packages/compliance.InMemoryEvidenceRepository's role exactly.
type InMemoryFindingRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*Finding
}

// NewInMemoryFindingRepository builds an empty
// InMemoryFindingRepository.
func NewInMemoryFindingRepository() *InMemoryFindingRepository {
	return &InMemoryFindingRepository{items: make(map[uuid.UUID]*Finding)}
}

// Create implements FindingRepository.
func (r *InMemoryFindingRepository) Create(_ context.Context, tenantID uuid.UUID, f *Finding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	if f.TenantID == uuid.Nil {
		f.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.items[f.ID] = &cp
	return nil
}

// Get implements FindingRepository.
func (r *InMemoryFindingRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Finding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.items[id]
	if !ok || f.TenantID != tenantID {
		return nil, ErrFindingNotFound
	}
	cp := *f
	return &cp, nil
}

// ListAll implements FindingRepository.
func (r *InMemoryFindingRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]Finding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Finding, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID {
			out = append(out, *f)
		}
	}
	return out, nil
}

// ListByStatus implements FindingRepository.
func (r *InMemoryFindingRepository) ListByStatus(_ context.Context, tenantID uuid.UUID, status FindingStatus) ([]Finding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Finding, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID && f.Status == status {
			out = append(out, *f)
		}
	}
	return out, nil
}

// Update implements FindingRepository.
func (r *InMemoryFindingRepository) Update(_ context.Context, tenantID uuid.UUID, f *Finding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.items[f.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrFindingNotFound
	}
	cp := *f
	r.items[f.ID] = &cp
	return nil
}

var _ FindingRepository = (*InMemoryFindingRepository)(nil)

// InMemoryRunRecordRepository is a process-local RunRecordRepository
// backed by a map guarded by a mutex.
type InMemoryRunRecordRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*RunRecord
}

// NewInMemoryRunRecordRepository builds an empty
// InMemoryRunRecordRepository.
func NewInMemoryRunRecordRepository() *InMemoryRunRecordRepository {
	return &InMemoryRunRecordRepository{items: make(map[uuid.UUID]*RunRecord)}
}

// Create implements RunRecordRepository. Rejects a RunRecord.ID that
// already exists with ErrDuplicateRunRecord -- RunRecords are
// append-only (see RunRecordRepository's doc comment), so Create is
// never an upsert, unlike this file's Finding/Update methods above.
func (r *InMemoryRunRecordRepository) Create(_ context.Context, tenantID uuid.UUID, rr *RunRecord) error {
	if rr == nil {
		return ErrInvalidRunRecord
	}
	if rr.TenantID == uuid.Nil {
		rr.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rr.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[rr.ID]; exists {
		return ErrDuplicateRunRecord
	}
	cp := *rr
	r.items[rr.ID] = &cp
	return nil
}

// Get implements RunRecordRepository.
func (r *InMemoryRunRecordRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*RunRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rr, ok := r.items[id]
	if !ok || rr.TenantID != tenantID {
		return nil, ErrInvalidRunRecord
	}
	cp := *rr
	return &cp, nil
}

// ListAll implements RunRecordRepository.
func (r *InMemoryRunRecordRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]RunRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RunRecord, 0)
	for _, rr := range r.items {
		if rr.TenantID == tenantID {
			out = append(out, *rr)
		}
	}
	return out, nil
}

// ListForScenario implements RunRecordRepository.
func (r *InMemoryRunRecordRepository) ListForScenario(_ context.Context, tenantID uuid.UUID, scenarioName string) ([]RunRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RunRecord, 0)
	for _, rr := range r.items {
		if rr.TenantID == tenantID && rr.ScenarioName == scenarioName {
			out = append(out, *rr)
		}
	}
	return out, nil
}

var _ RunRecordRepository = (*InMemoryRunRecordRepository)(nil)
