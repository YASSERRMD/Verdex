package corpusupdater

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryJobRepository is a process-local JobRepository backed by a
// map guarded by a mutex, intended for tests and other packages'
// fixtures -- never for production use, mirroring
// packages/compliance.InMemoryControlRepository's role exactly.
type InMemoryJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*CorpusUpdateJob
}

// NewInMemoryJobRepository builds an empty InMemoryJobRepository.
func NewInMemoryJobRepository() *InMemoryJobRepository {
	return &InMemoryJobRepository{jobs: make(map[uuid.UUID]*CorpusUpdateJob)}
}

func (r *InMemoryJobRepository) Create(_ context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error {
	if j == nil {
		return ErrInvalidJob
	}
	if j.TenantID == uuid.Nil {
		j.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, j.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *j
	r.jobs[j.ID] = &cp
	return nil
}

func (r *InMemoryJobRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*CorpusUpdateJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, ErrJobNotFound
	}
	cp := *j
	return &cp, nil
}

func (r *InMemoryJobRepository) ListByJurisdiction(_ context.Context, tenantID uuid.UUID, jurisdictionCode string) ([]CorpusUpdateJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CorpusUpdateJob, 0)
	for _, j := range r.jobs {
		if j.TenantID == tenantID && j.JurisdictionCode == jurisdictionCode {
			out = append(out, *j)
		}
	}
	return out, nil
}

func (r *InMemoryJobRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]CorpusUpdateJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CorpusUpdateJob, 0)
	for _, j := range r.jobs {
		if j.TenantID == tenantID {
			out = append(out, *j)
		}
	}
	return out, nil
}

func (r *InMemoryJobRepository) Update(_ context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error {
	if j == nil {
		return ErrInvalidJob
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.jobs[j.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrJobNotFound
	}
	cp := *j
	cp.TenantID = tenantID
	r.jobs[j.ID] = &cp
	return nil
}

var _ JobRepository = (*InMemoryJobRepository)(nil)

// InMemoryAmendmentRepository is a process-local AmendmentRepository.
type InMemoryAmendmentRepository struct {
	mu         sync.RWMutex
	amendments map[uuid.UUID]*Amendment
}

// NewInMemoryAmendmentRepository builds an empty
// InMemoryAmendmentRepository.
func NewInMemoryAmendmentRepository() *InMemoryAmendmentRepository {
	return &InMemoryAmendmentRepository{amendments: make(map[uuid.UUID]*Amendment)}
}

func (r *InMemoryAmendmentRepository) Create(_ context.Context, tenantID uuid.UUID, a *Amendment) error {
	if a == nil {
		return ErrInvalidAmendment
	}
	if a.TenantID == uuid.Nil {
		a.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, a.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *a
	r.amendments[a.ID] = &cp
	return nil
}

func (r *InMemoryAmendmentRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Amendment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.amendments[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrAmendmentNotFound
	}
	cp := *a
	return &cp, nil
}

func (r *InMemoryAmendmentRepository) ListForJob(_ context.Context, tenantID, jobID uuid.UUID) ([]Amendment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Amendment, 0)
	for _, a := range r.amendments {
		if a.TenantID == tenantID && a.JobID == jobID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (r *InMemoryAmendmentRepository) ListForTarget(_ context.Context, tenantID uuid.UUID, corpus CorpusTarget, targetID string) ([]Amendment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Amendment, 0)
	for _, a := range r.amendments {
		if a.TenantID == tenantID && a.TargetCorpus == corpus && a.TargetID == targetID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (r *InMemoryAmendmentRepository) Update(_ context.Context, tenantID uuid.UUID, a *Amendment) error {
	if a == nil {
		return ErrInvalidAmendment
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.amendments[a.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrAmendmentNotFound
	}
	cp := *a
	cp.TenantID = tenantID
	r.amendments[a.ID] = &cp
	return nil
}

var _ AmendmentRepository = (*InMemoryAmendmentRepository)(nil)
