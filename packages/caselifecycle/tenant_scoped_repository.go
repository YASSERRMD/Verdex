package caselifecycle

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope
// with PostgresRepository, exactly as
// packages/tenancy.TenantScopedDeploymentRepository composes it with
// persistence.DeploymentRepository: every method opens its own
// RLS-scoped transaction, so Row-Level Security enforces tenant
// isolation at the database layer in addition to PostgresRepository's
// own application-level requireMatchingTenant guard.
//
// This is the type production code should use against a live
// *pgxpool.Pool. PostgresRepository remains available directly for
// callers that already hold an open persistence.Executor (e.g. inside
// a larger transaction that also touches other tables).
type TenantScopedRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedRepository builds a TenantScopedRepository backed by
// pool.
func NewTenantScopedRepository(pool *pgxpool.Pool) *TenantScopedRepository {
	return &TenantScopedRepository{pool: pool}
}

// Create implements Repository.
func (r *TenantScopedRepository) Create(ctx context.Context, tenantID uuid.UUID, c *Case) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Create(ctx, tenantID, c)
	})
}

// Get implements Repository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Case, error) {
	var out *Case
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		c, err := NewPostgresRepository(exec).Get(ctx, tenantID, id)
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

// List implements Repository.
func (r *TenantScopedRepository) List(ctx context.Context, tenantID uuid.UUID, filter CaseFilter) ([]*Case, error) {
	var out []*Case
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		cs, err := NewPostgresRepository(exec).List(ctx, tenantID, filter)
		if err != nil {
			return err
		}
		out = cs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Update implements Repository.
func (r *TenantScopedRepository) Update(ctx context.Context, tenantID uuid.UUID, c *Case) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Update(ctx, tenantID, c)
	})
}

// AppendTransition implements Repository.
func (r *TenantScopedRepository) AppendTransition(ctx context.Context, tenantID uuid.UUID, rec *TransitionRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).AppendTransition(ctx, tenantID, rec)
	})
}

// ListTransitions implements Repository.
func (r *TenantScopedRepository) ListTransitions(ctx context.Context, tenantID, caseID uuid.UUID) ([]*TransitionRecord, error) {
	var out []*TransitionRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		recs, err := NewPostgresRepository(exec).ListTransitions(ctx, tenantID, caseID)
		if err != nil {
			return err
		}
		out = recs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ Repository = (*TenantScopedRepository)(nil)
