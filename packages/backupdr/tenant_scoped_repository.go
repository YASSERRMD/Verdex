package backupdr

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedPolicyRepository composes
// packages/tenancy.WithTenantScope with PostgresPolicyRepository,
// exactly as packages/privacy.TenantScopedInventoryRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresPolicyRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedPolicyRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedPolicyRepository builds a TenantScopedPolicyRepository
// backed by pool.
func NewTenantScopedPolicyRepository(pool *pgxpool.Pool) *TenantScopedPolicyRepository {
	return &TenantScopedPolicyRepository{pool: pool}
}

func (r *TenantScopedPolicyRepository) Set(ctx context.Context, tenantID uuid.UUID, p *BackupPolicy) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresPolicyRepository(exec).Set(ctx, tenantID, p)
	})
}

func (r *TenantScopedPolicyRepository) Get(ctx context.Context, tenantID uuid.UUID, class DataClass) (*BackupPolicy, error) {
	var out *BackupPolicy
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		p, err := NewPostgresPolicyRepository(exec).Get(ctx, tenantID, class)
		if err != nil {
			return err
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedPolicyRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]BackupPolicy, error) {
	var out []BackupPolicy
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresPolicyRepository(exec).ListAll(ctx, tenantID)
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

var _ PolicyRepository = (*TenantScopedPolicyRepository)(nil)

// TenantScopedRecordRepository composes packages/tenancy.WithTenantScope
// with PostgresRecordRepository.
type TenantScopedRecordRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedRecordRepository builds a TenantScopedRecordRepository
// backed by pool.
func NewTenantScopedRecordRepository(pool *pgxpool.Pool) *TenantScopedRecordRepository {
	return &TenantScopedRecordRepository{pool: pool}
}

func (r *TenantScopedRecordRepository) Create(ctx context.Context, tenantID uuid.UUID, rec *BackupRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRecordRepository(exec).Create(ctx, tenantID, rec)
	})
}

func (r *TenantScopedRecordRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*BackupRecord, error) {
	var out *BackupRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rec, err := NewPostgresRecordRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = rec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedRecordRepository) ListForClass(ctx context.Context, tenantID uuid.UUID, class DataClass) ([]BackupRecord, error) {
	var out []BackupRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRecordRepository(exec).ListForClass(ctx, tenantID, class)
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

func (r *TenantScopedRecordRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]BackupRecord, error) {
	var out []BackupRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRecordRepository(exec).ListAll(ctx, tenantID)
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

var _ RecordRepository = (*TenantScopedRecordRepository)(nil)

// TenantScopedDrillRepository composes packages/tenancy.WithTenantScope
// with PostgresDrillRepository.
type TenantScopedDrillRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedDrillRepository builds a TenantScopedDrillRepository
// backed by pool.
func NewTenantScopedDrillRepository(pool *pgxpool.Pool) *TenantScopedDrillRepository {
	return &TenantScopedDrillRepository{pool: pool}
}

func (r *TenantScopedDrillRepository) Create(ctx context.Context, tenantID uuid.UUID, d *RestoreDrill) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresDrillRepository(exec).Create(ctx, tenantID, d)
	})
}

func (r *TenantScopedDrillRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*RestoreDrill, error) {
	var out *RestoreDrill
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		d, err := NewPostgresDrillRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedDrillRepository) ListForClass(ctx context.Context, tenantID uuid.UUID, class DataClass) ([]RestoreDrill, error) {
	var out []RestoreDrill
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresDrillRepository(exec).ListForClass(ctx, tenantID, class)
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

func (r *TenantScopedDrillRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]RestoreDrill, error) {
	var out []RestoreDrill
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresDrillRepository(exec).ListAll(ctx, tenantID)
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

var _ DrillRepository = (*TenantScopedDrillRepository)(nil)

// TenantScopedTargetRepository composes packages/tenancy.WithTenantScope
// with PostgresTargetRepository.
type TenantScopedTargetRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedTargetRepository builds a TenantScopedTargetRepository
// backed by pool.
func NewTenantScopedTargetRepository(pool *pgxpool.Pool) *TenantScopedTargetRepository {
	return &TenantScopedTargetRepository{pool: pool}
}

func (r *TenantScopedTargetRepository) Set(ctx context.Context, tenantID uuid.UUID, t *Target) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresTargetRepository(exec).Set(ctx, tenantID, t)
	})
}

func (r *TenantScopedTargetRepository) Get(ctx context.Context, tenantID uuid.UUID, class DataClass) (*Target, error) {
	var out *Target
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		t, err := NewPostgresTargetRepository(exec).Get(ctx, tenantID, class)
		if err != nil {
			return err
		}
		out = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedTargetRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Target, error) {
	var out []Target
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresTargetRepository(exec).ListAll(ctx, tenantID)
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

var _ TargetRepository = (*TenantScopedTargetRepository)(nil)
