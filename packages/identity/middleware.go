package identity

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AuthMiddleware returns an http.Handler middleware that:
//
//  1. Extracts a Bearer token from the Authorization header.
//  2. Validates the token via provider.ValidateToken.
//  3. Constructs a minimal User from the claims and stores it on the
//     request context via WithUser so downstream handlers can call
//     UserFromContext.
//
// If no Authorization header is present, or the header value is not a
// valid "Bearer <token>" pair, the middleware responds with 401
// Unauthorized and does not call next.
//
// If the token fails validation (ErrTokenInvalid or ErrTokenExpired),
// the middleware responds with 401 Unauthorized.
//
// The store parameter is optional (may be nil). When non-nil the
// middleware uses it to look up a live session and merge the
// server-authoritative role list into the context user, which guards
// against stale roles cached in long-lived tokens.
func AuthMiddleware(provider Provider, store SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := extractBearerToken(r)
			if !ok {
				http.Error(w, ErrUnauthenticated.Error(), http.StatusUnauthorized)
				return
			}

			claims, err := provider.ValidateToken(r.Context(), token)
			if err != nil {
				switch {
				case errors.Is(err, ErrTokenExpired):
					http.Error(w, ErrTokenExpired.Error(), http.StatusUnauthorized)
				default:
					http.Error(w, ErrUnauthenticated.Error(), http.StatusUnauthorized)
				}
				return
			}

			user := &User{
				ID:       claims.UserID,
				TenantID: claims.TenantID,
				Email:    claims.Email,
				Roles:    claims.Roles,
				Status:   UserStatusActive,
			}

			// When a SessionStore is provided, attempt to refresh the
			// role list from the live session. On any error we continue
			// with the token-derived roles rather than rejecting the
			// request — the token was valid, so we degrade gracefully.
			// TokenID doubles as the session ID; an empty or unparseable
			// TokenID simply skips the lookup.
			if store != nil {
				if sessionID, parseErr := uuid.Parse(claims.TokenID); parseErr == nil {
					if session, getErr := store.Get(r.Context(), sessionID); getErr == nil {
						user.Roles = session.Roles
					}
				}
			}

			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that passes the request to next only if
// the authenticated user (stored by AuthMiddleware) holds at least one
// of the supplied roles. When the user has none of the required roles
// the middleware responds with 403 Forbidden.
//
// RequireRole must be composed inside AuthMiddleware (i.e. applied after
// AuthMiddleware in the handler chain):
//
//	mux.Handle("/cases", AuthMiddleware(p, s)(RequireRole(RoleJudge, RoleClerk)(handler)))
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, ErrUnauthenticated.Error(), http.StatusUnauthorized)
				return
			}

			for _, required := range roles {
				if user.HasRole(required) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, ErrForbidden.Error(), http.StatusForbidden)
		})
	}
}

// RequirePermission returns middleware that passes the request to next
// only if the authenticated user holds at least one role that grants all
// of the supplied permissions. When the user lacks any required permission
// the middleware responds with 403 Forbidden.
//
// RequirePermission must be composed inside AuthMiddleware, just like
// RequireRole.
func RequirePermission(perms ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, ErrUnauthenticated.Error(), http.StatusUnauthorized)
				return
			}

			for _, perm := range perms {
				if !user.HasPermission(perm) {
					http.Error(w, ErrForbidden.Error(), http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken reads the Authorization header and returns the
// token string if the scheme is "Bearer". It returns ("", false) for any
// malformed or absent header.
func extractBearerToken(r *http.Request) (string, bool) {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return "", false
	}
	parts := strings.SplitN(hdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}
