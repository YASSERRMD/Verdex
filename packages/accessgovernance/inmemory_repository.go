package accessgovernance

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryPolicyRepository is a process-local PolicyRepository backed
// by a map guarded by a mutex, intended for tests and other packages'
// fixtures -- never for production use, mirroring
// packages/keymanagement.InMemoryRepository's role exactly.
type InMemoryPolicyRepository struct {
	mu       sync.RWMutex
	policies map[uuid.UUID]*Policy
}

// NewInMemoryPolicyRepository builds an empty InMemoryPolicyRepository.
func NewInMemoryPolicyRepository() *InMemoryPolicyRepository {
	return &InMemoryPolicyRepository{policies: make(map[uuid.UUID]*Policy)}
}

func (r *InMemoryPolicyRepository) Create(_ context.Context, tenantID uuid.UUID, p *Policy) error {
	if p == nil {
		return ErrNilPolicy
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
	r.policies[p.ID] = &cp
	return nil
}

func (r *InMemoryPolicyRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.policies[id]
	if !ok || p.TenantID != tenantID {
		return nil, ErrPolicyNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *InMemoryPolicyRepository) List(_ context.Context, tenantID uuid.UUID) ([]Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Policy, 0)
	for _, p := range r.policies {
		if p.TenantID == tenantID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *InMemoryPolicyRepository) Update(_ context.Context, tenantID uuid.UUID, p *Policy) error {
	if p == nil {
		return ErrNilPolicy
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.policies[p.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrPolicyNotFound
	}
	cp := *p
	cp.TenantID = tenantID
	r.policies[p.ID] = &cp
	return nil
}

var _ PolicyRepository = (*InMemoryPolicyRepository)(nil)

// InMemoryCaseGrantRepository is a process-local CaseGrantRepository.
type InMemoryCaseGrantRepository struct {
	mu     sync.RWMutex
	grants map[uuid.UUID]*CaseGrant
}

// NewInMemoryCaseGrantRepository builds an empty
// InMemoryCaseGrantRepository.
func NewInMemoryCaseGrantRepository() *InMemoryCaseGrantRepository {
	return &InMemoryCaseGrantRepository{grants: make(map[uuid.UUID]*CaseGrant)}
}

func (r *InMemoryCaseGrantRepository) Create(_ context.Context, tenantID uuid.UUID, g *CaseGrant) error {
	if g == nil {
		return ErrInvalidGrant
	}
	if g.TenantID == uuid.Nil {
		g.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, g.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *g
	r.grants[g.ID] = &cp
	return nil
}

func (r *InMemoryCaseGrantRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*CaseGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.grants[id]
	if !ok || g.TenantID != tenantID {
		return nil, ErrGrantNotFound
	}
	cp := *g
	return &cp, nil
}

func (r *InMemoryCaseGrantRepository) ListForCase(_ context.Context, tenantID, caseID uuid.UUID) ([]CaseGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CaseGrant, 0)
	for _, g := range r.grants {
		if g.TenantID == tenantID && g.CaseID == caseID {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (r *InMemoryCaseGrantRepository) ListActive(_ context.Context, tenantID uuid.UUID, now time.Time) ([]CaseGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CaseGrant, 0)
	for _, g := range r.grants {
		if g.TenantID == tenantID && !g.IsExpired(now) {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (r *InMemoryCaseGrantRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]CaseGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CaseGrant, 0)
	for _, g := range r.grants {
		if g.TenantID == tenantID {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (r *InMemoryCaseGrantRepository) Revoke(_ context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.grants[id]
	if !ok || g.TenantID != tenantID {
		return ErrGrantNotFound
	}
	t := revokedAt
	g.RevokedAt = &t
	return nil
}

var _ CaseGrantRepository = (*InMemoryCaseGrantRepository)(nil)

// InMemoryGrantRepository is a process-local GrantRepository for JIT
// elevation Grant records.
type InMemoryGrantRepository struct {
	mu     sync.RWMutex
	grants map[uuid.UUID]*Grant
}

// NewInMemoryGrantRepository builds an empty InMemoryGrantRepository.
func NewInMemoryGrantRepository() *InMemoryGrantRepository {
	return &InMemoryGrantRepository{grants: make(map[uuid.UUID]*Grant)}
}

func (r *InMemoryGrantRepository) Create(_ context.Context, tenantID uuid.UUID, g *Grant) error {
	if g == nil {
		return ErrInvalidGrant
	}
	if g.TenantID == uuid.Nil {
		g.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, g.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *g
	r.grants[g.ID] = &cp
	return nil
}

func (r *InMemoryGrantRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.grants[id]
	if !ok || g.TenantID != tenantID {
		return nil, ErrGrantNotFound
	}
	cp := *g
	return &cp, nil
}

func (r *InMemoryGrantRepository) ListActive(_ context.Context, tenantID uuid.UUID, now time.Time) ([]Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Grant, 0)
	for _, g := range r.grants {
		if g.TenantID == tenantID && !g.IsExpired(now) {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (r *InMemoryGrantRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Grant, 0)
	for _, g := range r.grants {
		if g.TenantID == tenantID {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (r *InMemoryGrantRepository) Revoke(_ context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.grants[id]
	if !ok || g.TenantID != tenantID {
		return ErrGrantNotFound
	}
	t := revokedAt
	g.RevokedAt = &t
	return nil
}

var _ GrantRepository = (*InMemoryGrantRepository)(nil)

// InMemoryReviewRepository is a process-local ReviewRepository.
type InMemoryReviewRepository struct {
	mu      sync.RWMutex
	reviews map[uuid.UUID]*Review
}

// NewInMemoryReviewRepository builds an empty InMemoryReviewRepository.
func NewInMemoryReviewRepository() *InMemoryReviewRepository {
	return &InMemoryReviewRepository{reviews: make(map[uuid.UUID]*Review)}
}

func (r *InMemoryReviewRepository) Create(_ context.Context, tenantID uuid.UUID, rv *Review) error {
	if rv == nil {
		return ErrReviewNotFound
	}
	if rv.TenantID == uuid.Nil {
		rv.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rv.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rv
	r.reviews[rv.ID] = &cp
	return nil
}

func (r *InMemoryReviewRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Review, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rv, ok := r.reviews[id]
	if !ok || rv.TenantID != tenantID {
		return nil, ErrReviewNotFound
	}
	cp := *rv
	return &cp, nil
}

func (r *InMemoryReviewRepository) ListDue(_ context.Context, tenantID uuid.UUID, asOf time.Time) ([]Review, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Review, 0)
	for _, rv := range r.reviews {
		if rv.TenantID == tenantID && rv.IsPending() && !rv.DueAt.After(asOf) {
			out = append(out, *rv)
		}
	}
	return out, nil
}

func (r *InMemoryReviewRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]Review, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Review, 0)
	for _, rv := range r.reviews {
		if rv.TenantID == tenantID {
			out = append(out, *rv)
		}
	}
	return out, nil
}

func (r *InMemoryReviewRepository) Update(_ context.Context, tenantID uuid.UUID, rv *Review) error {
	if rv == nil {
		return ErrReviewNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.reviews[rv.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrReviewNotFound
	}
	cp := *rv
	cp.TenantID = tenantID
	r.reviews[rv.ID] = &cp
	return nil
}

var _ ReviewRepository = (*InMemoryReviewRepository)(nil)
