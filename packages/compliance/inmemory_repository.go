package compliance

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryEvidenceRepository is a process-local EvidenceRepository
// backed by a map guarded by a mutex, intended for tests and other
// packages' fixtures -- never for production use, mirroring
// packages/privacy.InMemoryInventoryRepository's role exactly.
type InMemoryEvidenceRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*ControlEvidence
}

// NewInMemoryEvidenceRepository builds an empty
// InMemoryEvidenceRepository.
func NewInMemoryEvidenceRepository() *InMemoryEvidenceRepository {
	return &InMemoryEvidenceRepository{items: make(map[uuid.UUID]*ControlEvidence)}
}

// Create implements EvidenceRepository.
func (r *InMemoryEvidenceRepository) Create(_ context.Context, tenantID uuid.UUID, e *ControlEvidence) error {
	if e == nil {
		return ErrInvalidEvidence
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *e
	r.items[e.ID] = &cp
	return nil
}

// Get implements EvidenceRepository.
func (r *InMemoryEvidenceRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ControlEvidence, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.items[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrEvidenceNotFound
	}
	cp := *e
	return &cp, nil
}

// ListForControl implements EvidenceRepository.
func (r *InMemoryEvidenceRepository) ListForControl(_ context.Context, tenantID, controlID uuid.UUID) ([]ControlEvidence, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ControlEvidence, 0)
	for _, e := range r.items {
		if e.TenantID == tenantID && e.ControlID == controlID {
			out = append(out, *e)
		}
	}
	return out, nil
}

// ListAll implements EvidenceRepository.
func (r *InMemoryEvidenceRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]ControlEvidence, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ControlEvidence, 0)
	for _, e := range r.items {
		if e.TenantID == tenantID {
			out = append(out, *e)
		}
	}
	return out, nil
}

var _ EvidenceRepository = (*InMemoryEvidenceRepository)(nil)

// InMemoryProfileRepository is a process-local ProfileRepository
// backed by a map guarded by a mutex.
type InMemoryProfileRepository struct {
	mu       sync.RWMutex
	profiles map[uuid.UUID]*Profile
}

// NewInMemoryProfileRepository builds an empty
// InMemoryProfileRepository.
func NewInMemoryProfileRepository() *InMemoryProfileRepository {
	return &InMemoryProfileRepository{profiles: make(map[uuid.UUID]*Profile)}
}

// Set implements ProfileRepository.
func (r *InMemoryProfileRepository) Set(_ context.Context, tenantID uuid.UUID, p *Profile) error {
	if p == nil {
		return ErrInvalidProfile
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *p
	r.profiles[tenantID] = &cp
	return nil
}

// Get implements ProfileRepository.
func (r *InMemoryProfileRepository) Get(_ context.Context, tenantID uuid.UUID) (*Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[tenantID]
	if !ok {
		return nil, ErrProfileNotFound
	}
	cp := *p
	return &cp, nil
}

var _ ProfileRepository = (*InMemoryProfileRepository)(nil)
