package bulkimport

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// InMemoryJobRepository is a process-local JobRepository backed by a
// map guarded by a mutex, intended for tests and other packages'
// fixtures -- never for production use, mirroring
// packages/privacy.InMemoryInventoryRepository's role exactly.
type InMemoryJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*ImportJob
}

// NewInMemoryJobRepository builds an empty InMemoryJobRepository.
func NewInMemoryJobRepository() *InMemoryJobRepository {
	return &InMemoryJobRepository{jobs: make(map[uuid.UUID]*ImportJob)}
}

func (r *InMemoryJobRepository) Create(_ context.Context, tenantID uuid.UUID, j *ImportJob) error {
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
	cp := j.Clone()
	r.jobs[j.ID] = &cp
	return nil
}

func (r *InMemoryJobRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ImportJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok || j.TenantID != tenantID {
		return nil, ErrJobNotFound
	}
	cp := j.Clone()
	return &cp, nil
}

func (r *InMemoryJobRepository) List(_ context.Context, tenantID uuid.UUID) ([]ImportJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ImportJob, 0)
	for _, j := range r.jobs {
		if j.TenantID == tenantID {
			out = append(out, j.Clone())
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].CreatedAt.Before(out[k].CreatedAt) })
	return out, nil
}

func (r *InMemoryJobRepository) Update(_ context.Context, tenantID uuid.UUID, j *ImportJob) error {
	if j == nil {
		return ErrInvalidJob
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.jobs[j.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrJobNotFound
	}
	cp := j.Clone()
	cp.TenantID = tenantID
	r.jobs[j.ID] = &cp
	return nil
}

var _ JobRepository = (*InMemoryJobRepository)(nil)

// InMemoryRecordRepository is a process-local RecordRepository.
type InMemoryRecordRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*ImportRecord
}

// NewInMemoryRecordRepository builds an empty InMemoryRecordRepository.
func NewInMemoryRecordRepository() *InMemoryRecordRepository {
	return &InMemoryRecordRepository{records: make(map[uuid.UUID]*ImportRecord)}
}

func (r *InMemoryRecordRepository) Create(_ context.Context, tenantID uuid.UUID, rec *ImportRecord) error {
	if rec == nil {
		return ErrInvalidRecord
	}
	if rec.TenantID == uuid.Nil {
		rec.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, rec.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := rec.Clone()
	r.records[rec.ID] = &cp
	return nil
}

func (r *InMemoryRecordRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ImportRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.records[id]
	if !ok || rec.TenantID != tenantID {
		return nil, ErrRecordNotFound
	}
	cp := rec.Clone()
	return &cp, nil
}

func (r *InMemoryRecordRepository) ListForJob(_ context.Context, tenantID, jobID uuid.UUID) ([]ImportRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ImportRecord, 0)
	for _, rec := range r.records {
		if rec.TenantID == tenantID && rec.JobID == jobID {
			out = append(out, rec.Clone())
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].SourceIndex < out[k].SourceIndex })
	return out, nil
}

func (r *InMemoryRecordRepository) FindByDedupKey(_ context.Context, tenantID, jobID uuid.UUID, key string) (*ImportRecord, error) {
	if key == "" {
		return nil, ErrRecordNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rec := range r.records {
		if rec.TenantID == tenantID && rec.JobID == jobID && rec.DedupKey == key && rec.Outcome == OutcomeImported {
			cp := rec.Clone()
			return &cp, nil
		}
	}
	return nil, ErrRecordNotFound
}

func (r *InMemoryRecordRepository) Update(_ context.Context, tenantID uuid.UUID, rec *ImportRecord) error {
	if rec == nil {
		return ErrInvalidRecord
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.records[rec.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrRecordNotFound
	}
	cp := rec.Clone()
	cp.TenantID = tenantID
	r.records[rec.ID] = &cp
	return nil
}

var _ RecordRepository = (*InMemoryRecordRepository)(nil)
