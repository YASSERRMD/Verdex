package notifications

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryRepository is an in-process Repository implementation for
// tests and other packages' test fixtures, mirroring
// packages/caseversioning.InMemoryRepository's convention exactly. It
// is safe for concurrent use.
type InMemoryRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*Notification
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{records: make(map[uuid.UUID]*Notification)}
}

func cloneNotification(n *Notification) *Notification {
	if n == nil {
		return nil
	}
	cp := *n
	if n.CaseID != nil {
		id := *n.CaseID
		cp.CaseID = &id
	}
	if n.RelatedEntityID != nil {
		id := *n.RelatedEntityID
		cp.RelatedEntityID = &id
	}
	if n.ReadAt != nil {
		t := *n.ReadAt
		cp.ReadAt = &t
	}
	return &cp
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, tenantID uuid.UUID, n *Notification) error {
	if n == nil {
		return wrapf("InMemoryRepository.Create", ErrNilNotification)
	}
	if n.TenantID == uuid.Nil {
		n.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, n.TenantID); err != nil {
		return err
	}
	if err := n.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}

	r.records[n.ID] = cloneNotification(n)
	return nil
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n, ok := r.records[id]
	if !ok || n.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneNotification(n), nil
}

// ListForRecipient implements Repository.
func (r *InMemoryRepository) ListForRecipient(_ context.Context, tenantID, recipientID uuid.UUID, filter Filter) ([]*Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Notification, 0)
	for _, n := range r.records {
		if n.TenantID != tenantID || n.RecipientID != recipientID {
			continue
		}
		if filter.UnreadOnly && n.IsRead() {
			continue
		}
		if filter.Kind != "" && n.Kind != filter.Kind {
			continue
		}
		out = append(out, cloneNotification(n))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// UnreadCount implements Repository.
func (r *InMemoryRepository) UnreadCount(_ context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, n := range r.records {
		if n.TenantID != tenantID || n.RecipientID != recipientID {
			continue
		}
		if !n.IsRead() {
			count++
		}
	}
	return count, nil
}

// MarkRead implements Repository.
func (r *InMemoryRepository) MarkRead(_ context.Context, tenantID, recipientID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.records[id]
	if !ok || n.TenantID != tenantID || n.RecipientID != recipientID {
		return ErrNotFound
	}
	if n.ReadAt == nil {
		now := time.Now().UTC()
		n.ReadAt = &now
	}
	return nil
}

// MarkAllRead implements Repository.
func (r *InMemoryRepository) MarkAllRead(_ context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	count := 0
	for _, n := range r.records {
		if n.TenantID != tenantID || n.RecipientID != recipientID {
			continue
		}
		if n.ReadAt == nil {
			n.ReadAt = &now
			count++
		}
	}
	return count, nil
}

var _ Repository = (*InMemoryRepository)(nil)

// preferenceKey uniquely identifies a Preference row within
// InMemoryPreferenceRepository's map.
type preferenceKey struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	kind     Kind
}

// InMemoryPreferenceRepository is an in-process PreferenceRepository
// implementation for tests and other packages' test fixtures. It is
// safe for concurrent use.
type InMemoryPreferenceRepository struct {
	mu      sync.RWMutex
	records map[preferenceKey]*Preference
}

// NewInMemoryPreferenceRepository builds an empty
// InMemoryPreferenceRepository.
func NewInMemoryPreferenceRepository() *InMemoryPreferenceRepository {
	return &InMemoryPreferenceRepository{records: make(map[preferenceKey]*Preference)}
}

func clonePreference(p *Preference) *Preference {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Channels = append([]Channel(nil), p.Channels...)
	return &cp
}

// Upsert implements PreferenceRepository.
func (r *InMemoryPreferenceRepository) Upsert(_ context.Context, tenantID uuid.UUID, p *Preference) error {
	if p == nil {
		return wrapf("InMemoryPreferenceRepository.Upsert", ErrNilPreference)
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := preferenceKey{tenantID: tenantID, userID: p.UserID, kind: p.Kind}
	r.records[key] = clonePreference(p)
	return nil
}

// Get implements PreferenceRepository.
func (r *InMemoryPreferenceRepository) Get(_ context.Context, tenantID, userID uuid.UUID, kind Kind) (*Preference, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := preferenceKey{tenantID: tenantID, userID: userID, kind: kind}
	p, ok := r.records[key]
	if !ok {
		return nil, ErrNotFound
	}
	return clonePreference(p), nil
}

// ListForUser implements PreferenceRepository.
func (r *InMemoryPreferenceRepository) ListForUser(_ context.Context, tenantID, userID uuid.UUID) ([]*Preference, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Preference, 0)
	for key, p := range r.records {
		if key.tenantID != tenantID || key.userID != userID {
			continue
		}
		out = append(out, clonePreference(p))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

var _ PreferenceRepository = (*InMemoryPreferenceRepository)(nil)
