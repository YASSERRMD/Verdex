package jurisdiction

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// LookupService provides read-only access to the jurisdiction registry.
type LookupService interface {
	// GetByID returns the jurisdiction with the given UUID, or
	// ErrJurisdictionNotFound if no match exists.
	GetByID(ctx context.Context, id uuid.UUID) (Jurisdiction, error)

	// GetByCountry returns all jurisdictions registered for the given ISO
	// 3166-1 alpha-2 country code.  Returns an empty slice (not an error) if
	// none are found.
	GetByCountry(ctx context.Context, countryCode string) ([]Jurisdiction, error)

	// ListAll returns every jurisdiction in the registry.
	ListAll(ctx context.Context) ([]Jurisdiction, error)

	// Search performs a case-insensitive substring search against CourtName,
	// CountryName, and CountryCode.  Returns matching jurisdictions ordered by
	// insertion order.
	Search(ctx context.Context, query string) ([]Jurisdiction, error)
}

// InMemoryLookupService is a thread-safe, in-memory implementation of
// LookupService.  It is suitable for seeded data, unit tests, and read-only
// look-ups where a persistent store is unavailable.
type InMemoryLookupService struct {
	mu   sync.RWMutex
	data []Jurisdiction
	idx  map[uuid.UUID]int // uuid -> slice index
}

// NewInMemoryLookupService creates a new InMemoryLookupService populated with
// the provided jurisdictions.  Any jurisdiction that fails Validate() is
// silently skipped.
func NewInMemoryLookupService(jurisdictions []Jurisdiction) *InMemoryLookupService {
	svc := &InMemoryLookupService{
		idx: make(map[uuid.UUID]int, len(jurisdictions)),
	}
	for _, j := range jurisdictions {
		if Validate(j) == nil {
			svc.idx[j.ID] = len(svc.data)
			svc.data = append(svc.data, j)
		}
	}
	return svc
}

// GetByID implements LookupService.
func (s *InMemoryLookupService) GetByID(_ context.Context, id uuid.UUID) (Jurisdiction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	i, ok := s.idx[id]
	if !ok {
		return Jurisdiction{}, ErrJurisdictionNotFound
	}
	return s.data[i], nil
}

// GetByCountry implements LookupService.
func (s *InMemoryLookupService) GetByCountry(_ context.Context, countryCode string) ([]Jurisdiction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	code := strings.ToUpper(strings.TrimSpace(countryCode))
	var out []Jurisdiction
	for _, j := range s.data {
		if j.CountryCode == code {
			out = append(out, j)
		}
	}
	return out, nil
}

// ListAll implements LookupService.
func (s *InMemoryLookupService) ListAll(_ context.Context) ([]Jurisdiction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Jurisdiction, len(s.data))
	copy(out, s.data)
	return out, nil
}

// Search implements LookupService.
func (s *InMemoryLookupService) Search(_ context.Context, query string) ([]Jurisdiction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]Jurisdiction, len(s.data))
		copy(out, s.data)
		return out, nil
	}

	var out []Jurisdiction
	for _, j := range s.data {
		if strings.Contains(strings.ToLower(j.CourtName), q) ||
			strings.Contains(strings.ToLower(j.CountryName), q) ||
			strings.Contains(strings.ToLower(j.CountryCode), q) {
			out = append(out, j)
		}
	}
	return out, nil
}
