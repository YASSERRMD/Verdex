package evidence

import (
	"context"
	"sync"
)

// ClassificationStore defines the persistence contract for Classification
// records, keyed by segment ID. This mirrors packages/jurisdiction's
// Repository interface pattern: a small, storage-agnostic contract that a
// relational, document, or (as implemented here) in-memory backend can
// satisfy.
type ClassificationStore interface {
	// Save persists c, keyed by c.SegmentID, replacing any existing record
	// for that segment. Returns ErrEmptyInput if c.SegmentID is empty.
	Save(ctx context.Context, c Classification) error

	// Get retrieves the Classification stored for segmentID. Returns
	// ErrSegmentNotFound if no record exists for segmentID.
	Get(ctx context.Context, segmentID string) (Classification, error)

	// List returns every stored Classification, in no particular order.
	// Returns an empty (nil) slice, not an error, when the store is empty.
	List(ctx context.Context) ([]Classification, error)

	// Delete removes the record for segmentID. Returns ErrSegmentNotFound
	// if no record exists for segmentID.
	Delete(ctx context.Context, segmentID string) error
}

// InMemoryClassificationStore is the default ClassificationStore
// implementation: a mutex-guarded in-memory map with no real database
// dependency, suitable for tests and for wiring EvidenceService end-to-end
// before a durable backend is introduced in a later phase.
type InMemoryClassificationStore struct {
	mu   sync.RWMutex
	data map[string]Classification
}

// NewInMemoryClassificationStore constructs an empty
// InMemoryClassificationStore.
func NewInMemoryClassificationStore() *InMemoryClassificationStore {
	return &InMemoryClassificationStore{data: make(map[string]Classification)}
}

// Save implements ClassificationStore.
func (s *InMemoryClassificationStore) Save(_ context.Context, c Classification) error {
	if c.SegmentID == "" {
		return ErrEmptyInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]Classification)
	}
	s.data[c.SegmentID] = c
	return nil
}

// Get implements ClassificationStore.
func (s *InMemoryClassificationStore) Get(_ context.Context, segmentID string) (Classification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data[segmentID]
	if !ok {
		return Classification{}, ErrSegmentNotFound
	}
	return c, nil
}

// List implements ClassificationStore.
func (s *InMemoryClassificationStore) List(_ context.Context) ([]Classification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data) == 0 {
		return nil, nil
	}
	out := make([]Classification, 0, len(s.data))
	for _, c := range s.data {
		out = append(out, c)
	}
	return out, nil
}

// Delete implements ClassificationStore.
func (s *InMemoryClassificationStore) Delete(_ context.Context, segmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[segmentID]; !ok {
		return ErrSegmentNotFound
	}
	delete(s.data, segmentID)
	return nil
}
