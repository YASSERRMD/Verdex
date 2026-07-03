package caselifecycle

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// InMemoryRepository is an in-process Repository implementation for
// tests and for other packages' test fixtures, mirroring
// packages/timeline/store.go's in-memory-store convention. It is safe
// for concurrent use.
type InMemoryRepository struct {
	mu          sync.RWMutex
	cases       map[uuid.UUID]*Case
	transitions map[uuid.UUID][]*TransitionRecord // keyed by CaseID
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		cases:       make(map[uuid.UUID]*Case),
		transitions: make(map[uuid.UUID][]*TransitionRecord),
	}
}

func cloneCase(c *Case) *Case {
	if c == nil {
		return nil
	}
	cp := *c
	cp.Metadata = make(map[string]string, len(c.Metadata))
	for k, v := range c.Metadata {
		cp.Metadata[k] = v
	}
	if c.ArchivedAt != nil {
		t := *c.ArchivedAt
		cp.ArchivedAt = &t
	}
	return &cp
}

func cloneTransition(r *TransitionRecord) *TransitionRecord {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}

// Create implements Repository.
func (repo *InMemoryRepository) Create(_ context.Context, tenantID uuid.UUID, c *Case) error {
	if c == nil {
		return wrapf("InMemoryRepository.Create", ErrInvalidCase)
	}
	if c.TenantID == uuid.Nil {
		c.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	repo.cases[c.ID] = cloneCase(c)
	return nil
}

// Get implements Repository.
func (repo *InMemoryRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Case, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	c, ok := repo.cases[id]
	if !ok || c.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneCase(c), nil
}

// List implements Repository.
func (repo *InMemoryRepository) List(_ context.Context, tenantID uuid.UUID, filter CaseFilter) ([]*Case, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	var out []*Case
	for _, c := range repo.cases {
		if c.TenantID != tenantID {
			continue
		}
		if filter.State != "" && c.State != filter.State {
			continue
		}
		if filter.JurisdictionID != uuid.Nil && c.JurisdictionID != filter.JurisdictionID {
			continue
		}
		if filter.CategoryID != "" && c.CategoryID != filter.CategoryID {
			continue
		}
		out = append(out, cloneCase(c))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID.String() < out[j].ID.String()
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// Update implements Repository.
func (repo *InMemoryRepository) Update(_ context.Context, tenantID uuid.UUID, c *Case) error {
	if c == nil {
		return wrapf("InMemoryRepository.Update", ErrInvalidCase)
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()

	existing, ok := repo.cases[c.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrNotFound
	}
	repo.cases[c.ID] = cloneCase(c)
	return nil
}

// AppendTransition implements Repository.
func (repo *InMemoryRepository) AppendTransition(_ context.Context, tenantID uuid.UUID, r *TransitionRecord) error {
	if r == nil {
		return wrapf("InMemoryRepository.AppendTransition", ErrInvalidCase)
	}
	if err := requireMatchingTenant(tenantID, r.TenantID); err != nil {
		return err
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()

	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	repo.transitions[r.CaseID] = append(repo.transitions[r.CaseID], cloneTransition(r))
	return nil
}

// ListTransitions implements Repository.
func (repo *InMemoryRepository) ListTransitions(_ context.Context, tenantID, caseID uuid.UUID) ([]*TransitionRecord, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	c, ok := repo.cases[caseID]
	if !ok || c.TenantID != tenantID {
		return nil, ErrNotFound
	}

	records := repo.transitions[caseID]
	out := make([]*TransitionRecord, 0, len(records))
	for _, r := range records {
		if r.TenantID != tenantID {
			continue
		}
		out = append(out, cloneTransition(r))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.Before(out[j].OccurredAt)
	})
	return out, nil
}

var _ Repository = (*InMemoryRepository)(nil)
