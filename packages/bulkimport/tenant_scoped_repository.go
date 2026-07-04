package bulkimport

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedJobRepository composes packages/tenancy.WithTenantScope
// with PostgresJobRepository, exactly as
// packages/privacy.TenantScopedInventoryRepository does: every method
// opens its own RLS-scoped transaction, so Row-Level Security enforces
// tenant isolation at the database layer in addition to
// PostgresJobRepository's own application-level requireMatchingTenant
// guard.
type TenantScopedJobRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedJobRepository builds a TenantScopedJobRepository
// backed by pool.
func NewTenantScopedJobRepository(pool *pgxpool.Pool) *TenantScopedJobRepository {
	return &TenantScopedJobRepository{pool: pool}
}

func (r *TenantScopedJobRepository) Create(ctx context.Context, tenantID uuid.UUID, j *ImportJob) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresJobRepository(exec).Create(ctx, tenantID, j)
	})
}

func (r *TenantScopedJobRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportJob, error) {
	var out *ImportJob
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		j, err := NewPostgresJobRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = j
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedJobRepository) List(ctx context.Context, tenantID uuid.UUID) ([]ImportJob, error) {
	var out []ImportJob
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresJobRepository(exec).List(ctx, tenantID)
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

func (r *TenantScopedJobRepository) Update(ctx context.Context, tenantID uuid.UUID, j *ImportJob) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresJobRepository(exec).Update(ctx, tenantID, j)
	})
}

var _ JobRepository = (*TenantScopedJobRepository)(nil)

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

func (r *TenantScopedRecordRepository) Create(ctx context.Context, tenantID uuid.UUID, rec *ImportRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRecordRepository(exec).Create(ctx, tenantID, rec)
	})
}

func (r *TenantScopedRecordRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportRecord, error) {
	var out *ImportRecord
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

func (r *TenantScopedRecordRepository) ListForJob(ctx context.Context, tenantID, jobID uuid.UUID) ([]ImportRecord, error) {
	var out []ImportRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRecordRepository(exec).ListForJob(ctx, tenantID, jobID)
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

func (r *TenantScopedRecordRepository) FindByDedupKey(ctx context.Context, tenantID, jobID uuid.UUID, key string) (*ImportRecord, error) {
	var out *ImportRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rec, err := NewPostgresRecordRepository(exec).FindByDedupKey(ctx, tenantID, jobID, key)
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

func (r *TenantScopedRecordRepository) Update(ctx context.Context, tenantID uuid.UUID, rec *ImportRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRecordRepository(exec).Update(ctx, tenantID, rec)
	})
}

var _ RecordRepository = (*TenantScopedRecordRepository)(nil)
