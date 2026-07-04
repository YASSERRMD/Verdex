package backupdr

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// policyKey is the composite key InMemoryPolicyRepository indexes by:
// one BackupPolicy per tenant/DataClass pair.
type policyKey struct {
	tenantID uuid.UUID
	class    DataClass
}

// InMemoryPolicyRepository is a process-local PolicyRepository backed
// by a map guarded by a mutex, intended for tests and fixtures --
// never for production use, mirroring
// packages/privacy.InMemoryConsentRepository's role exactly.
type InMemoryPolicyRepository struct {
	mu       sync.RWMutex
	policies map[policyKey]*BackupPolicy
}

// NewInMemoryPolicyRepository builds an empty InMemoryPolicyRepository.
func NewInMemoryPolicyRepository() *InMemoryPolicyRepository {
	return &InMemoryPolicyRepository{policies: make(map[policyKey]*BackupPolicy)}
}

// Set implements PolicyRepository.
func (r *InMemoryPolicyRepository) Set(_ context.Context, tenantID uuid.UUID, p *BackupPolicy) error {
	if p == nil {
		return ErrInvalidPolicy
	}
	if p.TenantID == uuid.Nil {
		p.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, p.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *p
	r.policies[policyKey{tenantID: tenantID, class: p.Class}] = &cp
	return nil
}

// Get implements PolicyRepository.
func (r *InMemoryPolicyRepository) Get(_ context.Context, tenantID uuid.UUID, class DataClass) (*BackupPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.policies[policyKey{tenantID: tenantID, class: class}]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	cp := *p
	return &cp, nil
}

// ListAll implements PolicyRepository.
func (r *InMemoryPolicyRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]BackupPolicy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BackupPolicy, 0)
	for k, p := range r.policies {
		if k.tenantID == tenantID {
			out = append(out, *p)
		}
	}
	return out, nil
}

var _ PolicyRepository = (*InMemoryPolicyRepository)(nil)

// InMemoryRecordRepository is a process-local RecordRepository backed
// by a map guarded by a mutex.
type InMemoryRecordRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*BackupRecord
}

// NewInMemoryRecordRepository builds an empty InMemoryRecordRepository.
func NewInMemoryRecordRepository() *InMemoryRecordRepository {
	return &InMemoryRecordRepository{records: make(map[uuid.UUID]*BackupRecord)}
}

// Create implements RecordRepository.
func (r *InMemoryRecordRepository) Create(_ context.Context, tenantID uuid.UUID, rec *BackupRecord) error {
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
	cp := *rec
	r.records[rec.ID] = &cp
	return nil
}

// Get implements RecordRepository.
func (r *InMemoryRecordRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*BackupRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.records[id]
	if !ok || rec.TenantID != tenantID {
		return nil, ErrRecordNotFound
	}
	cp := *rec
	return &cp, nil
}

// ListForClass implements RecordRepository.
func (r *InMemoryRecordRepository) ListForClass(_ context.Context, tenantID uuid.UUID, class DataClass) ([]BackupRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BackupRecord, 0)
	for _, rec := range r.records {
		if rec.TenantID == tenantID && rec.Class == class {
			out = append(out, *rec)
		}
	}
	return out, nil
}

// ListAll implements RecordRepository.
func (r *InMemoryRecordRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]BackupRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BackupRecord, 0)
	for _, rec := range r.records {
		if rec.TenantID == tenantID {
			out = append(out, *rec)
		}
	}
	return out, nil
}

var _ RecordRepository = (*InMemoryRecordRepository)(nil)

// InMemoryDrillRepository is a process-local DrillRepository backed by
// a map guarded by a mutex.
type InMemoryDrillRepository struct {
	mu     sync.RWMutex
	drills map[uuid.UUID]*RestoreDrill
}

// NewInMemoryDrillRepository builds an empty InMemoryDrillRepository.
func NewInMemoryDrillRepository() *InMemoryDrillRepository {
	return &InMemoryDrillRepository{drills: make(map[uuid.UUID]*RestoreDrill)}
}

// Create implements DrillRepository.
func (r *InMemoryDrillRepository) Create(_ context.Context, tenantID uuid.UUID, d *RestoreDrill) error {
	if d == nil {
		return ErrInvalidDrill
	}
	if d.TenantID == uuid.Nil {
		d.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.drills[d.ID] = &cp
	return nil
}

// Get implements DrillRepository.
func (r *InMemoryDrillRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*RestoreDrill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drills[id]
	if !ok || d.TenantID != tenantID {
		return nil, ErrDrillNotFound
	}
	cp := *d
	return &cp, nil
}

// ListForClass implements DrillRepository.
func (r *InMemoryDrillRepository) ListForClass(_ context.Context, tenantID uuid.UUID, class DataClass) ([]RestoreDrill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RestoreDrill, 0)
	for _, d := range r.drills {
		if d.TenantID == tenantID && d.Class == class {
			out = append(out, *d)
		}
	}
	return out, nil
}

// ListAll implements DrillRepository.
func (r *InMemoryDrillRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]RestoreDrill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RestoreDrill, 0)
	for _, d := range r.drills {
		if d.TenantID == tenantID {
			out = append(out, *d)
		}
	}
	return out, nil
}

var _ DrillRepository = (*InMemoryDrillRepository)(nil)

// targetKey is the composite key InMemoryTargetRepository indexes by:
// one Target per tenant/DataClass pair, mirroring policyKey exactly.
type targetKey struct {
	tenantID uuid.UUID
	class    DataClass
}

// InMemoryTargetRepository is a process-local TargetRepository backed
// by a map guarded by a mutex.
type InMemoryTargetRepository struct {
	mu      sync.RWMutex
	targets map[targetKey]*Target
}

// NewInMemoryTargetRepository builds an empty InMemoryTargetRepository.
func NewInMemoryTargetRepository() *InMemoryTargetRepository {
	return &InMemoryTargetRepository{targets: make(map[targetKey]*Target)}
}

// Set implements TargetRepository.
func (r *InMemoryTargetRepository) Set(_ context.Context, tenantID uuid.UUID, t *Target) error {
	if t == nil {
		return ErrInvalidTarget
	}
	if t.TenantID == uuid.Nil {
		t.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, t.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *t
	r.targets[targetKey{tenantID: tenantID, class: t.Class}] = &cp
	return nil
}

// Get implements TargetRepository.
func (r *InMemoryTargetRepository) Get(_ context.Context, tenantID uuid.UUID, class DataClass) (*Target, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.targets[targetKey{tenantID: tenantID, class: class}]
	if !ok {
		return nil, ErrTargetNotFound
	}
	cp := *t
	return &cp, nil
}

// ListAll implements TargetRepository.
func (r *InMemoryTargetRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]Target, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Target, 0)
	for k, t := range r.targets {
		if k.tenantID == tenantID {
			out = append(out, *t)
		}
	}
	return out, nil
}

var _ TargetRepository = (*InMemoryTargetRepository)(nil)
