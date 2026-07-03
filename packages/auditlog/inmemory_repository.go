package auditlog

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryRepository is a process-local Repository backed by a slice
// guarded by a mutex, intended for tests and other packages' fixtures
// — never for production use, mirroring
// packages/keymanagement.InMemoryRepository's role exactly.
type InMemoryRepository struct {
	mu     sync.RWMutex
	events map[uuid.UUID][]Event
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{events: make(map[uuid.UUID][]Event)}
}

// Append implements Repository.
func (r *InMemoryRepository) Append(_ context.Context, tenantID uuid.UUID, event *Event) error {
	if event == nil {
		return wrapf("InMemoryRepository.Append", ErrNilEvent)
	}
	if event.TenantID == uuid.Nil {
		event.TenantID = tenantID
	}
	if event.TenantID != tenantID {
		return wrapf("InMemoryRepository.Append", ErrCrossTenantAccess)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[tenantID] = append(r.events[tenantID], *event)
	return nil
}

// Last implements Repository.
func (r *InMemoryRepository) Last(_ context.Context, tenantID uuid.UUID) (*Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.events[tenantID]
	if len(list) == 0 {
		return nil, nil
	}
	last := list[len(list)-1]
	return &last, nil
}

// ListAll implements Repository.
func (r *InMemoryRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Event, len(r.events[tenantID]))
	copy(out, r.events[tenantID])
	sortByChainOrder(out)
	return out, nil
}

// Query implements Repository.
func (r *InMemoryRepository) Query(_ context.Context, tenantID uuid.UUID, filter Filter) ([]Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}

	var out []Event
	for _, e := range r.events[tenantID] {
		if !matchesFilter(e, filter) {
			continue
		}
		out = append(out, e)
	}
	sortByChainOrder(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PurgeBefore implements Repository.
func (r *InMemoryRepository) PurgeBefore(_ context.Context, tenantID uuid.UUID, cutoff time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.events[tenantID]
	kept := make([]Event, 0, len(list))
	removed := 0
	for _, e := range list {
		if e.Time.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	r.events[tenantID] = kept
	return removed, nil
}

func matchesFilter(e Event, f Filter) bool {
	if f.Actor != "" && e.Actor != f.Actor {
		return false
	}
	if f.CaseID != uuid.Nil && e.CaseID != f.CaseID {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
		return false
	}
	if len(f.Kinds) > 0 {
		found := false
		for _, k := range f.Kinds {
			if e.Kind == k {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if !f.Since.IsZero() && e.Time.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && e.Time.After(f.Until) {
		return false
	}
	return true
}

func sortByChainOrder(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Time.Equal(events[j].Time) {
			return events[i].ID.String() < events[j].ID.String()
		}
		return events[i].Time.Before(events[j].Time)
	})
}

var _ Repository = (*InMemoryRepository)(nil)
