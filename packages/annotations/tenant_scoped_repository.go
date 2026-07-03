package annotations

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope
// with PostgresRepository, exactly as
// packages/casesearch.TenantScopedRepository and
// packages/signoff.TenantScopedRepository do: every method opens its
// own RLS-scoped transaction, so Row-Level Security enforces tenant
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
func (r *TenantScopedRepository) Create(ctx context.Context, tenantID uuid.UUID, a *Annotation) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Create(ctx, tenantID, a)
	})
}

// Get implements Repository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	var out *Annotation
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		a, err := NewPostgresRepository(exec).Get(ctx, tenantID, id)
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

// ListByCase implements Repository.
func (r *TenantScopedRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, filter AnchorFilter) ([]*Annotation, error) {
	var out []*Annotation
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

// Thread implements Repository.
func (r *TenantScopedRepository) Thread(ctx context.Context, tenantID, rootID uuid.UUID) ([]*Annotation, error) {
	var out []*Annotation
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).Thread(ctx, tenantID, rootID)
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

// UpdateBody implements Repository.
func (r *TenantScopedRepository) UpdateBody(ctx context.Context, tenantID, id uuid.UUID, body string) (*Annotation, error) {
	var out *Annotation
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		a, err := NewPostgresRepository(exec).UpdateBody(ctx, tenantID, id, body)
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

// Delete implements Repository.
func (r *TenantScopedRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Delete(ctx, tenantID, id)
	})
}

// Resolve implements Repository.
func (r *TenantScopedRepository) Resolve(ctx context.Context, tenantID, id, resolvedBy uuid.UUID) (*Annotation, error) {
	var out *Annotation
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		a, err := NewPostgresRepository(exec).Resolve(ctx, tenantID, id, resolvedBy)
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

// Reopen implements Repository.
func (r *TenantScopedRepository) Reopen(ctx context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	var out *Annotation
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		a, err := NewPostgresRepository(exec).Reopen(ctx, tenantID, id)
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

// MentionsFor implements Repository.
func (r *TenantScopedRepository) MentionsFor(ctx context.Context, tenantID, userID uuid.UUID) ([]Mention, error) {
	var out []Mention
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).MentionsFor(ctx, tenantID, userID)
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

// AppendAudit implements Repository.
func (r *TenantScopedRepository) AppendAudit(ctx context.Context, tenantID uuid.UUID, rec *AuditRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).AppendAudit(ctx, tenantID, rec)
	})
}

// ListAudit implements Repository.
func (r *TenantScopedRepository) ListAudit(ctx context.Context, tenantID, annotationID uuid.UUID) ([]*AuditRecord, error) {
	var out []*AuditRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).ListAudit(ctx, tenantID, annotationID)
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

var _ Repository = (*TenantScopedRepository)(nil)
