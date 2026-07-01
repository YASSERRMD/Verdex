package tenancy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// TenantScopedDeploymentRepository composes WithTenantScope with
// persistence.DeploymentRepository so callers get a repository whose
// methods already take a tenantID and internally open the
// RLS-scoped transaction described in scope.go, rather than a plain
// persistence.DeploymentRepository that application code could
// accidentally call without tenant scoping for tenant-owned data.
//
// Every method here opens its own WithTenantScope transaction; there
// is no way to obtain the underlying persistence.DeploymentRepository
// or *pgxpool.Pool from a TenantScopedDeploymentRepository, by design.
type TenantScopedDeploymentRepository struct {
	pool *pgxpool.Pool
	repo persistence.DeploymentRepository
}

// NewTenantScopedDeploymentRepository builds a
// TenantScopedDeploymentRepository backed by pool and repo. repo is
// typically persistence.NewPostgresDeploymentRepository(); it is
// accepted as an interface so tests can substitute a fake.
func NewTenantScopedDeploymentRepository(pool *pgxpool.Pool, repo persistence.DeploymentRepository) *TenantScopedDeploymentRepository {
	return &TenantScopedDeploymentRepository{pool: pool, repo: repo}
}

// Create inserts d for tenantID, refusing (via ErrCrossTenantAccess)
// if d.TenantID is already set to a different tenant. On success it
// also stamps d.TenantID = tenantID before insert, for callers that
// construct a Deployment without setting TenantID explicitly.
func (r *TenantScopedDeploymentRepository) Create(ctx context.Context, tenantID uuid.UUID, d *persistence.Deployment) error {
	if d == nil {
		return fmt.Errorf("tenancy: TenantScopedDeploymentRepository.Create: d must not be nil")
	}
	if d.TenantID == uuid.Nil {
		d.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}

	return WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return r.repo.Create(ctx, exec, d)
	})
}

// Get returns the deployment with the given id, scoped to tenantID.
// Because the underlying query runs inside a WithTenantScope
// transaction, Row-Level Security alone guarantees a deployment
// belonging to a different tenant is invisible (returned as
// persistence.ErrNotFound), even if this defense-in-depth layer were
// somehow bypassed.
func (r *TenantScopedDeploymentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*persistence.Deployment, error) {
	var out *persistence.Deployment
	err := WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		d, err := r.repo.Get(ctx, exec, id)
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

// List returns every deployment visible to tenantID (i.e. every
// deployment RLS permits under app.current_tenant_id = tenantID).
func (r *TenantScopedDeploymentRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*persistence.Deployment, error) {
	var out []*persistence.Deployment
	err := WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		ds, err := r.repo.List(ctx, exec)
		if err != nil {
			return err
		}
		out = ds
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Update overwrites the mutable fields of d, refusing (via
// ErrCrossTenantAccess) before any database access if d.TenantID does
// not match tenantID.
func (r *TenantScopedDeploymentRepository) Update(ctx context.Context, tenantID uuid.UUID, d *persistence.Deployment) error {
	if d == nil {
		return fmt.Errorf("tenancy: TenantScopedDeploymentRepository.Update: d must not be nil")
	}
	if err := requireMatchingTenant(tenantID, d.TenantID); err != nil {
		return err
	}

	return WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return r.repo.Update(ctx, exec, d)
	})
}

// Delete removes the deployment with the given id, scoped to
// tenantID. A deployment belonging to a different tenant is invisible
// under RLS and Delete reports persistence.ErrNotFound for it, exactly
// as it would for a nonexistent id.
func (r *TenantScopedDeploymentRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return r.repo.Delete(ctx, exec, id)
	})
}
