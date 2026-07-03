package notifications

import (
	"context"

	"github.com/google/uuid"
)

// Filter narrows List to a subset of a recipient's notifications.
type Filter struct {
	// UnreadOnly, if true, restricts results to notifications with a
	// nil ReadAt.
	UnreadOnly bool

	// Kind, if non-empty, restricts results to this Kind.
	Kind Kind

	// Limit caps the number of results returned. Zero means "no
	// limit" (Repository implementations may still apply a sane
	// default).
	Limit int
}

// Repository persists Notification records, scoped to a tenant on
// every call, mirroring packages/caseversioning.Repository's
// convention exactly. Implementations must refuse (via
// ErrCrossTenantAccess) to operate on a Notification whose TenantID
// does not match the tenantID argument.
//
// Two implementations are provided: InMemoryRepository (tests and
// other packages' fixtures) and PostgresRepository/
// TenantScopedRepository (backed by the `notifications` table — see
// packages/persistence/migrations/000016_create_notifications.up.sql).
type Repository interface {
	// Create inserts n. n.ID is generated if zero, and n.CreatedAt is
	// set if zero. Returns validation errors from n.Validate() and
	// ErrCrossTenantAccess if n.TenantID does not match tenantID.
	Create(ctx context.Context, tenantID uuid.UUID, n *Notification) error

	// Get returns the notification with the given id, scoped to
	// tenantID. Returns ErrNotFound if no such notification is visible
	// to tenantID.
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Notification, error)

	// ListForRecipient returns notifications addressed to recipientID
	// visible to tenantID, optionally narrowed by filter, ordered by
	// CreatedAt descending (newest first).
	ListForRecipient(ctx context.Context, tenantID, recipientID uuid.UUID, filter Filter) ([]*Notification, error)

	// UnreadCount returns the number of unread notifications addressed
	// to recipientID, visible to tenantID.
	UnreadCount(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error)

	// MarkRead sets ReadAt (to the current time, if not already set)
	// on the notification identified by id, scoped to tenantID and
	// recipientID. Returns ErrNotFound if no such notification is
	// visible. Idempotent — marking an already-read notification read
	// again is not an error.
	MarkRead(ctx context.Context, tenantID, recipientID, id uuid.UUID) error

	// MarkAllRead sets ReadAt on every unread notification addressed
	// to recipientID, visible to tenantID. Returns the number of
	// notifications newly marked read.
	MarkAllRead(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error)
}

// PreferenceRepository persists per-user, per-Kind Preference records,
// scoped to a tenant on every call, mirroring Repository's convention.
type PreferenceRepository interface {
	// Upsert inserts or updates p, keyed by (TenantID, UserID, Kind).
	// Returns validation errors from p.Validate() and
	// ErrCrossTenantAccess if p.TenantID does not match tenantID.
	Upsert(ctx context.Context, tenantID uuid.UUID, p *Preference) error

	// Get returns the Preference for (userID, kind), scoped to
	// tenantID. Returns ErrNotFound if no explicit Preference row
	// exists — callers should treat that as "default: enabled, in-app
	// only" (see Service.isEnabled), not as an error condition to
	// surface to the end user.
	Get(ctx context.Context, tenantID, userID uuid.UUID, kind Kind) (*Preference, error)

	// ListForUser returns every explicit Preference row for userID,
	// scoped to tenantID. Kinds with no explicit row are not included
	// — callers apply the same "default: enabled" rule as Get.
	ListForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]*Preference, error)
}
