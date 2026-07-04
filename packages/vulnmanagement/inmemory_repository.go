package vulnmanagement

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryFindingRepository is a process-local FindingRepository
// backed by a map guarded by a mutex, intended for tests and other
// packages' fixtures -- never for production use, mirroring
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

// ListBySource implements FindingRepository.
func (r *InMemoryFindingRepository) ListBySource(_ context.Context, tenantID uuid.UUID, source ScannerSource) ([]Finding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Finding, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID && f.Source == source {
			out = append(out, *f)
		}
	}
	return out, nil
}

// ListByStatus implements FindingRepository.
func (r *InMemoryFindingRepository) ListByStatus(_ context.Context, tenantID uuid.UUID, status Status) ([]Finding, error) {
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

// InMemoryTriageRepository is a process-local TriageRepository backed
// by a map guarded by a mutex.
type InMemoryTriageRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*TriageDecision
}

// NewInMemoryTriageRepository builds an empty InMemoryTriageRepository.
func NewInMemoryTriageRepository() *InMemoryTriageRepository {
	return &InMemoryTriageRepository{items: make(map[uuid.UUID]*TriageDecision)}
}

// Create implements TriageRepository.
func (r *InMemoryTriageRepository) Create(_ context.Context, tenantID uuid.UUID, d *TriageDecision) error {
	if d == nil {
		return ErrInvalidTriageDecision
	}
	if d.TenantID == uuid.Nil {
		d.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.items[d.ID] = &cp
	return nil
}

// ListForFinding implements TriageRepository.
func (r *InMemoryTriageRepository) ListForFinding(_ context.Context, tenantID, findingID uuid.UUID) ([]TriageDecision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TriageDecision, 0)
	for _, d := range r.items {
		if d.TenantID == tenantID && d.FindingID == findingID {
			out = append(out, *d)
		}
	}
	return out, nil
}

// ListAll implements TriageRepository.
func (r *InMemoryTriageRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]TriageDecision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TriageDecision, 0)
	for _, d := range r.items {
		if d.TenantID == tenantID {
			out = append(out, *d)
		}
	}
	return out, nil
}

var _ TriageRepository = (*InMemoryTriageRepository)(nil)
