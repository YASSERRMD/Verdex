package privacy

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryInventoryRepository is a process-local InventoryRepository
// backed by a map guarded by a mutex, intended for tests and other
// packages' fixtures -- never for production use, mirroring
// packages/accessgovernance.InMemoryPolicyRepository's role exactly.
type InMemoryInventoryRepository struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]*DataInventoryEntry
}

// NewInMemoryInventoryRepository builds an empty
// InMemoryInventoryRepository.
func NewInMemoryInventoryRepository() *InMemoryInventoryRepository {
	return &InMemoryInventoryRepository{entries: make(map[uuid.UUID]*DataInventoryEntry)}
}

func (r *InMemoryInventoryRepository) Create(_ context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error {
	if e == nil {
		return ErrInvalidInventoryEntry
	}
	if e.TenantID == uuid.Nil {
		e.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, e.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *e
	r.entries[e.ID] = &cp
	return nil
}

func (r *InMemoryInventoryRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*DataInventoryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrInventoryEntryNotFound
	}
	cp := *e
	return &cp, nil
}

func (r *InMemoryInventoryRepository) List(_ context.Context, tenantID uuid.UUID) ([]DataInventoryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DataInventoryEntry, 0)
	for _, e := range r.entries {
		if e.TenantID == tenantID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (r *InMemoryInventoryRepository) Update(_ context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error {
	if e == nil {
		return ErrInvalidInventoryEntry
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.entries[e.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrInventoryEntryNotFound
	}
	cp := *e
	cp.TenantID = tenantID
	r.entries[e.ID] = &cp
	return nil
}

var _ InventoryRepository = (*InMemoryInventoryRepository)(nil)

// InMemoryConsentRepository is a process-local ConsentRepository.
type InMemoryConsentRepository struct {
	mu      sync.RWMutex
	records map[uuid.UUID]*ConsentRecord
}

// NewInMemoryConsentRepository builds an empty
// InMemoryConsentRepository.
func NewInMemoryConsentRepository() *InMemoryConsentRepository {
	return &InMemoryConsentRepository{records: make(map[uuid.UUID]*ConsentRecord)}
}

func (r *InMemoryConsentRepository) Create(_ context.Context, tenantID uuid.UUID, c *ConsentRecord) error {
	if c == nil {
		return ErrInvalidConsentRecord
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
	r.records[c.ID] = &cp
	return nil
}

func (r *InMemoryConsentRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ConsentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.records[id]
	if !ok || c.TenantID != tenantID {
		return nil, ErrConsentNotFound
	}
	cp := *c
	return &cp, nil
}

func (r *InMemoryConsentRepository) ListForSubject(_ context.Context, tenantID uuid.UUID, subjectID string) ([]ConsentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConsentRecord, 0)
	for _, c := range r.records {
		if c.TenantID == tenantID && c.SubjectID == subjectID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *InMemoryConsentRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]ConsentRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConsentRecord, 0)
	for _, c := range r.records {
		if c.TenantID == tenantID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *InMemoryConsentRepository) Update(_ context.Context, tenantID uuid.UUID, c *ConsentRecord) error {
	if c == nil {
		return ErrInvalidConsentRecord
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.records[c.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrConsentNotFound
	}
	cp := *c
	cp.TenantID = tenantID
	r.records[c.ID] = &cp
	return nil
}

var _ ConsentRepository = (*InMemoryConsentRepository)(nil)

// InMemorySARRepository is a process-local SARRepository.
type InMemorySARRepository struct {
	mu   sync.RWMutex
	sars map[uuid.UUID]*SubjectAccessRequest
}

// NewInMemorySARRepository builds an empty InMemorySARRepository.
func NewInMemorySARRepository() *InMemorySARRepository {
	return &InMemorySARRepository{sars: make(map[uuid.UUID]*SubjectAccessRequest)}
}

func (r *InMemorySARRepository) Create(_ context.Context, tenantID uuid.UUID, s *SubjectAccessRequest) error {
	if s == nil {
		return ErrInvalidSAR
	}
	if s.TenantID == uuid.Nil {
		s.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, s.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.sars[s.ID] = &cp
	return nil
}

func (r *InMemorySARRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*SubjectAccessRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sars[id]
	if !ok || s.TenantID != tenantID {
		return nil, ErrSARNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *InMemorySARRepository) ListForSubject(_ context.Context, tenantID uuid.UUID, subjectID string) ([]SubjectAccessRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SubjectAccessRequest, 0)
	for _, s := range r.sars {
		if s.TenantID == tenantID && s.SubjectID == subjectID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *InMemorySARRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]SubjectAccessRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SubjectAccessRequest, 0)
	for _, s := range r.sars {
		if s.TenantID == tenantID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *InMemorySARRepository) Update(_ context.Context, tenantID uuid.UUID, s *SubjectAccessRequest) error {
	if s == nil {
		return ErrInvalidSAR
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.sars[s.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrSARNotFound
	}
	cp := *s
	cp.TenantID = tenantID
	r.sars[s.ID] = &cp
	return nil
}

var _ SARRepository = (*InMemorySARRepository)(nil)

// InMemoryErasureRepository is a process-local ErasureRepository.
type InMemoryErasureRepository struct {
	mu       sync.RWMutex
	requests map[uuid.UUID]*ErasureRequest
}

// NewInMemoryErasureRepository builds an empty
// InMemoryErasureRepository.
func NewInMemoryErasureRepository() *InMemoryErasureRepository {
	return &InMemoryErasureRepository{requests: make(map[uuid.UUID]*ErasureRequest)}
}

func (r *InMemoryErasureRepository) Create(_ context.Context, tenantID uuid.UUID, req *ErasureRequest) error {
	if req == nil {
		return ErrInvalidErasureRequest
	}
	if req.TenantID == uuid.Nil {
		req.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, req.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *req
	r.requests[req.ID] = &cp
	return nil
}

func (r *InMemoryErasureRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ErasureRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	req, ok := r.requests[id]
	if !ok || req.TenantID != tenantID {
		return nil, ErrErasureNotFound
	}
	cp := *req
	return &cp, nil
}

func (r *InMemoryErasureRepository) ListForSubject(_ context.Context, tenantID uuid.UUID, subjectID string) ([]ErasureRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ErasureRequest, 0)
	for _, req := range r.requests {
		if req.TenantID == tenantID && req.SubjectID == subjectID {
			out = append(out, *req)
		}
	}
	return out, nil
}

func (r *InMemoryErasureRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]ErasureRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ErasureRequest, 0)
	for _, req := range r.requests {
		if req.TenantID == tenantID {
			out = append(out, *req)
		}
	}
	return out, nil
}

func (r *InMemoryErasureRepository) Update(_ context.Context, tenantID uuid.UUID, req *ErasureRequest) error {
	if req == nil {
		return ErrInvalidErasureRequest
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.requests[req.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrErasureNotFound
	}
	cp := *req
	cp.TenantID = tenantID
	r.requests[req.ID] = &cp
	return nil
}

var _ ErasureRepository = (*InMemoryErasureRepository)(nil)
