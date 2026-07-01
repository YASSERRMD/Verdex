package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TokenClaims holds the verified payload extracted from a bearer token.
// All fields are populated by Provider.ValidateToken after cryptographic
// verification succeeds.
type TokenClaims struct {
	// UserID is the subject of the token — the authenticated user's ID.
	UserID uuid.UUID

	// TenantID is the tenant the token is scoped to.
	TenantID uuid.UUID

	// Email is the user's email address at the time the token was issued.
	Email string

	// Roles is the set of roles the user held at the time the token was
	// issued. Implementations should refresh role assignments on each
	// request rather than trusting long-lived claims.
	Roles []Role

	// IssuedAt is the time the token was originally minted.
	IssuedAt time.Time

	// ExpiresAt is the time after which the token must be rejected.
	ExpiresAt time.Time

	// TokenID is an optional unique identifier for the token (JWT jti).
	// When set it may be used to support revocation via a deny-list.
	TokenID string
}

// IsExpired reports whether the claims have passed their ExpiresAt time
// using the provided now time (callers should pass time.Now()).
func (c *TokenClaims) IsExpired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

// Provider is a pluggable authentication backend. Implementations sign
// and verify bearer tokens; the core identity package never touches the
// raw signing material.
//
// Typical implementations will use JWT (HMAC or RSA/EC signatures) or
// opaque tokens validated against an external IdP. For tests use
// NoOpProvider.
type Provider interface {
	// IssueToken mints a bearer token for user and returns the raw token
	// string. The implementation chooses the expiry, signing algorithm,
	// and any additional claims.
	IssueToken(ctx context.Context, user *User) (string, error)

	// ValidateToken verifies the token string cryptographically, checks
	// expiry, and returns the extracted claims. It returns
	// ErrTokenInvalid if the token cannot be verified and ErrTokenExpired
	// if the token is syntactically valid but past its expiry.
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
}

// noOpToken is the opaque token format used by NoOpProvider.
// Format: "noop:<userID>:<tenantID>:<email>".
// This is intentionally not secure and must only be used in tests.

// NoOpProvider is a test-only Provider that issues unsigned tokens and
// validates them by parsing back the values it encoded. It has no
// cryptographic security and must never be used outside tests.
type NoOpProvider struct {
	// TTL is the duration a token is considered valid. Defaults to 1 hour
	// if zero.
	TTL time.Duration
}

// ttl returns the effective TTL, substituting the default when zero.
func (p *NoOpProvider) ttl() time.Duration {
	if p.TTL > 0 {
		return p.TTL
	}
	return time.Hour
}

// IssueToken encodes the user's fields into a plain-text token.
func (p *NoOpProvider) IssueToken(_ context.Context, user *User) (string, error) {
	if user == nil {
		return "", ErrUserNotFound
	}
	now := time.Now().UTC()
	exp := now.Add(p.ttl())
	// Encode roles as a comma-joined string.
	roles := encodeRoles(user.Roles)
	token := fmt.Sprintf("noop:%s:%s:%s:%s:%d:%d",
		user.ID.String(),
		user.TenantID.String(),
		user.Email,
		roles,
		now.Unix(),
		exp.Unix(),
	)
	return token, nil
}

// ValidateToken parses a token previously issued by IssueToken.
func (p *NoOpProvider) ValidateToken(_ context.Context, token string) (*TokenClaims, error) {
	var (
		userIDStr   string
		tenantIDStr string
		email       string
		rolesStr    string
		issuedUnix  int64
		expiresUnix int64
	)
	_, err := fmt.Sscanf(token, "noop:%36s %36s %s %s %d %d",
		&userIDStr, &tenantIDStr, &email, &rolesStr, &issuedUnix, &expiresUnix)
	if err != nil {
		// Fallback to a more permissive parse using split.
		parts := splitToken(token)
		if len(parts) != 7 || parts[0] != "noop" {
			return nil, ErrTokenInvalid
		}
		userIDStr = parts[1]
		tenantIDStr = parts[2]
		email = parts[3]
		rolesStr = parts[4]
		issuedUnix = parseUnix(parts[5])
		expiresUnix = parseUnix(parts[6])
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	claims := &TokenClaims{
		UserID:    userID,
		TenantID:  tenantID,
		Email:     email,
		Roles:     decodeRoles(rolesStr),
		IssuedAt:  time.Unix(issuedUnix, 0).UTC(),
		ExpiresAt: time.Unix(expiresUnix, 0).UTC(),
	}

	if claims.IsExpired(time.Now()) {
		return nil, ErrTokenExpired
	}
	return claims, nil
}

// splitToken splits the noop token on colons, returning up to 7 parts.
func splitToken(token string) []string {
	parts := make([]string, 0, 7)
	start := 0
	count := 0
	for i := 0; i < len(token); i++ {
		if token[i] == ':' {
			if count < 6 {
				parts = append(parts, token[start:i])
				start = i + 1
				count++
			}
		}
	}
	parts = append(parts, token[start:])
	return parts
}

// encodeRoles joins roles with a comma for embedding in the noop token.
func encodeRoles(roles []Role) string {
	if len(roles) == 0 {
		return "_"
	}
	out := ""
	for i, r := range roles {
		if i > 0 {
			out += ","
		}
		out += string(r)
	}
	return out
}

// decodeRoles splits a comma-joined roles string back into a []Role.
func decodeRoles(s string) []Role {
	if s == "_" || s == "" {
		return nil
	}
	var roles []Role
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			roles = append(roles, Role(s[start:i]))
			start = i + 1
		}
	}
	return roles
}

// parseUnix converts a decimal string to int64 without importing strconv
// (avoids a circular import risk in this small helper).
func parseUnix(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
