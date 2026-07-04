package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedEvidenceRepository composes
// packages/tenancy.WithTenantScope with PostgresEvidenceRepository,
// exactly as packages/privacy.TenantScopedInventoryRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresEvidenceRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedEvidenceRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedEvidenceRepository builds a
// TenantScopedEvidenceRepository backed by pool.
func NewTenantScopedEvidenceRepository(pool *pgxpool.Pool) *TenantScopedEvidenceRepository {
	return &TenantScopedEvidenceRepository{pool: pool}
}

func (r *TenantScopedEvidenceRepository) Create(ctx context.Context, tenantID uuid.UUID, e *ControlEvidence) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresEvidenceRepository(exec).Create(ctx, tenantID, e)
	})
}

func (r *TenantScopedEvidenceRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ControlEvidence, error) {
	var out *ControlEvidence
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		e, err := NewPostgresEvidenceRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedEvidenceRepository) ListForControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]ControlEvidence, error) {
	var out []ControlEvidence
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresEvidenceRepository(exec).ListForControl(ctx, tenantID, controlID)
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

func (r *TenantScopedEvidenceRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ControlEvidence, error) {
	var out []ControlEvidence
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresEvidenceRepository(exec).ListAll(ctx, tenantID)
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

var _ EvidenceRepository = (*TenantScopedEvidenceRepository)(nil)

// TenantScopedProfileRepository composes
// packages/tenancy.WithTenantScope with PostgresProfileRepository.
type TenantScopedProfileRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedProfileRepository builds a
// TenantScopedProfileRepository backed by pool.
func NewTenantScopedProfileRepository(pool *pgxpool.Pool) *TenantScopedProfileRepository {
	return &TenantScopedProfileRepository{pool: pool}
}

func (r *TenantScopedProfileRepository) Set(ctx context.Context, tenantID uuid.UUID, p *Profile) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresProfileRepository(exec).Set(ctx, tenantID, p)
	})
}

func (r *TenantScopedProfileRepository) Get(ctx context.Context, tenantID uuid.UUID) (*Profile, error) {
	var out *Profile
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		p, err := NewPostgresProfileRepository(exec).Get(ctx, tenantID)
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

var _ ProfileRepository = (*TenantScopedProfileRepository)(nil)

// TenantScopedControlRepository wraps PostgresControlRepository with
// a plain, unscoped pool connection -- unlike the two repositories
// above, compliance_controls carries no tenant_id column and no RLS
// policy (see PostgresControlRepository's doc comment), so there is no
// tenant scope to apply via packages/tenancy.WithTenantScope. This
// type exists purely so callers wiring up production dependencies have
// one consistent "Tenant-scoped-repository-set" constructor family to
// reach for per package, even though this particular repository's
// scoping is a no-op by design.
type TenantScopedControlRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedControlRepository builds a
// TenantScopedControlRepository backed by pool.
func NewTenantScopedControlRepository(pool *pgxpool.Pool) *TenantScopedControlRepository {
	return &TenantScopedControlRepository{pool: pool}
}

func (r *TenantScopedControlRepository) Create(ctx context.Context, c *Control) error {
	return NewPostgresControlRepository(r.pool).Create(ctx, c)
}

func (r *TenantScopedControlRepository) Get(ctx context.Context, id uuid.UUID) (*Control, error) {
	return NewPostgresControlRepository(r.pool).Get(ctx, id)
}

func (r *TenantScopedControlRepository) GetByCode(ctx context.Context, code string) (*Control, error) {
	return NewPostgresControlRepository(r.pool).GetByCode(ctx, code)
}

func (r *TenantScopedControlRepository) List(ctx context.Context) ([]Control, error) {
	return NewPostgresControlRepository(r.pool).List(ctx)
}

func (r *TenantScopedControlRepository) ListByFramework(ctx context.Context, framework Framework) ([]Control, error) {
	return NewPostgresControlRepository(r.pool).ListByFramework(ctx, framework)
}

func (r *TenantScopedControlRepository) Update(ctx context.Context, c *Control) error {
	return NewPostgresControlRepository(r.pool).Update(ctx, c)
}

var _ ControlRepository = (*TenantScopedControlRepository)(nil)
