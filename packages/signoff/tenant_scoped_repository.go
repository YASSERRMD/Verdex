package signoff

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope
// with PostgresRepository, exactly as
// packages/caselifecycle.TenantScopedRepository does: every method
// opens its own RLS-scoped transaction, so Row-Level Security
// enforces tenant isolation at the database layer in addition to
// PostgresRepository's own application-level requireMatchingTenant
// guard.
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

// Get implements Repository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, error) {
	var out *SignoffRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		rec, err := NewPostgresRepository(exec).Get(ctx, tenantID, caseID)
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

// Upsert implements Repository.
func (r *TenantScopedRepository) Upsert(ctx context.Context, tenantID uuid.UUID, rec *SignoffRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Upsert(ctx, tenantID, rec)
	})
}

// AppendAudit implements Repository.
func (r *TenantScopedRepository) AppendAudit(ctx context.Context, tenantID uuid.UUID, e *AuditEntry) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).AppendAudit(ctx, tenantID, e)
	})
}

// ListAudit implements Repository.
func (r *TenantScopedRepository) ListAudit(ctx context.Context, tenantID, caseID uuid.UUID) ([]*AuditEntry, error) {
	var out []*AuditEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		entries, err := NewPostgresRepository(exec).ListAudit(ctx, tenantID, caseID)
		if err != nil {
			return err
		}
		out = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ Repository = (*TenantScopedRepository)(nil)
