package integration

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedConfigRepository composes
// packages/tenancy.WithTenantScope with PostgresConfigRepository,
// exactly as packages/compliance.TenantScopedControlRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresConfigRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedConfigRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedConfigRepository builds a TenantScopedConfigRepository
// backed by pool.
func NewTenantScopedConfigRepository(pool *pgxpool.Pool) *TenantScopedConfigRepository {
	return &TenantScopedConfigRepository{pool: pool}
}

func (r *TenantScopedConfigRepository) Create(ctx context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresConfigRepository(exec).Create(ctx, tenantID, cfg)
	})
}

func (r *TenantScopedConfigRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ConnectorConfig, error) {
	var out *ConnectorConfig
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		cfg, err := NewPostgresConfigRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = cfg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedConfigRepository) List(ctx context.Context, tenantID uuid.UUID) ([]ConnectorConfig, error) {
	var out []ConnectorConfig
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresConfigRepository(exec).List(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedConfigRepository) Update(ctx context.Context, tenantID uuid.UUID, cfg *ConnectorConfig) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresConfigRepository(exec).Update(ctx, tenantID, cfg)
	})
}

var _ ConfigRepository = (*TenantScopedConfigRepository)(nil)

// TenantScopedCredentialsRepository composes
// packages/tenancy.WithTenantScope with PostgresCredentialsRepository.
type TenantScopedCredentialsRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedCredentialsRepository builds a
// TenantScopedCredentialsRepository backed by pool.
func NewTenantScopedCredentialsRepository(pool *pgxpool.Pool) *TenantScopedCredentialsRepository {
	return &TenantScopedCredentialsRepository{pool: pool}
}

func (r *TenantScopedCredentialsRepository) Create(ctx context.Context, tenantID uuid.UUID, c *ConnectorCredentials) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresCredentialsRepository(exec).Create(ctx, tenantID, c)
	})
}

func (r *TenantScopedCredentialsRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ConnectorCredentials, error) {
	var out *ConnectorCredentials
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		c, err := NewPostgresCredentialsRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedCredentialsRepository) Update(ctx context.Context, tenantID uuid.UUID, c *ConnectorCredentials) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresCredentialsRepository(exec).Update(ctx, tenantID, c)
	})
}

var _ CredentialsRepository = (*TenantScopedCredentialsRepository)(nil)

// TenantScopedFieldMappingRepository composes
// packages/tenancy.WithTenantScope with PostgresFieldMappingRepository.
type TenantScopedFieldMappingRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedFieldMappingRepository builds a
// TenantScopedFieldMappingRepository backed by pool.
func NewTenantScopedFieldMappingRepository(pool *pgxpool.Pool) *TenantScopedFieldMappingRepository {
	return &TenantScopedFieldMappingRepository{pool: pool}
}

func (r *TenantScopedFieldMappingRepository) Create(ctx context.Context, tenantID uuid.UUID, m *FieldMapping) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFieldMappingRepository(exec).Create(ctx, tenantID, m)
	})
}

func (r *TenantScopedFieldMappingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*FieldMapping, error) {
	var out *FieldMapping
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		m, err := NewPostgresFieldMappingRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedFieldMappingRepository) List(ctx context.Context, tenantID uuid.UUID) ([]FieldMapping, error) {
	var out []FieldMapping
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFieldMappingRepository(exec).List(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedFieldMappingRepository) Update(ctx context.Context, tenantID uuid.UUID, m *FieldMapping) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFieldMappingRepository(exec).Update(ctx, tenantID, m)
	})
}

var _ FieldMappingRepository = (*TenantScopedFieldMappingRepository)(nil)

// TenantScopedImportRunRepository composes
// packages/tenancy.WithTenantScope with PostgresImportRunRepository.
type TenantScopedImportRunRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedImportRunRepository builds a
// TenantScopedImportRunRepository backed by pool.
func NewTenantScopedImportRunRepository(pool *pgxpool.Pool) *TenantScopedImportRunRepository {
	return &TenantScopedImportRunRepository{pool: pool}
}

func (r *TenantScopedImportRunRepository) Create(ctx context.Context, tenantID uuid.UUID, run *ImportRun) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresImportRunRepository(exec).Create(ctx, tenantID, run)
	})
}

func (r *TenantScopedImportRunRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportRun, error) {
	var out *ImportRun
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		run, err := NewPostgresImportRunRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = run
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedImportRunRepository) ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ImportRun, error) {
	var out []ImportRun
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresImportRunRepository(exec).ListForConnector(ctx, tenantID, connectorConfigID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedImportRunRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ImportRun, error) {
	var out []ImportRun
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresImportRunRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ ImportRunRepository = (*TenantScopedImportRunRepository)(nil)

// TenantScopedDeliveryRunRepository composes
// packages/tenancy.WithTenantScope with PostgresDeliveryRunRepository.
type TenantScopedDeliveryRunRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedDeliveryRunRepository builds a
// TenantScopedDeliveryRunRepository backed by pool.
func NewTenantScopedDeliveryRunRepository(pool *pgxpool.Pool) *TenantScopedDeliveryRunRepository {
	return &TenantScopedDeliveryRunRepository{pool: pool}
}

func (r *TenantScopedDeliveryRunRepository) Create(ctx context.Context, tenantID uuid.UUID, run *DeliveryRun) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresDeliveryRunRepository(exec).Create(ctx, tenantID, run)
	})
}

func (r *TenantScopedDeliveryRunRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*DeliveryRun, error) {
	var out *DeliveryRun
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		run, err := NewPostgresDeliveryRunRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = run
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedDeliveryRunRepository) ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]DeliveryRun, error) {
	var out []DeliveryRun
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresDeliveryRunRepository(exec).ListForConnector(ctx, tenantID, connectorConfigID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedDeliveryRunRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]DeliveryRun, error) {
	var out []DeliveryRun
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresDeliveryRunRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ DeliveryRunRepository = (*TenantScopedDeliveryRunRepository)(nil)

// TenantScopedReconciliationRepository composes
// packages/tenancy.WithTenantScope with
// PostgresReconciliationRepository.
type TenantScopedReconciliationRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedReconciliationRepository builds a
// TenantScopedReconciliationRepository backed by pool.
func NewTenantScopedReconciliationRepository(pool *pgxpool.Pool) *TenantScopedReconciliationRepository {
	return &TenantScopedReconciliationRepository{pool: pool}
}

func (r *TenantScopedReconciliationRepository) Create(ctx context.Context, tenantID uuid.UUID, res *ReconciliationResult) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresReconciliationRepository(exec).Create(ctx, tenantID, res)
	})
}

func (r *TenantScopedReconciliationRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ReconciliationResult, error) {
	var out *ReconciliationResult
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		res, err := NewPostgresReconciliationRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedReconciliationRepository) ListForConnector(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ReconciliationResult, error) {
	var out []ReconciliationResult
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresReconciliationRepository(exec).ListForConnector(ctx, tenantID, connectorConfigID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedReconciliationRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ReconciliationResult, error) {
	var out []ReconciliationResult
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresReconciliationRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ ReconciliationRepository = (*TenantScopedReconciliationRepository)(nil)
