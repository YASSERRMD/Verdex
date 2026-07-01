package eval

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ResultStore is the persistence contract for EvalReports.
//
// Implementations must be safe for concurrent use from multiple goroutines.
type ResultStore interface {
	// SaveReport persists report.  The store assigns a unique runID derived
	// from report.RunAt if none is provided externally; callers can use
	// LoadReport with the same runID to retrieve it later.
	SaveReport(ctx context.Context, report EvalReport) error

	// LoadReport retrieves the EvalReport associated with runID.  Returns an
	// error wrapping ErrEvalFailed when no report with that ID exists.
	LoadReport(ctx context.Context, runID string) (*EvalReport, error)

	// ListReports returns up to limit EvalReports ordered by RunAt descending
	// (most recent first).  Pass 0 or a negative value to return all reports.
	ListReports(ctx context.Context, limit int) ([]EvalReport, error)
}

// InMemoryResultStore is a thread-safe in-process implementation of
// ResultStore.  All data is lost when the process exits.
type InMemoryResultStore struct {
	mu      sync.RWMutex
	reports map[string]EvalReport // keyed by runID
	order   []string              // insertion order by runID (oldest first)
}

// NewInMemoryResultStore returns a ready-to-use InMemoryResultStore.
func NewInMemoryResultStore() *InMemoryResultStore {
	return &InMemoryResultStore{
		reports: make(map[string]EvalReport),
	}
}

// SaveReport stores report using a runID derived from RunAt.
//
// If report.RunAt is zero the current UTC time is used.  Reports saved at the
// same nanosecond are disambiguated with a monotonic counter suffix.
func (s *InMemoryResultStore) SaveReport(_ context.Context, report EvalReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if report.RunAt.IsZero() {
		report.RunAt = time.Now().UTC()
	}
	runID := runIDFromTime(report.RunAt)

	// Handle collisions.
	if _, exists := s.reports[runID]; exists {
		runID = fmt.Sprintf("%s-%d", runID, len(s.reports))
	}

	s.reports[runID] = report
	s.order = append(s.order, runID)
	return nil
}

// LoadReport retrieves the EvalReport for the given runID.
func (s *InMemoryResultStore) LoadReport(_ context.Context, runID string) (*EvalReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.reports[runID]
	if !ok {
		return nil, fmt.Errorf("%w: report %q not found", ErrEvalFailed, runID)
	}
	cp := r
	return &cp, nil
}

// ListReports returns up to limit reports ordered by RunAt descending.
func (s *InMemoryResultStore) ListReports(_ context.Context, limit int) ([]EvalReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]EvalReport, 0, len(s.reports))
	for _, r := range s.reports {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].RunAt.After(all[j].RunAt)
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// runIDFromTime derives a stable string key from a time value.
func runIDFromTime(t time.Time) string {
	return t.UTC().Format("20060102T150405.000000000Z")
}
