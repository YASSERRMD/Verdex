package garelease

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryReleaseCandidateRepository is a process-local
// ReleaseCandidateRepository backed by a map guarded by a mutex,
// intended for tests and fixtures -- never for production use,
// mirroring packages/compliance.InMemoryControlRepository's role
// exactly.
type InMemoryReleaseCandidateRepository struct {
	mu         sync.RWMutex
	candidates map[uuid.UUID]*ReleaseCandidate
}

// NewInMemoryReleaseCandidateRepository builds an empty
// InMemoryReleaseCandidateRepository.
func NewInMemoryReleaseCandidateRepository() *InMemoryReleaseCandidateRepository {
	return &InMemoryReleaseCandidateRepository{candidates: make(map[uuid.UUID]*ReleaseCandidate)}
}

// Create implements ReleaseCandidateRepository.
func (r *InMemoryReleaseCandidateRepository) Create(_ context.Context, c *ReleaseCandidate) error {
	if c == nil {
		return ErrInvalidCandidate
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.candidates {
		if existing.Version == c.Version && existing.ID != c.ID {
			return wrapf("InMemoryReleaseCandidateRepository.Create", ErrInvalidCandidate)
		}
	}
	cp := *c
	r.candidates[c.ID] = &cp
	return nil
}

// Get implements ReleaseCandidateRepository.
func (r *InMemoryReleaseCandidateRepository) Get(_ context.Context, id uuid.UUID) (*ReleaseCandidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.candidates[id]
	if !ok {
		return nil, ErrCandidateNotFound
	}
	cp := *c
	return &cp, nil
}

// GetByVersion implements ReleaseCandidateRepository.
func (r *InMemoryReleaseCandidateRepository) GetByVersion(_ context.Context, version string) (*ReleaseCandidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.candidates {
		if c.Version == version {
			cp := *c
			return &cp, nil
		}
	}
	return nil, ErrCandidateNotFound
}

// List implements ReleaseCandidateRepository.
func (r *InMemoryReleaseCandidateRepository) List(_ context.Context) ([]ReleaseCandidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ReleaseCandidate, 0, len(r.candidates))
	for _, c := range r.candidates {
		out = append(out, *c)
	}
	return out, nil
}

var _ ReleaseCandidateRepository = (*InMemoryReleaseCandidateRepository)(nil)

// InMemoryReleaseRepository is a process-local ReleaseRepository backed
// by a map guarded by a mutex, intended for tests and fixtures -- never
// for production use.
type InMemoryReleaseRepository struct {
	mu       sync.RWMutex
	releases map[uuid.UUID]*Release
}

// NewInMemoryReleaseRepository builds an empty
// InMemoryReleaseRepository.
func NewInMemoryReleaseRepository() *InMemoryReleaseRepository {
	return &InMemoryReleaseRepository{releases: make(map[uuid.UUID]*Release)}
}

// Create implements ReleaseRepository.
func (r *InMemoryReleaseRepository) Create(_ context.Context, rel *Release) error {
	if rel == nil {
		return ErrInvalidRelease
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.releases {
		if existing.CandidateID == rel.CandidateID && existing.ID != rel.ID {
			return ErrAlreadyReleased
		}
	}
	cp := *rel
	r.releases[rel.ID] = &cp
	return nil
}

// Get implements ReleaseRepository.
func (r *InMemoryReleaseRepository) Get(_ context.Context, id uuid.UUID) (*Release, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rel, ok := r.releases[id]
	if !ok {
		return nil, ErrReleaseNotFound
	}
	cp := *rel
	return &cp, nil
}

// GetByCandidateID implements ReleaseRepository.
func (r *InMemoryReleaseRepository) GetByCandidateID(_ context.Context, candidateID uuid.UUID) (*Release, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rel := range r.releases {
		if rel.CandidateID == candidateID {
			cp := *rel
			return &cp, nil
		}
	}
	return nil, ErrReleaseNotFound
}

// List implements ReleaseRepository.
func (r *InMemoryReleaseRepository) List(_ context.Context) ([]Release, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Release, 0, len(r.releases))
	for _, rel := range r.releases {
		out = append(out, *rel)
	}
	return out, nil
}

var _ ReleaseRepository = (*InMemoryReleaseRepository)(nil)
