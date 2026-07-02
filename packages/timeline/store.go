package timeline

import (
	"context"
	"sync"
)

// CaseGraph is the full party/timeline graph persisted for a single case:
// its parties, extracted facts, assembled events, derived claims,
// detected conflicts, and party relationships.
type CaseGraph struct {
	// CaseID identifies the case this graph belongs to.
	CaseID string

	// Parties lists every Party registered for the case.
	Parties []Party

	// Facts lists every PartyFact attributed for the case.
	Facts []PartyFact

	// Events lists every Event extracted for the case, in Timeline
	// assembly order (see assemble.go) once assembled.
	Events []Event

	// Claims lists every Claim linked for the case.
	Claims []Claim

	// Conflicts lists every Conflict detected for the case.
	Conflicts []Conflict

	// Relationships lists every Relationship recorded between the case's
	// parties.
	Relationships []Relationship
}

// TimelineStore defines the persistence contract for a case's party and
// timeline graph, keyed by case ID. This mirrors packages/evidence/store.go's
// ClassificationStore pattern: a small, storage-agnostic contract that a
// relational, document, or (as implemented here) in-memory backend can
// satisfy.
type TimelineStore interface {
	// SaveGraph persists graph, keyed by graph.CaseID, replacing any
	// existing record for that case. Returns ErrEmptyInput if graph.CaseID
	// is empty.
	SaveGraph(ctx context.Context, graph CaseGraph) error

	// GetGraph retrieves the CaseGraph stored for caseID. Returns
	// ErrCaseNotFound if no record exists for caseID.
	GetGraph(ctx context.Context, caseID string) (CaseGraph, error)

	// DeleteGraph removes the record for caseID. Returns ErrCaseNotFound
	// if no record exists for caseID.
	DeleteGraph(ctx context.Context, caseID string) error

	// ListCaseIDs returns every case ID with a stored graph, in no
	// particular order. Returns an empty (nil) slice, not an error, when
	// the store is empty.
	ListCaseIDs(ctx context.Context) ([]string, error)
}

// InMemoryTimelineStore is the default TimelineStore implementation: a
// mutex-guarded in-memory map with no real database dependency, suitable
// for tests and for wiring TimelineService end-to-end before a durable
// backend is introduced in a later phase.
type InMemoryTimelineStore struct {
	mu   sync.RWMutex
	data map[string]CaseGraph
}

// NewInMemoryTimelineStore constructs an empty InMemoryTimelineStore.
func NewInMemoryTimelineStore() *InMemoryTimelineStore {
	return &InMemoryTimelineStore{data: make(map[string]CaseGraph)}
}

// SaveGraph implements TimelineStore.
func (s *InMemoryTimelineStore) SaveGraph(_ context.Context, graph CaseGraph) error {
	if graph.CaseID == "" {
		return ErrEmptyInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]CaseGraph)
	}
	s.data[graph.CaseID] = graph
	return nil
}

// GetGraph implements TimelineStore.
func (s *InMemoryTimelineStore) GetGraph(_ context.Context, caseID string) (CaseGraph, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.data[caseID]
	if !ok {
		return CaseGraph{}, ErrCaseNotFound
	}
	return g, nil
}

// DeleteGraph implements TimelineStore.
func (s *InMemoryTimelineStore) DeleteGraph(_ context.Context, caseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[caseID]; !ok {
		return ErrCaseNotFound
	}
	delete(s.data, caseID)
	return nil
}

// ListCaseIDs implements TimelineStore.
func (s *InMemoryTimelineStore) ListCaseIDs(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(s.data))
	for id := range s.data {
		out = append(out, id)
	}
	return out, nil
}
