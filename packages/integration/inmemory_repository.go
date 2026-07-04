package integration

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// InMemoryConfigRepository is a process-local ConfigRepository backed
// by a map guarded by a mutex, intended for tests and other packages'
// fixtures -- never for production use, mirroring
// packages/compliance.InMemoryControlRepository's role exactly.
type InMemoryConfigRepository struct {
	mu      sync.RWMutex
	configs map[uuid.UUID]*ConnectorConfig
}

// NewInMemoryConfigRepository builds an empty InMemoryConfigRepository.
func NewInMemoryConfigRepository() *InMemoryConfigRepository {
	return &InMemoryConfigRepository{configs: make(map[uuid.UUID]*ConnectorConfig)}
}

func (r *InMemoryConfigRepository) Create(_ context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error {
	if cfg == nil {
		return ErrInvalidConnectorConfig
	}
	if cfg.TenantID == uuid.Nil {
		cfg.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, cfg.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *cfg
	r.configs[cfg.ID] = &cp
	return nil
}

func (r *InMemoryConfigRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ConnectorConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.configs[id]
	if !ok || cfg.TenantID != tenantID {
		return nil, ErrConnectorNotFound
	}
	cp := *cfg
	return &cp, nil
}

func (r *InMemoryConfigRepository) List(_ context.Context, tenantID uuid.UUID) ([]ConnectorConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConnectorConfig, 0)
	for _, cfg := range r.configs {
		if cfg.TenantID == tenantID {
			out = append(out, *cfg)
		}
	}
	return out, nil
}

func (r *InMemoryConfigRepository) Update(_ context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error {
	if cfg == nil {
		return ErrInvalidConnectorConfig
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.configs[cfg.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrConnectorNotFound
	}
	cp := *cfg
	cp.TenantID = tenantID
	r.configs[cfg.ID] = &cp
	return nil
}

var _ ConfigRepository = (*InMemoryConfigRepository)(nil)

// InMemoryCredentialsRepository is a process-local
// CredentialsRepository.
type InMemoryCredentialsRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*ConnectorCredentials
}

// NewInMemoryCredentialsRepository builds an empty
// InMemoryCredentialsRepository.
func NewInMemoryCredentialsRepository() *InMemoryCredentialsRepository {
	return &InMemoryCredentialsRepository{items: make(map[uuid.UUID]*ConnectorCredentials)}
}

func (r *InMemoryCredentialsRepository) Create(_ context.Context, tenantID uuid.UUID, creds *ConnectorCredentials) error {
	if creds == nil {
		return ErrInvalidCredentials
	}
	if creds.TenantID == uuid.Nil {
		creds.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, creds.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *creds
	r.items[creds.ID] = &cp
	return nil
}

func (r *InMemoryCredentialsRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ConnectorCredentials, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.items[id]
	if !ok || c.TenantID != tenantID {
		return nil, ErrCredentialsNotFound
	}
	cp := *c
	return &cp, nil
}

func (r *InMemoryCredentialsRepository) Update(_ context.Context, tenantID uuid.UUID, creds *ConnectorCredentials) error {
	if creds == nil {
		return ErrInvalidCredentials
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.items[creds.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrCredentialsNotFound
	}
	cp := *creds
	cp.TenantID = tenantID
	r.items[creds.ID] = &cp
	return nil
}

var _ CredentialsRepository = (*InMemoryCredentialsRepository)(nil)

// InMemoryFieldMappingRepository is a process-local
// FieldMappingRepository.
type InMemoryFieldMappingRepository struct {
	mu       sync.RWMutex
	mappings map[uuid.UUID]*FieldMapping
}

// NewInMemoryFieldMappingRepository builds an empty
// InMemoryFieldMappingRepository.
func NewInMemoryFieldMappingRepository() *InMemoryFieldMappingRepository {
	return &InMemoryFieldMappingRepository{mappings: make(map[uuid.UUID]*FieldMapping)}
}

func (r *InMemoryFieldMappingRepository) Create(_ context.Context, tenantID uuid.UUID, m *FieldMapping) error {
	if m == nil {
		return ErrInvalidFieldMapping
	}
	if m.TenantID == uuid.Nil {
		m.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, m.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *m
	r.mappings[m.ID] = &cp
	return nil
}

func (r *InMemoryFieldMappingRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*FieldMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.mappings[id]
	if !ok || m.TenantID != tenantID {
		return nil, ErrMappingNotFound
	}
	cp := *m
	return &cp, nil
}

func (r *InMemoryFieldMappingRepository) List(_ context.Context, tenantID uuid.UUID) ([]FieldMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FieldMapping, 0)
	for _, m := range r.mappings {
		if m.TenantID == tenantID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (r *InMemoryFieldMappingRepository) Update(_ context.Context, tenantID uuid.UUID, m *FieldMapping) error {
	if m == nil {
		return ErrInvalidFieldMapping
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.mappings[m.ID]
	if !ok || existing.TenantID != tenantID {
		return ErrMappingNotFound
	}
	cp := *m
	cp.TenantID = tenantID
	r.mappings[m.ID] = &cp
	return nil
}

var _ FieldMappingRepository = (*InMemoryFieldMappingRepository)(nil)

// InMemoryImportRunRepository is a process-local ImportRunRepository.
type InMemoryImportRunRepository struct {
	mu   sync.RWMutex
	runs map[uuid.UUID]*ImportRun
}

// NewInMemoryImportRunRepository builds an empty
// InMemoryImportRunRepository.
func NewInMemoryImportRunRepository() *InMemoryImportRunRepository {
	return &InMemoryImportRunRepository{runs: make(map[uuid.UUID]*ImportRun)}
}

func (r *InMemoryImportRunRepository) Create(_ context.Context, tenantID uuid.UUID, run *ImportRun) error {
	if run == nil {
		return ErrInvalidImportRun
	}
	if run.TenantID == uuid.Nil {
		run.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, run.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *run
	r.runs[run.ID] = &cp
	return nil
}

func (r *InMemoryImportRunRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ImportRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok || run.TenantID != tenantID {
		return nil, ErrImportRunNotFound
	}
	cp := *run
	return &cp, nil
}

func (r *InMemoryImportRunRepository) ListForConnector(_ context.Context, tenantID, connectorConfigID uuid.UUID) ([]ImportRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ImportRun, 0)
	for _, run := range r.runs {
		if run.TenantID == tenantID && run.ConnectorConfigID == connectorConfigID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (r *InMemoryImportRunRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]ImportRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ImportRun, 0)
	for _, run := range r.runs {
		if run.TenantID == tenantID {
			out = append(out, *run)
		}
	}
	return out, nil
}

var _ ImportRunRepository = (*InMemoryImportRunRepository)(nil)

// InMemoryDeliveryRunRepository is a process-local
// DeliveryRunRepository.
type InMemoryDeliveryRunRepository struct {
	mu   sync.RWMutex
	runs map[uuid.UUID]*DeliveryRun
}

// NewInMemoryDeliveryRunRepository builds an empty
// InMemoryDeliveryRunRepository.
func NewInMemoryDeliveryRunRepository() *InMemoryDeliveryRunRepository {
	return &InMemoryDeliveryRunRepository{runs: make(map[uuid.UUID]*DeliveryRun)}
}

func (r *InMemoryDeliveryRunRepository) Create(_ context.Context, tenantID uuid.UUID, run *DeliveryRun) error {
	if run == nil {
		return ErrInvalidDeliveryRun
	}
	if run.TenantID == uuid.Nil {
		run.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, run.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *run
	r.runs[run.ID] = &cp
	return nil
}

func (r *InMemoryDeliveryRunRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*DeliveryRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok || run.TenantID != tenantID {
		return nil, ErrDeliveryRunNotFound
	}
	cp := *run
	return &cp, nil
}

func (r *InMemoryDeliveryRunRepository) ListForConnector(_ context.Context, tenantID, connectorConfigID uuid.UUID) ([]DeliveryRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DeliveryRun, 0)
	for _, run := range r.runs {
		if run.TenantID == tenantID && run.ConnectorConfigID == connectorConfigID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (r *InMemoryDeliveryRunRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]DeliveryRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DeliveryRun, 0)
	for _, run := range r.runs {
		if run.TenantID == tenantID {
			out = append(out, *run)
		}
	}
	return out, nil
}

var _ DeliveryRunRepository = (*InMemoryDeliveryRunRepository)(nil)

// InMemoryReconciliationRepository is a process-local
// ReconciliationRepository.
type InMemoryReconciliationRepository struct {
	mu      sync.RWMutex
	results map[uuid.UUID]*ReconciliationResult
}

// NewInMemoryReconciliationRepository builds an empty
// InMemoryReconciliationRepository.
func NewInMemoryReconciliationRepository() *InMemoryReconciliationRepository {
	return &InMemoryReconciliationRepository{results: make(map[uuid.UUID]*ReconciliationResult)}
}

func (r *InMemoryReconciliationRepository) Create(_ context.Context, tenantID uuid.UUID, result *ReconciliationResult) error {
	if result == nil {
		return ErrInvalidReconciliation
	}
	if result.TenantID == uuid.Nil {
		result.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, result.TenantID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *result
	r.results[result.ID] = &cp
	return nil
}

func (r *InMemoryReconciliationRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*ReconciliationResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.results[id]
	if !ok || result.TenantID != tenantID {
		return nil, ErrReconciliationNotFound
	}
	cp := *result
	return &cp, nil
}

func (r *InMemoryReconciliationRepository) ListForConnector(_ context.Context, tenantID, connectorConfigID uuid.UUID) ([]ReconciliationResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ReconciliationResult, 0)
	for _, result := range r.results {
		if result.TenantID == tenantID && result.ConnectorConfigID == connectorConfigID {
			out = append(out, *result)
		}
	}
	return out, nil
}

func (r *InMemoryReconciliationRepository) ListAll(_ context.Context, tenantID uuid.UUID) ([]ReconciliationResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ReconciliationResult, 0)
	for _, result := range r.results {
		if result.TenantID == tenantID {
			out = append(out, *result)
		}
	}
	return out, nil
}

var _ ReconciliationRepository = (*InMemoryReconciliationRepository)(nil)
