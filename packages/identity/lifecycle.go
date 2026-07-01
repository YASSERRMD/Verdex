package identity

import (
	"errors"
	"time"
)

// LifecycleEvent describes a state transition that happened to a User.
// Callers may consume these events to publish domain events, write audit
// log entries, or trigger notifications.
type LifecycleEvent struct {
	// Kind is a short machine-readable identifier for the transition.
	Kind string

	// User is the updated User value after the transition has been
	// applied. The caller should persist this updated value.
	User *User
}

// ErrAlreadyActive is returned by EnableUser when the user is already in
// the active state.
var ErrAlreadyActive = errors.New("identity: user is already active")

// ErrAlreadyDisabled is returned by DisableUser when the user is already
// disabled.
var ErrAlreadyDisabled = errors.New("identity: user is already disabled")

// InviteUser transitions u to the invited state, records the invitation
// timestamp, and returns a LifecycleEvent describing what happened.
//
// InviteUser is a pure function: it does not perform I/O. The caller is
// responsible for persisting the updated User (e.g. via UserRepository)
// and acting on the returned event (e.g. sending an invitation email).
//
// InviteUser does not validate the email address format; that check
// belongs in the calling layer.
func InviteUser(u *User, roles []Role, now time.Time) (*User, LifecycleEvent, error) {
	if u == nil {
		return nil, LifecycleEvent{}, ErrUserNotFound
	}

	updated := copyUser(u)
	updated.Status = UserStatusInvited
	updated.Roles = roles
	t := now.UTC()
	updated.InvitedAt = &t
	updated.DisabledAt = nil
	updated.UpdatedAt = now.UTC()

	return updated, LifecycleEvent{Kind: "user.invited", User: updated}, nil
}

// DisableUser transitions u to the disabled state, recording the time
// the account was suspended. The user will not be able to authenticate
// after this transition is persisted.
//
// DisableUser is a pure function and does not perform I/O. It returns
// ErrAlreadyDisabled if u is already disabled.
func DisableUser(u *User, now time.Time) (*User, LifecycleEvent, error) {
	if u == nil {
		return nil, LifecycleEvent{}, ErrUserNotFound
	}
	if u.Status == UserStatusDisabled {
		return nil, LifecycleEvent{}, ErrAlreadyDisabled
	}

	updated := copyUser(u)
	updated.Status = UserStatusDisabled
	t := now.UTC()
	updated.DisabledAt = &t
	updated.UpdatedAt = now.UTC()

	return updated, LifecycleEvent{Kind: "user.disabled", User: updated}, nil
}

// EnableUser transitions u from disabled or invited back to the active
// state, clearing the DisabledAt timestamp.
//
// EnableUser is a pure function and does not perform I/O. It returns
// ErrAlreadyActive if u is already active.
func EnableUser(u *User, now time.Time) (*User, LifecycleEvent, error) {
	if u == nil {
		return nil, LifecycleEvent{}, ErrUserNotFound
	}
	if u.Status == UserStatusActive {
		return nil, LifecycleEvent{}, ErrAlreadyActive
	}

	updated := copyUser(u)
	updated.Status = UserStatusActive
	updated.DisabledAt = nil
	updated.UpdatedAt = now.UTC()

	return updated, LifecycleEvent{Kind: "user.enabled", User: updated}, nil
}

// copyUser returns a shallow copy of u with the Roles slice independently
// allocated so mutations to the returned copy's Roles do not affect the
// original.
func copyUser(u *User) *User {
	cp := *u
	if u.Roles != nil {
		cp.Roles = make([]Role, len(u.Roles))
		copy(cp.Roles, u.Roles)
	}
	return &cp
}
