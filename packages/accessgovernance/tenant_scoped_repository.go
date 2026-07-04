package accessgovernance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedPolicyRepository composes
// packages/tenancy.WithTenantScope with PostgresPolicyRepository,
// exactly as packages/keymanagement.TenantScopedRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in
// addition to PostgresPolicyRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedPolicyRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedPolicyRepository builds a
// TenantScopedPolicyRepository backed by pool.
func NewTenantScopedPolicyRepository(pool *pgxpool.Pool) *TenantScopedPolicyRepository {
	return &TenantScopedPolicyRepository{pool: pool}
}

func (r *TenantScopedPolicyRepository) Create(ctx context.Context, tenantID uuid.UUID, p *Policy) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresPolicyRepository(exec).Create(ctx, tenantID, p)
	})
}

func (r *TenantScopedPolicyRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Policy, error) {
	var out *Policy
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		p, err := NewPostgresPolicyRepository(exec).Get(ctx, tenantID, id)
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

func (r *TenantScopedPolicyRepository) List(ctx context.Context, tenantID uuid.UUID) ([]Policy, error) {
	var out []Policy
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresPolicyRepository(exec).List(ctx, tenantID)
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

func (r *TenantScopedPolicyRepository) Update(ctx context.Context, tenantID uuid.UUID, p *Policy) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresPolicyRepository(exec).Update(ctx, tenantID, p)
	})
}

var _ PolicyRepository = (*TenantScopedPolicyRepository)(nil)

// TenantScopedCaseGrantRepository composes
// packages/tenancy.WithTenantScope with PostgresCaseGrantRepository.
type TenantScopedCaseGrantRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedCaseGrantRepository builds a
// TenantScopedCaseGrantRepository backed by pool.
func NewTenantScopedCaseGrantRepository(pool *pgxpool.Pool) *TenantScopedCaseGrantRepository {
	return &TenantScopedCaseGrantRepository{pool: pool}
}

func (r *TenantScopedCaseGrantRepository) Create(ctx context.Context, tenantID uuid.UUID, g *CaseGrant) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresCaseGrantRepository(exec).Create(ctx, tenantID, g)
	})
}

func (r *TenantScopedCaseGrantRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*CaseGrant, error) {
	var out *CaseGrant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		g, err := NewPostgresCaseGrantRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = g
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedCaseGrantRepository) ListForCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]CaseGrant, error) {
	var out []CaseGrant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresCaseGrantRepository(exec).ListForCase(ctx, tenantID, caseID)
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

func (r *TenantScopedCaseGrantRepository) ListActive(ctx context.Context, tenantID uuid.UUID, now time.Time) ([]CaseGrant, error) {
	var out []CaseGrant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresCaseGrantRepository(exec).ListActive(ctx, tenantID, now)
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

func (r *TenantScopedCaseGrantRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]CaseGrant, error) {
	var out []CaseGrant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresCaseGrantRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedCaseGrantRepository) Revoke(ctx context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresCaseGrantRepository(exec).Revoke(ctx, tenantID, id, revokedAt)
	})
}

var _ CaseGrantRepository = (*TenantScopedCaseGrantRepository)(nil)

// TenantScopedGrantRepository composes packages/tenancy.WithTenantScope
// with PostgresGrantRepository.
type TenantScopedGrantRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedGrantRepository builds a TenantScopedGrantRepository
// backed by pool.
func NewTenantScopedGrantRepository(pool *pgxpool.Pool) *TenantScopedGrantRepository {
	return &TenantScopedGrantRepository{pool: pool}
}

func (r *TenantScopedGrantRepository) Create(ctx context.Context, tenantID uuid.UUID, g *Grant) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresGrantRepository(exec).Create(ctx, tenantID, g)
	})
}

func (r *TenantScopedGrantRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Grant, error) {
	var out *Grant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		g, err := NewPostgresGrantRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = g
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedGrantRepository) ListActive(ctx context.Context, tenantID uuid.UUID, now time.Time) ([]Grant, error) {
	var out []Grant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresGrantRepository(exec).ListActive(ctx, tenantID, now)
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

func (r *TenantScopedGrantRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Grant, error) {
	var out []Grant
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresGrantRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedGrantRepository) Revoke(ctx context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresGrantRepository(exec).Revoke(ctx, tenantID, id, revokedAt)
	})
}

var _ GrantRepository = (*TenantScopedGrantRepository)(nil)

// TenantScopedReviewRepository composes
// packages/tenancy.WithTenantScope with PostgresReviewRepository.
type TenantScopedReviewRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedReviewRepository builds a
// TenantScopedReviewRepository backed by pool.
func NewTenantScopedReviewRepository(pool *pgxpool.Pool) *TenantScopedReviewRepository {
	return &TenantScopedReviewRepository{pool: pool}
}

func (r *TenantScopedReviewRepository) Create(ctx context.Context, tenantID uuid.UUID, rv *Review) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresReviewRepository(exec).Create(ctx, tenantID, rv)
	})
}

func (r *TenantScopedReviewRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Review, error) {
	var out *Review
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rv, err := NewPostgresReviewRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = rv
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedReviewRepository) ListDue(ctx context.Context, tenantID uuid.UUID, asOf time.Time) ([]Review, error) {
	var out []Review
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresReviewRepository(exec).ListDue(ctx, tenantID, asOf)
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

func (r *TenantScopedReviewRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]Review, error) {
	var out []Review
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresReviewRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedReviewRepository) Update(ctx context.Context, tenantID uuid.UUID, rv *Review) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresReviewRepository(exec).Update(ctx, tenantID, rv)
	})
}

var _ ReviewRepository = (*TenantScopedReviewRepository)(nil)
