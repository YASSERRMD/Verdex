package caseversioning

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope with
// PostgresRepository, exactly as packages/annotations.TenantScopedRepository
// and packages/casesearch.TenantScopedRepository do: every method opens
// its own RLS-scoped transaction, so Row-Level Security enforces tenant
// isolation at the database layer in addition to PostgresRepository's
// own application-level requireMatchingTenant guard.
//
// This is the type production code should use against a live
// *pgxpool.Pool. PostgresRepository remains available directly for
// callers that already hold an open persistence.Executor.
type TenantScopedRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedRepository builds a TenantScopedRepository backed by
// pool.
func NewTenantScopedRepository(pool *pgxpool.Pool) *TenantScopedRepository {
	return &TenantScopedRepository{pool: pool}
}

// Create implements Repository.
func (r *TenantScopedRepository) Create(ctx context.Context, tenantID uuid.UUID, s *Snapshot) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Create(ctx, tenantID, s)
	})
}

// Get implements Repository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Snapshot, error) {
	var out *Snapshot
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		s, err := NewPostgresRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListByCase implements Repository.
func (r *TenantScopedRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter SnapshotFilter) ([]*Snapshot, error) {
	var out []*Snapshot
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).ListByCase(ctx, tenantID, caseID, filter)
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

// Latest implements Repository.
func (r *TenantScopedRepository) Latest(ctx context.Context, tenantID, caseID uuid.UUID, kind ArtifactKind) (*Snapshot, error) {
	var out *Snapshot
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		s, err := NewPostgresRepository(exec).Latest(ctx, tenantID, caseID, kind)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ Repository = (*TenantScopedRepository)(nil)
