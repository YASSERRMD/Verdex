package pilot

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryDeploymentRepository is a process-local DeploymentRepository
// backed by a map guarded by a mutex, intended for tests and other
// packages' fixtures -- never for production use, mirroring
// packages/compliance.InMemoryEvidenceRepository's role exactly.
type InMemoryDeploymentRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*PilotDeployment
}

// NewInMemoryDeploymentRepository builds an empty
// InMemoryDeploymentRepository.
func NewInMemoryDeploymentRepository() *InMemoryDeploymentRepository {
	return &InMemoryDeploymentRepository{items: make(map[uuid.UUID]*PilotDeployment)}
}

// Create implements DeploymentRepository.
func (r *InMemoryDeploymentRepository) Create(_ context.Context, tenantID uuid.UUID, d *PilotDeployment) error {
	if d == nil {
		return ErrInvalidDeployment
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
	r.items[d.ID] = &cp
	return nil
}

// Get implements DeploymentRepository.
func (r *InMemoryDeploymentRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*PilotDeployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.items[id]
	if !ok || d.TenantID != tenantID {
		return nil, ErrDeploymentNotFound
	}
	cp := *d
	return &cp, nil
}

// ListAll implements DeploymentRepository.
func (r *InMemoryDeploymentRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]PilotDeployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PilotDeployment, 0)
	for _, d := range r.items {
		if d.TenantID == tenantID {
			out = append(out, *d)
		}
	}
	return out, nil
}

// Update implements DeploymentRepository.
func (r *InMemoryDeploymentRepository) Update(_ context.Context, tenantID uuid.UUID, d *PilotDeployment) error {
	if d == nil {
		return ErrInvalidDeployment
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.items[d.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrDeploymentNotFound
	}
	cp := *d
	cp.TenantID = tenantID
	r.items[d.ID] = &cp
	return nil
}

var _ DeploymentRepository = (*InMemoryDeploymentRepository)(nil)

// InMemoryCaseRepository is a process-local CaseRepository backed by a
// map guarded by a mutex.
type InMemoryCaseRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*PilotCase
}

// NewInMemoryCaseRepository builds an empty InMemoryCaseRepository.
func NewInMemoryCaseRepository() *InMemoryCaseRepository {
	return &InMemoryCaseRepository{items: make(map[uuid.UUID]*PilotCase)}
}

// Create implements CaseRepository.
func (r *InMemoryCaseRepository) Create(_ context.Context, tenantID uuid.UUID, c *PilotCase) error {
	if c == nil {
		return ErrInvalidCase
	}
	if c.TenantID == uuid.Nil {
		c.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, c.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.items[c.ID] = &cp
	return nil
}

// Get implements CaseRepository.
func (r *InMemoryCaseRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*PilotCase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.items[id]
	if !ok || c.TenantID != tenantID {
		return nil, ErrCaseNotFound
	}
	cp := *c
	return &cp, nil
}

// ListForDeployment implements CaseRepository.
func (r *InMemoryCaseRepository) ListForDeployment(_ context.Context, tenantID, deploymentID uuid.UUID) ([]PilotCase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PilotCase, 0)
	for _, c := range r.items {
		if c.TenantID == tenantID && c.DeploymentID == deploymentID {
			out = append(out, *c)
		}
	}
	return out, nil
}

// Update implements CaseRepository.
func (r *InMemoryCaseRepository) Update(_ context.Context, tenantID uuid.UUID, c *PilotCase) error {
	if c == nil {
		return ErrInvalidCase
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.items[c.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrCaseNotFound
	}
	cp := *c
	cp.TenantID = tenantID
	r.items[c.ID] = &cp
	return nil
}

var _ CaseRepository = (*InMemoryCaseRepository)(nil)

// InMemoryFeedbackRepository is a process-local FeedbackRepository
// backed by a map guarded by a mutex.
type InMemoryFeedbackRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*FeedbackEntry
}

// NewInMemoryFeedbackRepository builds an empty
// InMemoryFeedbackRepository.
func NewInMemoryFeedbackRepository() *InMemoryFeedbackRepository {
	return &InMemoryFeedbackRepository{items: make(map[uuid.UUID]*FeedbackEntry)}
}

// Create implements FeedbackRepository.
func (r *InMemoryFeedbackRepository) Create(_ context.Context, tenantID uuid.UUID, f *FeedbackEntry) error {
	if f == nil {
		return ErrInvalidFeedback
	}
	if f.TenantID == uuid.Nil {
		f.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.items[f.ID] = &cp
	return nil
}

// Get implements FeedbackRepository.
func (r *InMemoryFeedbackRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*FeedbackEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.items[id]
	if !ok || f.TenantID != tenantID {
		return nil, ErrFeedbackNotFound
	}
	cp := *f
	return &cp, nil
}

// ListForCase implements FeedbackRepository.
func (r *InMemoryFeedbackRepository) ListForCase(_ context.Context, tenantID, pilotCaseID uuid.UUID) ([]FeedbackEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FeedbackEntry, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID && f.PilotCaseID == pilotCaseID {
			out = append(out, *f)
		}
	}
	return out, nil
}

// ListForDeployment implements FeedbackRepository, returning every
// FeedbackEntry whose PilotCaseID is in pilotCaseIDs.
func (r *InMemoryFeedbackRepository) ListForDeployment(_ context.Context, tenantID uuid.UUID, pilotCaseIDs []uuid.UUID) ([]FeedbackEntry, error) {
	wanted := make(map[uuid.UUID]bool, len(pilotCaseIDs))
	for _, id := range pilotCaseIDs {
		wanted[id] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FeedbackEntry, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID && wanted[f.PilotCaseID] {
			out = append(out, *f)
		}
	}
	return out, nil
}

// ListAll implements FeedbackRepository.
func (r *InMemoryFeedbackRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]FeedbackEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FeedbackEntry, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID {
			out = append(out, *f)
		}
	}
	return out, nil
}

var _ FeedbackRepository = (*InMemoryFeedbackRepository)(nil)

// InMemoryFindingRepository is a process-local FindingRepository
// backed by a map guarded by a mutex.
type InMemoryFindingRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*PilotFinding
}

// NewInMemoryFindingRepository builds an empty
// InMemoryFindingRepository.
func NewInMemoryFindingRepository() *InMemoryFindingRepository {
	return &InMemoryFindingRepository{items: make(map[uuid.UUID]*PilotFinding)}
}

// Create implements FindingRepository.
func (r *InMemoryFindingRepository) Create(_ context.Context, tenantID uuid.UUID, f *PilotFinding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	if f.TenantID == uuid.Nil {
		f.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, f.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *f
	r.items[f.ID] = &cp
	return nil
}

// Get implements FindingRepository.
func (r *InMemoryFindingRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*PilotFinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.items[id]
	if !ok || f.TenantID != tenantID {
		return nil, ErrFindingNotFound
	}
	cp := *f
	return &cp, nil
}

// ListForDeployment implements FindingRepository.
func (r *InMemoryFindingRepository) ListForDeployment(_ context.Context, tenantID, deploymentID uuid.UUID) ([]PilotFinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PilotFinding, 0)
	for _, f := range r.items {
		if f.TenantID == tenantID && f.DeploymentID == deploymentID {
			out = append(out, *f)
		}
	}
	return out, nil
}

// Update implements FindingRepository.
func (r *InMemoryFindingRepository) Update(_ context.Context, tenantID uuid.UUID, f *PilotFinding) error {
	if f == nil {
		return ErrInvalidFinding
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.items[f.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrFindingNotFound
	}
	cp := *f
	cp.TenantID = tenantID
	r.items[f.ID] = &cp
	return nil
}

var _ FindingRepository = (*InMemoryFindingRepository)(nil)

// InMemoryRefinementRepository is a process-local RefinementRepository
// backed by a map guarded by a mutex.
type InMemoryRefinementRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*RefinementRecord
}

// NewInMemoryRefinementRepository builds an empty
// InMemoryRefinementRepository.
func NewInMemoryRefinementRepository() *InMemoryRefinementRepository {
	return &InMemoryRefinementRepository{items: make(map[uuid.UUID]*RefinementRecord)}
}

// Create implements RefinementRepository.
func (r *InMemoryRefinementRepository) Create(_ context.Context, tenantID uuid.UUID, rec *RefinementRecord) error {
	if rec == nil {
		return ErrInvalidRefinement
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
	r.items[rec.ID] = &cp
	return nil
}

// Get implements RefinementRepository.
func (r *InMemoryRefinementRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*RefinementRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.items[id]
	if !ok || rec.TenantID != tenantID {
		return nil, ErrRefinementNotFound
	}
	cp := *rec
	return &cp, nil
}

// ListForFinding implements RefinementRepository.
func (r *InMemoryRefinementRepository) ListForFinding(_ context.Context, tenantID, findingID uuid.UUID) ([]RefinementRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RefinementRecord, 0)
	for _, rec := range r.items {
		if rec.TenantID == tenantID && rec.FindingID == findingID {
			out = append(out, *rec)
		}
	}
	return out, nil
}

// ListForDeployment implements RefinementRepository, returning every
// RefinementRecord whose FindingID is in findingIDs.
func (r *InMemoryRefinementRepository) ListForDeployment(_ context.Context, tenantID uuid.UUID, findingIDs []uuid.UUID) ([]RefinementRecord, error) {
	wanted := make(map[uuid.UUID]bool, len(findingIDs))
	for _, id := range findingIDs {
		wanted[id] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RefinementRecord, 0)
	for _, rec := range r.items {
		if rec.TenantID == tenantID && wanted[rec.FindingID] {
			out = append(out, *rec)
		}
	}
	return out, nil
}

// Update implements RefinementRepository.
func (r *InMemoryRefinementRepository) Update(_ context.Context, tenantID uuid.UUID, rec *RefinementRecord) error {
	if rec == nil {
		return ErrInvalidRefinement
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.items[rec.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrRefinementNotFound
	}
	cp := *rec
	cp.TenantID = tenantID
	r.items[rec.ID] = &cp
	return nil
}

var _ RefinementRepository = (*InMemoryRefinementRepository)(nil)
