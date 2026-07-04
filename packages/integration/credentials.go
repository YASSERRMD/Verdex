package integration

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CredentialKind classifies the authentication shape a
// ConnectorCredentials record describes, mirroring
// packages/keymanagement.KeyState's small-closed-string-enum
// convention.
type CredentialKind string

const (
	// CredentialKindAPIKey describes a single bearer API key.
	CredentialKindAPIKey CredentialKind = "api_key"

	// CredentialKindOAuthClientCredentials describes an OAuth 2.0
	// client-credentials grant (client ID + client secret exchanged for
	// a bearer token).
	CredentialKindOAuthClientCredentials CredentialKind = "oauth_client_credentials"

	// CredentialKindMutualTLS describes a mutual-TLS client certificate
	// used to authenticate the connection itself, independent of any
	// application-level token.
	CredentialKindMutualTLS CredentialKind = "mutual_tls"

	// CredentialKindNone describes a connector that requires no
	// authentication at all (e.g. SandboxConnector).
	CredentialKindNone CredentialKind = "none"
)

// IsValid reports whether k is one of the named CredentialKind
// constants.
func (k CredentialKind) IsValid() bool {
	switch k {
	case CredentialKindAPIKey, CredentialKindOAuthClientCredentials, CredentialKindMutualTLS, CredentialKindNone:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (k CredentialKind) String() string { return string(k) }

// ConnectorCredentials is a tenant-scoped record describing how one
// ConnectorConfig authenticates to its external system (task 5). It
// deliberately never carries raw secret material -- only a
// SecretRef/handle naming where the actual key/certificate/client
// secret lives in packages/keymanagement or packages/encryption
// (referenced by tag/ID only, never imported, exactly as
// packages/compliance.Control.MappedTo references platform features
// by string tag without importing the tagged packages). Resolving
// SecretRef to real secret bytes is the caller's responsibility
// (typically a keymanagement.Provider lookup at call time), not
// something this package or type ever does.
type ConnectorCredentials struct {
	// ID uniquely identifies this credentials record.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this record to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// Kind names the authentication shape (API key, OAuth client
	// credentials, mutual TLS, or none).
	Kind CredentialKind `json:"kind"`

	// SecretRef is an opaque handle/reference to where the actual
	// secret material is stored -- e.g. a
	// packages/keymanagement.KeyMetadata.ID string, an
	// packages/encryption key handle, or an external secrets-manager
	// path. Required unless Kind is CredentialKindNone. This package
	// never stores or transmits the referenced secret's raw bytes.
	SecretRef string `json:"secret_ref,omitempty"`

	// ClientID is the public, non-secret identifier associated with
	// this credential set (an API key's key-ID prefix, an OAuth client
	// ID). Safe to store and log; unlike SecretRef's target, this
	// value alone does not authenticate a call.
	ClientID string `json:"client_id,omitempty"`

	// TokenURL is the OAuth token endpoint used to exchange the
	// referenced client secret for a bearer token, when Kind is
	// CredentialKindOAuthClientCredentials. Empty for other kinds.
	TokenURL string `json:"token_url,omitempty"`

	// Scopes lists the OAuth scopes requested, when Kind is
	// CredentialKindOAuthClientCredentials.
	Scopes []string `json:"scopes,omitempty"`

	// LastVerifiedAt records when Validate last confirmed this
	// credential set is structurally usable and (if VerifyFunc was
	// supplied) accepted by the external system. Zero means never
	// verified.
	LastVerifiedAt time.Time `json:"last_verified_at,omitempty"`

	// CreatedBy is the identity.User who registered this credential
	// set.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks c for structural well-formedness: a valid Kind, a
// non-nil TenantID, and (unless Kind is CredentialKindNone) a non-blank
// SecretRef so this record can never silently mean "authenticated with
// nothing". OAuth client-credentials additionally require a non-blank
// ClientID and TokenURL.
func (c *ConnectorCredentials) Validate() error {
	if c == nil {
		return ErrInvalidCredentials
	}
	if c.TenantID == uuid.Nil {
		return wrapf("ConnectorCredentials.Validate", ErrEmptyTenantID)
	}
	if !c.Kind.IsValid() {
		return wrapf("ConnectorCredentials.Validate", ErrInvalidCredentials)
	}
	if c.Kind != CredentialKindNone && strings.TrimSpace(c.SecretRef) == "" {
		return wrapf("ConnectorCredentials.Validate", ErrInvalidCredentials)
	}
	if c.Kind == CredentialKindOAuthClientCredentials {
		if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.TokenURL) == "" {
			return wrapf("ConnectorCredentials.Validate", ErrInvalidCredentials)
		}
	}
	return nil
}

// VerifyFunc resolves and checks a ConnectorCredentials record's
// referenced secret against the external system, returning a non-nil
// error if the secret is missing, expired, or rejected. Callers
// typically implement this by looking up SecretRef through
// packages/keymanagement.Provider.Key (or an equivalent
// packages/encryption lookup) and then issuing Connector.Ping. A nil
// VerifyFunc skips the live check and AuthorizeCredentials only
// performs structural Validate.
type VerifyFunc func(ctx context.Context, creds ConnectorCredentials) error

// AuthorizeCredentials validates creds structurally and, if verify is
// non-nil, invokes verify to confirm the referenced secret is actually
// accepted by the external system -- the "per-connector auth
// validation before any call" task 5 requires. On success it returns a
// copy of creds with LastVerifiedAt set to now. Callers should call
// this before the first Connector call of a session and periodically
// thereafter (e.g. before each ImportRun), not on every single
// ImportCases/DeliverReport invocation.
func AuthorizeCredentials(ctx context.Context, creds ConnectorCredentials, verify VerifyFunc, now time.Time) (ConnectorCredentials, error) {
	if err := creds.Validate(); err != nil {
		return ConnectorCredentials{}, err
	}
	if verify != nil {
		if err := verify(ctx, creds); err != nil {
			return ConnectorCredentials{}, wrapf("AuthorizeCredentials", err)
		}
	}
	out := creds
	out.LastVerifiedAt = now.UTC()
	return out, nil
}
