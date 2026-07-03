package reasoningeval

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Store is the persistence contract for evaluation history: automated
// QualityScores, human ExpertReviews, and Alerts raised over time,
// mirroring packages/eval.ResultStore's shape extended to reasoningeval's
// three record kinds.
//
// Implementations must be safe for concurrent use from multiple
// goroutines.
type Store interface {
	// SaveScore persists score.
	SaveScore(ctx context.Context, score QualityScore) error
	// ListScores returns every persisted QualityScore, most recent first.
	// If runID is non-empty, only scores with that RunID are returned. If
	// jurisdictionCode is non-empty, only scores with that
	// JurisdictionCode are returned. Either filter may be combined with
	// the other; pass "" to skip a filter.
	ListScores(ctx context.Context, runID, jurisdictionCode string) ([]QualityScore, error)

	// SaveReview persists review. Returns an error if review.Validate()
	// fails.
	SaveReview(ctx context.Context, review ExpertReview) error
	// GetReview retrieves the ExpertReview with the given reviewID.
	// Returns an error wrapping ErrReviewNotFound if none exists.
	GetReview(ctx context.Context, reviewID string) (ExpertReview, error)
	// ListReviews returns every persisted ExpertReview for caseID, most
	// recent first. If caseID is empty, every review is returned.
	ListReviews(ctx context.Context, caseID string) ([]ExpertReview, error)

	// SaveAlert persists alert.
	SaveAlert(ctx context.Context, alert Alert) error
	// ListAlerts returns every persisted Alert, most recent first.
	ListAlerts(ctx context.Context) ([]Alert, error)
}

// InMemoryStore is a thread-safe in-process implementation of Store. All
// data is lost when the process exits, mirroring
// packages/eval.InMemoryResultStore's convention.
type InMemoryStore struct {
	mu      sync.RWMutex
	scores  []QualityScore
	reviews map[string]ExpertReview
	revOrd  []string
	alerts  []Alert
}

// NewInMemoryStore returns a ready-to-use InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		reviews: make(map[string]ExpertReview),
	}
}

// SaveScore implements Store.
func (s *InMemoryStore) SaveScore(_ context.Context, score QualityScore) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scores = append(s.scores, score)
	return nil
}

// ListScores implements Store.
func (s *InMemoryStore) ListScores(_ context.Context, runID, jurisdictionCode string) ([]QualityScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]QualityScore, 0, len(s.scores))
	for _, sc := range s.scores {
		if runID != "" && sc.RunID != runID {
			continue
		}
		if jurisdictionCode != "" && sc.JurisdictionCode != jurisdictionCode {
			continue
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScoredAt.After(out[j].ScoredAt) })
	return out, nil
}

// SaveReview implements Store.
func (s *InMemoryStore) SaveReview(_ context.Context, review ExpertReview) error {
	if err := review.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.reviews[review.ReviewID]; !exists {
		s.revOrd = append(s.revOrd, review.ReviewID)
	}
	s.reviews[review.ReviewID] = review
	return nil
}

// GetReview implements Store.
func (s *InMemoryStore) GetReview(_ context.Context, reviewID string) (ExpertReview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.reviews[reviewID]
	if !ok {
		return ExpertReview{}, fmt.Errorf("%w: review %q not found", ErrReviewNotFound, reviewID)
	}
	return r, nil
}

// ListReviews implements Store.
func (s *InMemoryStore) ListReviews(_ context.Context, caseID string) ([]ExpertReview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ExpertReview, 0, len(s.revOrd))
	for _, id := range s.revOrd {
		r := s.reviews[id]
		if caseID != "" && r.CaseID != caseID {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReviewedAt.After(out[j].ReviewedAt) })
	return out, nil
}

// SaveAlert implements Store.
func (s *InMemoryStore) SaveAlert(_ context.Context, alert Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, alert)
	return nil
}

// ListAlerts implements Store.
func (s *InMemoryStore) ListAlerts(_ context.Context) ([]Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Alert, len(s.alerts))
	copy(out, s.alerts)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
