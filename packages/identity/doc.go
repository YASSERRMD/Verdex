// Package identity implements the identity and role-based access control
// (RBAC) layer for the Verdex judicial reasoning platform.
//
// It provides:
//
//   - Five named roles (judge, advocate, clerk, admin, auditor) and a
//     permission matrix that maps each role to the set of actions it may
//     perform (see role.go, permission.go).
//
//   - A User entity with lifecycle states (active, invited, disabled) and
//     timestamp fields (see user.go).
//
//   - A pluggable authentication Provider interface for issuing and
//     validating bearer tokens, with a NoOpProvider for testing
//     (see token.go).
//
//   - A Session abstraction and SessionStore interface for server-side
//     session management (see session.go).
//
//   - A UserRepository interface for persistent user storage (see
//     repository.go).
//
//   - Pure business-logic helpers for user lifecycle events: InviteUser,
//     DisableUser, EnableUser (see lifecycle.go).
//
//   - net/http middleware (AuthMiddleware, RequireRole, RequirePermission)
//     that validates bearer tokens and enforces RBAC (see middleware.go).
//
//   - Context helpers for reading identity from a context.Context
//     (see context.go).
//
//   - Sentinel errors for unauthenticated / forbidden / not-found / token
//     failure cases (see errors.go).
//
// The RBAC matrix is documented in doc/rbac-matrix.md.
package identity
