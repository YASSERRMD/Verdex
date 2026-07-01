package identity

import "errors"

// Sentinel errors returned by identity package functions and middleware.
// Callers use errors.Is to distinguish these from transient errors.
var (
	// ErrUnauthenticated is returned (and mapped to HTTP 401) when a
	// request carries no bearer token or the token cannot be decoded.
	ErrUnauthenticated = errors.New("identity: unauthenticated")

	// ErrForbidden is returned (and mapped to HTTP 403) when the
	// authenticated user lacks the required role or permission.
	ErrForbidden = errors.New("identity: forbidden")

	// ErrUserNotFound is returned by UserRepository and SessionStore
	// operations when the requested record does not exist.
	ErrUserNotFound = errors.New("identity: user not found")

	// ErrTokenInvalid is returned by Provider.ValidateToken when the
	// token string is syntactically malformed or the signature does not
	// verify.
	ErrTokenInvalid = errors.New("identity: token invalid")

	// ErrTokenExpired is returned by Provider.ValidateToken when the
	// token is cryptographically valid but its expiry time has passed.
	ErrTokenExpired = errors.New("identity: token expired")
)
