package keymanagement

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// requireMatchingTenant returns ErrCrossTenantAccess if entityTenantID
// is set and does not equal scopeTenantID, mirroring
// packages/notifications and packages/caseversioning's unexported
// helper of the same name and behavior.
func requireMatchingTenant(scopeTenantID, entityTenantID uuid.UUID) error {
	if entityTenantID != uuid.Nil && entityTenantID != scopeTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}

// InMemoryRepository is an in-process Repository implementation for
// tests and other packages' test fixtures, mirroring
// packages/notifications.InMemoryRepository's convention exactly. It
// is safe for concurrent use.
type InMemoryRepository struct {
	mu      sync.RWMutex
	records map[string]*KeyMetadata // keyed by KeyMetadata.ID
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{records: make(map[string]*KeyMetadata)}
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, tenantID uuid.UUID, m *KeyMetadata) error {
	if m == nil {
		return wrapf("InMemoryRepository.Create", ErrNilKeyMetadata)
	}
	if m.TenantID == uuid.Nil {
		m.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, m.TenantID); err != nil {
		return err
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	if err := m.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if m.State == KeyStateActive {
		for _, existing := range r.records {
			if existing.TenantID == tenantID && existing.State == KeyStateActive && existing.ID != m.ID {
				return wrapf("InMemoryRepository.Create", ErrInvalidKeyState)
			}
		}
	}

	r.records[m.ID] = m.clone()
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, tenantID uuid.UUID, id string) (*KeyMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.records[id]
	if !ok || m.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return m.clone(), nil
}

// GetActive implements Repository.
func (r *InMemoryRepository) GetActive(_ context.Context, tenantID uuid.UUID) (*KeyMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, m := range r.records {
		if m.TenantID == tenantID && m.State == KeyStateActive {
			return m.clone(), nil
		}
	}
	return nil, ErrNoActiveKey
}

// ListForTenant implements Repository.
func (r *InMemoryRepository) ListForTenant(_ context.Context, tenantID uuid.UUID, filter Filter) ([]*KeyMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*KeyMetadata, 0)
	for _, m := range r.records {
		if m.TenantID != tenantID {
			continue
		}
		if filter.State != "" && m.State != filter.State {
			continue
		}
		out = append(out, m.clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version > out[j].Version
	})
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// UpdateState implements Repository.
func (r *InMemoryRepository) UpdateState(_ context.Context, tenantID uuid.UUID, id string, newState KeyState) error {
	if !newState.IsValid() {
		return ErrInvalidKeyState
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.records[id]
	if !ok || m.TenantID != tenantID {
		return ErrNotFound
	}

	if newState == KeyStateActive {
		for _, existing := range r.records {
			if existing.TenantID == tenantID && existing.State == KeyStateActive && existing.ID != id {
				return wrapf("InMemoryRepository.UpdateState", ErrInvalidKeyState)
			}
		}
	}

	m.State = newState
	return nil
}

// MaxVersion implements Repository.
func (r *InMemoryRepository) MaxVersion(_ context.Context, tenantID uuid.UUID) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	max := 0
	for _, m := range r.records {
		if m.TenantID == tenantID && m.Version > max {
			max = m.Version
		}
	}
	return max, nil
}

var _ Repository = (*InMemoryRepository)(nil)

// InMemoryAuditRepository is an in-process AuditRepository
// implementation for tests, safe for concurrent use.
type InMemoryAuditRepository struct {
	mu      sync.RWMutex
	entries []*AuditEntry
}

// NewInMemoryAuditRepository builds an empty InMemoryAuditRepository.
func NewInMemoryAuditRepository() *InMemoryAuditRepository {
	return &InMemoryAuditRepository{}
}

// Record implements AuditRepository.
func (r *InMemoryAuditRepository) Record(_ context.Context, tenantID uuid.UUID, entry *AuditEntry) error {
	if entry == nil {
		return wrapf("InMemoryAuditRepository.Record", ErrNilKeyMetadata)
	}
	if entry.TenantID == uuid.Nil {
		entry.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, entry.TenantID); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cp := *entry
	r.entries = append(r.entries, &cp)
	return nil
}

// ListForTenant implements AuditRepository.
func (r *InMemoryAuditRepository) ListForTenant(_ context.Context, tenantID uuid.UUID, limit int) ([]*AuditEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*AuditEntry, 0)
	for _, e := range r.entries {
		if e.TenantID != tenantID {
			continue
		}
		cp := *e
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

var _ AuditRepository = (*InMemoryAuditRepository)(nil)
