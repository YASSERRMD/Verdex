package notifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedRepository composes packages/tenancy.WithTenantScope
// with PostgresRepository, exactly as
// packages/caseversioning.TenantScopedRepository does: every method
// opens its own RLS-scoped transaction, so Row-Level Security enforces
// tenant isolation at the database layer in addition to
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

// Create implements Repository.
func (r *TenantScopedRepository) Create(ctx context.Context, tenantID uuid.UUID, n *Notification) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).Create(ctx, tenantID, n)
	})
}

// Get implements Repository.
func (r *TenantScopedRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*Notification, error) {
	var out *Notification
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		n, err := NewPostgresRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = n
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListForRecipient implements Repository.
func (r *TenantScopedRepository) ListForRecipient(ctx context.Context, tenantID, recipientID uuid.UUID, filter Filter) ([]*Notification, error) {
	var out []*Notification
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresRepository(exec).ListForRecipient(ctx, tenantID, recipientID, filter)
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

// UnreadCount implements Repository.
func (r *TenantScopedRepository) UnreadCount(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	var out int
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		count, err := NewPostgresRepository(exec).UnreadCount(ctx, tenantID, recipientID)
		if err != nil {
			return err
		}
		out = count
		return nil
	})
	if err != nil {
		return 0, err
	}
	return out, nil
}

// MarkRead implements Repository.
func (r *TenantScopedRepository) MarkRead(ctx context.Context, tenantID, recipientID, id uuid.UUID) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresRepository(exec).MarkRead(ctx, tenantID, recipientID, id)
	})
}

// MarkAllRead implements Repository.
func (r *TenantScopedRepository) MarkAllRead(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	var out int
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		count, err := NewPostgresRepository(exec).MarkAllRead(ctx, tenantID, recipientID)
		if err != nil {
			return err
		}
		out = count
		return nil
	})
	if err != nil {
		return 0, err
	}
	return out, nil
}

var _ Repository = (*TenantScopedRepository)(nil)

// TenantScopedPreferenceRepository composes
// packages/tenancy.WithTenantScope with PostgresPreferenceRepository,
// mirroring TenantScopedRepository exactly.
type TenantScopedPreferenceRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedPreferenceRepository builds a
// TenantScopedPreferenceRepository backed by pool.
func NewTenantScopedPreferenceRepository(pool *pgxpool.Pool) *TenantScopedPreferenceRepository {
	return &TenantScopedPreferenceRepository{pool: pool}
}

// Upsert implements PreferenceRepository.
func (r *TenantScopedPreferenceRepository) Upsert(ctx context.Context, tenantID uuid.UUID, p *Preference) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresPreferenceRepository(exec).Upsert(ctx, tenantID, p)
	})
}

// Get implements PreferenceRepository.
func (r *TenantScopedPreferenceRepository) Get(ctx context.Context, tenantID, userID uuid.UUID, kind Kind) (*Preference, error) {
	var out *Preference
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		p, err := NewPostgresPreferenceRepository(exec).Get(ctx, tenantID, userID, kind)
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

// ListForUser implements PreferenceRepository.
func (r *TenantScopedPreferenceRepository) ListForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]*Preference, error) {
	var out []*Preference
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresPreferenceRepository(exec).ListForUser(ctx, tenantID, userID)
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

var _ PreferenceRepository = (*TenantScopedPreferenceRepository)(nil)
