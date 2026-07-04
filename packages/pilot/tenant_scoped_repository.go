package pilot

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedDeploymentRepository composes
// packages/tenancy.WithTenantScope with PostgresDeploymentRepository,
// exactly as packages/compliance.TenantScopedEvidenceRepository does:
// every method opens its own RLS-scoped transaction, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to PostgresDeploymentRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedDeploymentRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedDeploymentRepository builds a
// TenantScopedDeploymentRepository backed by pool.
func NewTenantScopedDeploymentRepository(pool *pgxpool.Pool) *TenantScopedDeploymentRepository {
	return &TenantScopedDeploymentRepository{pool: pool}
}

func (r *TenantScopedDeploymentRepository) Create(ctx context.Context, tenantID uuid.UUID, d *PilotDeployment) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresDeploymentRepository(exec).Create(ctx, tenantID, d)
	})
}

func (r *TenantScopedDeploymentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotDeployment, error) {
	var out *PilotDeployment
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		d, err := NewPostgresDeploymentRepository(exec).Get(ctx, tenantID, id)
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

func (r *TenantScopedDeploymentRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]PilotDeployment, error) {
	var out []PilotDeployment
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresDeploymentRepository(exec).ListAll(ctx, tenantID)
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

func (r *TenantScopedDeploymentRepository) Update(ctx context.Context, tenantID uuid.UUID, d *PilotDeployment) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresDeploymentRepository(exec).Update(ctx, tenantID, d)
	})
}

var _ DeploymentRepository = (*TenantScopedDeploymentRepository)(nil)

// TenantScopedCaseRepository composes packages/tenancy.WithTenantScope
// with PostgresCaseRepository.
type TenantScopedCaseRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedCaseRepository builds a TenantScopedCaseRepository
// backed by pool.
func NewTenantScopedCaseRepository(pool *pgxpool.Pool) *TenantScopedCaseRepository {
	return &TenantScopedCaseRepository{pool: pool}
}

func (r *TenantScopedCaseRepository) Create(ctx context.Context, tenantID uuid.UUID, c *PilotCase) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresCaseRepository(exec).Create(ctx, tenantID, c)
	})
}

func (r *TenantScopedCaseRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotCase, error) {
	var out *PilotCase
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		c, err := NewPostgresCaseRepository(exec).Get(ctx, tenantID, id)
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

func (r *TenantScopedCaseRepository) ListForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotCase, error) {
	var out []PilotCase
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresCaseRepository(exec).ListForDeployment(ctx, tenantID, deploymentID)
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

func (r *TenantScopedCaseRepository) Update(ctx context.Context, tenantID uuid.UUID, c *PilotCase) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresCaseRepository(exec).Update(ctx, tenantID, c)
	})
}

var _ CaseRepository = (*TenantScopedCaseRepository)(nil)

// TenantScopedFeedbackRepository composes
// packages/tenancy.WithTenantScope with PostgresFeedbackRepository.
type TenantScopedFeedbackRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedFeedbackRepository builds a
// TenantScopedFeedbackRepository backed by pool.
func NewTenantScopedFeedbackRepository(pool *pgxpool.Pool) *TenantScopedFeedbackRepository {
	return &TenantScopedFeedbackRepository{pool: pool}
}

func (r *TenantScopedFeedbackRepository) Create(ctx context.Context, tenantID uuid.UUID, f *FeedbackEntry) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFeedbackRepository(exec).Create(ctx, tenantID, f)
	})
}

func (r *TenantScopedFeedbackRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*FeedbackEntry, error) {
	var out *FeedbackEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		f, err := NewPostgresFeedbackRepository(exec).Get(ctx, tenantID, id)
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

func (r *TenantScopedFeedbackRepository) ListForCase(ctx context.Context, tenantID, pilotCaseID uuid.UUID) ([]FeedbackEntry, error) {
	var out []FeedbackEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFeedbackRepository(exec).ListForCase(ctx, tenantID, pilotCaseID)
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

func (r *TenantScopedFeedbackRepository) ListForDeployment(ctx context.Context, tenantID uuid.UUID, pilotCaseIDs []uuid.UUID) ([]FeedbackEntry, error) {
	var out []FeedbackEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFeedbackRepository(exec).ListForDeployment(ctx, tenantID, pilotCaseIDs)
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

func (r *TenantScopedFeedbackRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]FeedbackEntry, error) {
	var out []FeedbackEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFeedbackRepository(exec).ListAll(ctx, tenantID)
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

var _ FeedbackRepository = (*TenantScopedFeedbackRepository)(nil)

// TenantScopedFindingRepository composes
// packages/tenancy.WithTenantScope with PostgresFindingRepository.
type TenantScopedFindingRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedFindingRepository builds a
// TenantScopedFindingRepository backed by pool.
func NewTenantScopedFindingRepository(pool *pgxpool.Pool) *TenantScopedFindingRepository {
	return &TenantScopedFindingRepository{pool: pool}
}

func (r *TenantScopedFindingRepository) Create(ctx context.Context, tenantID uuid.UUID, f *PilotFinding) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFindingRepository(exec).Create(ctx, tenantID, f)
	})
}

func (r *TenantScopedFindingRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotFinding, error) {
	var out *PilotFinding
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

func (r *TenantScopedFindingRepository) ListForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotFinding, error) {
	var out []PilotFinding
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresFindingRepository(exec).ListForDeployment(ctx, tenantID, deploymentID)
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

func (r *TenantScopedFindingRepository) Update(ctx context.Context, tenantID uuid.UUID, f *PilotFinding) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresFindingRepository(exec).Update(ctx, tenantID, f)
	})
}

var _ FindingRepository = (*TenantScopedFindingRepository)(nil)

// TenantScopedRefinementRepository composes
// packages/tenancy.WithTenantScope with PostgresRefinementRepository.
type TenantScopedRefinementRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedRefinementRepository builds a
// TenantScopedRefinementRepository backed by pool.
func NewTenantScopedRefinementRepository(pool *pgxpool.Pool) *TenantScopedRefinementRepository {
	return &TenantScopedRefinementRepository{pool: pool}
}

func (r *TenantScopedRefinementRepository) Create(ctx context.Context, tenantID uuid.UUID, rec *RefinementRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRefinementRepository(exec).Create(ctx, tenantID, rec)
	})
}

func (r *TenantScopedRefinementRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*RefinementRecord, error) {
	var out *RefinementRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rec, err := NewPostgresRefinementRepository(exec).Get(ctx, tenantID, id)
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

func (r *TenantScopedRefinementRepository) ListForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]RefinementRecord, error) {
	var out []RefinementRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRefinementRepository(exec).ListForFinding(ctx, tenantID, findingID)
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

func (r *TenantScopedRefinementRepository) ListForDeployment(ctx context.Context, tenantID uuid.UUID, findingIDs []uuid.UUID) ([]RefinementRecord, error) {
	var out []RefinementRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRefinementRepository(exec).ListForDeployment(ctx, tenantID, findingIDs)
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

func (r *TenantScopedRefinementRepository) Update(ctx context.Context, tenantID uuid.UUID, rec *RefinementRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRefinementRepository(exec).Update(ctx, tenantID, rec)
	})
}

var _ RefinementRepository = (*TenantScopedRefinementRepository)(nil)
