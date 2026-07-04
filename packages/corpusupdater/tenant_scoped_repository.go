package corpusupdater

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

func (r *TenantScopedJobRepository) Create(ctx context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresJobRepository(exec).Create(ctx, tenantID, j)
	})
}

func (r *TenantScopedJobRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*CorpusUpdateJob, error) {
	var out *CorpusUpdateJob
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

func (r *TenantScopedJobRepository) ListByJurisdiction(ctx context.Context, tenantID uuid.UUID, jurisdictionCode string) ([]CorpusUpdateJob, error) {
	var out []CorpusUpdateJob
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresJobRepository(exec).ListByJurisdiction(ctx, tenantID, jurisdictionCode)
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

func (r *TenantScopedJobRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]CorpusUpdateJob, error) {
	var out []CorpusUpdateJob
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresJobRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedJobRepository) Update(ctx context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresJobRepository(exec).Update(ctx, tenantID, j)
	})
}

var _ JobRepository = (*TenantScopedJobRepository)(nil)

// TenantScopedAmendmentRepository composes
// packages/tenancy.WithTenantScope with PostgresAmendmentRepository.
type TenantScopedAmendmentRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedAmendmentRepository builds a
// TenantScopedAmendmentRepository backed by pool.
func NewTenantScopedAmendmentRepository(pool *pgxpool.Pool) *TenantScopedAmendmentRepository {
	return &TenantScopedAmendmentRepository{pool: pool}
}

func (r *TenantScopedAmendmentRepository) Create(ctx context.Context, tenantID uuid.UUID, a *Amendment) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAmendmentRepository(exec).Create(ctx, tenantID, a)
	})
}

func (r *TenantScopedAmendmentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Amendment, error) {
	var out *Amendment
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		a, err := NewPostgresAmendmentRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedAmendmentRepository) ListForJob(ctx context.Context, tenantID, jobID uuid.UUID) ([]Amendment, error) {
	var out []Amendment
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresAmendmentRepository(exec).ListForJob(ctx, tenantID, jobID)
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

func (r *TenantScopedAmendmentRepository) ListForTarget(ctx context.Context, tenantID uuid.UUID, corpus CorpusTarget, targetID string) ([]Amendment, error) {
	var out []Amendment
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresAmendmentRepository(exec).ListForTarget(ctx, tenantID, corpus, targetID)
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

func (r *TenantScopedAmendmentRepository) Update(ctx context.Context, tenantID uuid.UUID, a *Amendment) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresAmendmentRepository(exec).Update(ctx, tenantID, a)
	})
}

var _ AmendmentRepository = (*TenantScopedAmendmentRepository)(nil)
