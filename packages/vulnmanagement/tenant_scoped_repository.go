package vulnmanagement

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedFindingRepository composes
// packages/tenancy.WithTenantScope with PostgresFindingRepository,
// exactly as packages/compliance.TenantScopedEvidenceRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresFindingRepository's own application-level
// requireMatchingTenant guard.
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

func (r *TenantScopedFindingRepository) ListBySource(ctx context.Context, tenantID uuid.UUID, source ScannerSource) ([]Finding, error) {
	var out []Finding
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFindingRepository(exec).ListBySource(ctx, tenantID, source)
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

func (r *TenantScopedFindingRepository) ListByStatus(ctx context.Context, tenantID uuid.UUID, status Status) ([]Finding, error) {
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

// TenantScopedTriageRepository composes packages/tenancy.WithTenantScope
// with PostgresTriageRepository.
type TenantScopedTriageRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedTriageRepository builds a TenantScopedTriageRepository
// backed by pool.
func NewTenantScopedTriageRepository(pool *pgxpool.Pool) *TenantScopedTriageRepository {
	return &TenantScopedTriageRepository{pool: pool}
}

func (r *TenantScopedTriageRepository) Create(ctx context.Context, tenantID uuid.UUID, d *TriageDecision) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresTriageRepository(exec).Create(ctx, tenantID, d)
	})
}

func (r *TenantScopedTriageRepository) ListForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]TriageDecision, error) {
	var out []TriageDecision
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresTriageRepository(exec).ListForFinding(ctx, tenantID, findingID)
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

func (r *TenantScopedTriageRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]TriageDecision, error) {
	var out []TriageDecision
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresTriageRepository(exec).ListAll(ctx, tenantID)
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

var _ TriageRepository = (*TenantScopedTriageRepository)(nil)
