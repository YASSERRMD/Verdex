package identity

import (
	"context"

	"github.com/google/uuid"
)

// identityContextKey is the unexported key type for values stored in a
// context by this package. Using a named struct prevents collisions with
// keys from other packages that use the same underlying type.
type identityContextKey struct{}

// withUserKey is the key under which a *User is stored.
type withUserKey struct{}

// WithUser returns a copy of ctx carrying user as the authenticated
// identity for the current request. Downstream handlers retrieve it via
// UserFromContext.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, withUserKey{}, user)
}

// UserFromContext retrieves the *User attached to ctx by WithUser. It
// returns (nil, false) if no user has been stored on the context.
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(withUserKey{}).(*User)
	return u, ok && u != nil
}

// UserIDFromContext is a convenience helper that returns the ID of the
// user on ctx. It returns (uuid.Nil, false) when no user is present.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	u, ok := UserFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return u.ID, true
}

// RolesFromContext returns the []Role of the authenticated user on ctx.
// It returns (nil, false) when no user is present.
func RolesFromContext(ctx context.Context) ([]Role, bool) {
	u, ok := UserFromContext(ctx)
	if !ok {
		return nil, false
	}
	return u.Roles, true
}

// identityContextKey is exported so middleware in sibling packages can
// avoid re-declaring the same type. It is intentionally kept private to
// this package; the public API is the helper functions above.
var _ = identityContextKey{} // silence "declared and not used" if referenced
