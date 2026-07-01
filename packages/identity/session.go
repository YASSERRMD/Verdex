package identity

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Session represents a server-side session associated with a validated
// user. Sessions are created by the authentication flow after a token is
// verified and are refreshed on activity. They are independent of the
// underlying token mechanism: the bearer token identifies who the user is,
// and the session carries the hydrated, server-authoritative state.
type Session struct {
	// ID is the globally unique session identifier. It is opaque to the
	// client; the client presents the bearer token, not the session ID.
	ID uuid.UUID

	// UserID is the user this session belongs to.
	UserID uuid.UUID

	// TenantID is the tenant this session is scoped to.
	TenantID uuid.UUID

	// Roles holds the roles active for this session. These are resolved
	// at session creation time from the user's current role assignments.
	Roles []Role

	// ExpiresAt is the absolute time after which this session is no
	// longer valid and must be rejected even if the bearer token has not
	// yet expired.
	ExpiresAt time.Time

	// CreatedAt is the time this session was first established.
	CreatedAt time.Time
}

// IsExpired reports whether the session has passed ExpiresAt.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// SessionStore is the persistence interface for Session objects. The
// standard implementation will write to a database or cache; tests use
// an in-memory map.
//
// All methods receive a context so callers can propagate deadlines and
// cancellation.
type SessionStore interface {
	// Create persists a new session. It returns an error if a session
	// with the same ID already exists or if the store is unavailable.
	Create(ctx context.Context, session *Session) error

	// Get retrieves the session identified by id. It returns
	// ErrUserNotFound (reused as a sentinel) if no such session exists,
	// or ErrTokenExpired if the session exists but has expired.
	Get(ctx context.Context, id uuid.UUID) (*Session, error)

	// Delete removes the session identified by id. It is idempotent:
	// deleting a session that does not exist returns nil.
	Delete(ctx context.Context, id uuid.UUID) error

	// Refresh extends the ExpiresAt of an existing session by the
	// implementation-defined TTL. It returns ErrUserNotFound if the
	// session does not exist.
	Refresh(ctx context.Context, id uuid.UUID) (*Session, error)
}
