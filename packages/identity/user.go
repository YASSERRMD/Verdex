package identity

import (
	"time"

	"github.com/google/uuid"
)

// UserStatus represents the lifecycle state of a user account within a
// tenant.
type UserStatus string

const (
	// UserStatusActive means the account is fully enabled and the user
	// can authenticate.
	UserStatusActive UserStatus = "active"

	// UserStatusInvited means an invitation has been sent but the user
	// has not yet accepted it or set their credentials.
	UserStatusInvited UserStatus = "invited"

	// UserStatusDisabled means the account has been explicitly suspended
	// by an administrator. The user cannot authenticate while disabled.
	UserStatusDisabled UserStatus = "disabled"
)

// IsValid reports whether s is one of the known UserStatus constants.
func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusActive, UserStatusInvited, UserStatusDisabled:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s UserStatus) String() string { return string(s) }

// User represents a platform user scoped to a single tenant. A user may
// hold multiple Roles simultaneously; their effective permissions are the
// union of all per-role permissions in PermissionMatrix.
//
// Pointer fields (InvitedAt, DisabledAt) are nil when the corresponding
// lifecycle event has not yet occurred.
type User struct {
	// ID is the globally unique identifier for this user record.
	ID uuid.UUID

	// TenantID is the tenant this user belongs to. Users are strictly
	// scoped to one tenant and cannot access data in other tenants.
	TenantID uuid.UUID

	// Email is the user's primary identifier for authentication and
	// notifications.
	Email string

	// Name is the user's display name.
	Name string

	// Roles is the set of roles currently assigned to this user within
	// their tenant. An empty slice means the user has no permissions.
	Roles []Role

	// Status is the current lifecycle state of the account.
	Status UserStatus

	// InvitedAt is the time an invitation was issued for this user. It
	// is nil for users who were created directly without an invitation
	// flow.
	InvitedAt *time.Time

	// DisabledAt is the time the account was most recently disabled. It
	// is nil while the account is active or invited.
	DisabledAt *time.Time

	// CreatedAt is the time the user record was first created.
	CreatedAt time.Time

	// UpdatedAt is the time the user record was last modified.
	UpdatedAt time.Time
}

// HasRole reports whether the user currently holds role r.
func (u *User) HasRole(r Role) bool {
	for _, role := range u.Roles {
		if role == r {
			return true
		}
	}
	return false
}

// HasPermission reports whether any of the user's roles grant perm.
func (u *User) HasPermission(perm Permission) bool {
	for _, role := range u.Roles {
		if HasPermission(role, perm) {
			return true
		}
	}
	return false
}

// IsActive reports whether the user account is in the active state.
func (u *User) IsActive() bool { return u.Status == UserStatusActive }
