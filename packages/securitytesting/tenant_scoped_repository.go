package securitytesting

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRunRecordRepository composes
// packages/tenancy.WithTenantScope with PostgresRunRecordRepository,
// exactly as packages/compliance.TenantScopedEvidenceRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresRunRecordRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedRunRecordRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedRunRecordRepository builds a
// TenantScopedRunRecordRepository backed by pool.
func NewTenantScopedRunRecordRepository(pool *pgxpool.Pool) *TenantScopedRunRecordRepository {
	return &TenantScopedRunRecordRepository{pool: pool}
}

func (r *TenantScopedRunRecordRepository) Create(ctx context.Context, tenantID uuid.UUID, rr *RunRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRunRecordRepository(exec).Create(ctx, tenantID, rr)
	})
}

func (r *TenantScopedRunRecordRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*RunRecord, error) {
	var out *RunRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rr, err := NewPostgresRunRecordRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = rr
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedRunRecordRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]RunRecord, error) {
	var out []RunRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRunRecordRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedRunRecordRepository) ListForScenario(ctx context.Context, tenantID uuid.UUID, scenarioName string) ([]RunRecord, error) {
	var out []RunRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRunRecordRepository(exec).ListForScenario(ctx, tenantID, scenarioName)
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

var _ RunRecordRepository = (*TenantScopedRunRecordRepository)(nil)

// TenantScopedFindingRepository composes packages/tenancy.WithTenantScope
// with PostgresFindingRepository.
type TenantScopedFindingRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedFindingRepository builds a
// TenantScopedFindingRepository backed by pool.
func NewTenantScopedFindingRepository(pool *pgxpool.Pool) *TenantScopedFindingRepository {
	return &TenantScopedFindingRepository{pool: pool}
}

func (r *TenantScopedFindingRepository) Create(ctx context.Context, tenantID uuid.UUID, f *Finding) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFindingRepository(exec).Create(ctx, tenantID, f)
	})
}

func (r *TenantScopedFindingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Finding, error) {
	var out *Finding
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		f, err := NewPostgresFindingRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = f
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedFindingRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Finding, error) {
	var out []Finding
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFindingRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedFindingRepository) ListByStatus(ctx context.Context, tenantID uuid.UUID, status FindingStatus) ([]Finding, error) {
	var out []Finding
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFindingRepository(exec).ListByStatus(ctx, tenantID, status)
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

func (r *TenantScopedFindingRepository) Update(ctx context.Context, tenantID uuid.UUID, f *Finding) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFindingRepository(exec).Update(ctx, tenantID, f)
	})
}

var _ FindingRepository = (*TenantScopedFindingRepository)(nil)
