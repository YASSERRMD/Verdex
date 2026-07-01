package identity

import (
	"context"

	"github.com/google/uuid"
)

// UserRepository is the persistence interface for User records. It
// isolates the domain from any particular storage technology (SQL, NoSQL,
// in-memory). Callers access users exclusively through this interface;
// concrete implementations live in application-layer packages, not here.
//
// All operations are tenant-scoped: a correct implementation must never
// return data belonging to a different tenant than the one embedded in
// the context or supplied as an argument.
type UserRepository interface {
	// Create persists a new user record. The caller must set u.ID and
	// u.TenantID before calling. Create returns an error if a user with
	// the same ID or Email+TenantID combination already exists.
	Create(ctx context.Context, u *User) error

	// GetByID retrieves the user identified by id. Returns ErrUserNotFound
	// if no matching record exists.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// GetByEmail retrieves the user with the given email address within
	// tenantID. Returns ErrUserNotFound if no matching record exists.
	GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error)

	// Update persists changes to an existing user record. The caller must
	// not change u.ID or u.TenantID. Returns ErrUserNotFound if the
	// record does not exist.
	Update(ctx context.Context, u *User) error

	// ListByTenant returns all user records that belong to tenantID.
	// The result slice may be empty but is never nil on a successful
	// call.
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*User, error)

	// Delete removes the user record identified by id. It is idempotent:
	// deleting a user that does not exist returns nil.
	Delete(ctx context.Context, id uuid.UUID) error
}
