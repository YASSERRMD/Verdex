package localization

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Preference is a single user's durable locale preference within a
// tenant (task 6's server-side half): which Locale their UI and
// generated reports should render in. This is the one genuinely
// stateful concern in this phase -- the Catalog itself
// (catalog.go) is process-wide, compiled-in seed data, but *which*
// locale a given user wants is a per-user setting that must survive a
// server restart and follow the user across devices/sessions, hence a
// real repository/migration (see repository.go,
// packages/persistence/migrations/000036_create_localization.up.sql)
// rather than an in-memory-only cookie value.
type Preference struct {
	// ID uniquely identifies this preference record.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this preference belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// UserID is the identity.User this preference is set for. A user
	// may have at most one Preference (see repository.go's
	// upsert-by-tenant-and-user semantics).
	UserID uuid.UUID `json:"user_id"`

	// Locale is the user's preferred locale.
	Locale Locale `json:"locale"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks p for structural well-formedness.
func (p *Preference) Validate() error {
	if p == nil {
		return ErrInvalidPreference
	}
	if p.TenantID == uuid.Nil {
		return wrapf("Preference.Validate", ErrEmptyTenantID)
	}
	if p.UserID == uuid.Nil {
		return wrapf("Preference.Validate", ErrInvalidPreference)
	}
	if !p.Locale.IsValid() {
		return wrapf("Preference.Validate", ErrInvalidLocale)
	}
	return nil
}

// PreferenceRepository persists Preference records, scoped to a
// tenant on every call, mirroring
// packages/privacy.InventoryRepository's conventions. Upsert sets or
// replaces the caller's Preference for (tenantID, userID) -- a user
// has at most one Preference row, so there is no separate Create vs
// Update distinction a caller needs to track.
type PreferenceRepository interface {
	Upsert(ctx context.Context, tenantID uuid.UUID, p *Preference) error
	Get(ctx context.Context, tenantID, userID uuid.UUID) (*Preference, error)
	Delete(ctx context.Context, tenantID, userID uuid.UUID) error
}
