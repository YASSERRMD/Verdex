package compliance

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// ControlRepository persists Control catalogue rows. Unlike the
// tenant-scoped evidence/profile/snapshot repositories elsewhere in
// this package, a Control catalogue entry is not itself tenant-scoped
// data -- it describes a requirement that may apply across many
// tenants -- mirroring how packages/jurisdiction's jurisdiction
// definitions are shared reference data rather than per-tenant rows.
// A deployment narrows which catalogued controls actually apply to it
// via Profile (profile.go), not by forking the catalogue.
type ControlRepository interface {
	Create(ctx context.Context, c *Control) error
	Get(ctx context.Context, id uuid.UUID) (*Control, error)
	GetByCode(ctx context.Context, code string) (*Control, error)
	List(ctx context.Context) ([]Control, error)
	ListByFramework(ctx context.Context, framework Framework) ([]Control, error)
	Update(ctx context.Context, c *Control) error
}

// InMemoryControlRepository is a process-local ControlRepository
// backed by a map guarded by a mutex, intended for tests, fixtures,
// and the seed catalogue (seed.go) -- never for production use,
// mirroring packages/privacy.InMemoryInventoryRepository's role
// exactly.
type InMemoryControlRepository struct {
	mu       sync.RWMutex
	controls map[uuid.UUID]*Control
}

// NewInMemoryControlRepository builds an empty
// InMemoryControlRepository.
func NewInMemoryControlRepository() *InMemoryControlRepository {
	return &InMemoryControlRepository{controls: make(map[uuid.UUID]*Control)}
}

// Create implements ControlRepository. Returns ErrDuplicateControl if
// c.Code already exists on a different control.
func (r *InMemoryControlRepository) Create(_ context.Context, c *Control) error {
	if c == nil {
		return ErrInvalidControl
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.controls {
		if existing.Code == c.Code && existing.ID != c.ID {
			return ErrDuplicateControl
		}
	}
	cp := *c
	r.controls[c.ID] = &cp
	return nil
}

// Get implements ControlRepository.
func (r *InMemoryControlRepository) Get(_ context.Context, id uuid.UUID) (*Control, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.controls[id]
	if !ok {
		return nil, ErrControlNotFound
	}
	cp := *c
	return &cp, nil
}

// GetByCode implements ControlRepository.
func (r *InMemoryControlRepository) GetByCode(_ context.Context, code string) (*Control, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.controls {
		if c.Code == code {
			cp := *c
			return &cp, nil
		}
	}
	return nil, ErrControlNotFound
}

// List implements ControlRepository.
func (r *InMemoryControlRepository) List(_ context.Context) ([]Control, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Control, 0, len(r.controls))
	for _, c := range r.controls {
		out = append(out, *c)
	}
	return out, nil
}

// ListByFramework implements ControlRepository.
func (r *InMemoryControlRepository) ListByFramework(_ context.Context, framework Framework) ([]Control, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Control, 0)
	for _, c := range r.controls {
		if c.Framework == framework {
			out = append(out, *c)
		}
	}
	return out, nil
}

// Update implements ControlRepository.
func (r *InMemoryControlRepository) Update(_ context.Context, c *Control) error {
	if c == nil {
		return ErrInvalidControl
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.controls[c.ID]; !ok {
		return ErrControlNotFound
	}
	cp := *c
	r.controls[c.ID] = &cp
	return nil
}

var _ ControlRepository = (*InMemoryControlRepository)(nil)
