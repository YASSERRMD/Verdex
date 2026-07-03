package keymanagement

import (
	"time"

	"github.com/google/uuid"
)

// KeyState classifies the lifecycle stage of a key version, mirroring
// how packages/caseversioning.ArtifactKind and
// packages/notifications.Kind use a small closed string enum rather
// than a free-form status field.
type KeyState string

const (
	// KeyStateActive means this key version is the tenant's current
	// key: CurrentKey resolves to it, and it is used for all new
	// encryption operations. At most one key version per tenant is
	// Active at any time (enforced by a partial unique index — see
	// migrations/000018_create_keymanagement.up.sql).
	KeyStateActive KeyState = "active"

	// KeyStateRotating is a transient state a key version may pass
	// through while a rotation is being provisioned but not yet
	// promoted to Active (e.g. a KMS-backed Provider that
	// asynchronously generates key material). Not used by
	// LocalFileProvider or FileProvider, which rotate synchronously,
	// but reserved for a future cloud-KMS-backed Provider.
	KeyStateRotating KeyState = "rotating"

	// KeyStateRetired means this key version was previously Active but
	// has been superseded by a newer rotation. A retired key must
	// remain resolvable via Provider.Key/encryption.KeySource.Key so
	// ciphertext written while it was Active stays decryptable — see
	// Rotate's doc comment.
	KeyStateRetired KeyState = "retired"

	// KeyStateRevoked means this key version has been explicitly and
	// permanently disabled (e.g. suspected compromise). Unlike
	// Retired, a Revoked key is expected to eventually become
	// undecryptable-by-policy; this package still resolves it (Provider
	// cannot un-decrypt already-written ciphertext), but Revoke is
	// audited distinctly from an ordinary rotation and access policies
	// may choose to reject new reads of a Revoked key going forward.
	KeyStateRevoked KeyState = "revoked"
)

// allKeyStates is the exhaustive set of recognized KeyState values,
// used by IsValid.
var allKeyStates = map[KeyState]struct{}{
	KeyStateActive:   {},
	KeyStateRotating: {},
	KeyStateRetired:  {},
	KeyStateRevoked:  {},
}

// IsValid reports whether s is one of the recognized KeyState
// constants.
func (s KeyState) IsValid() bool {
	_, ok := allKeyStates[s]
	return ok
}

// String satisfies fmt.Stringer.
func (s KeyState) String() string { return string(s) }

// KeyMetadata is the persisted record describing one key version:
// identity, tenant ownership, lifecycle state, and validity window.
// The key's raw material is never a field on this type and never
// stored alongside it — KeyMetadata rows are metadata only; the
// actual key bytes live behind a Provider (provider.go), sourced from
// an offline/file-backed or env-backed store, never from Postgres in
// plaintext. See doc/key-management.md, "Metadata vs. material".
type KeyMetadata struct {
	// ID uniquely identifies this key version. Matches the ID an
	// encryption.Key/encryption.Envelope records, so a KeyMetadata row
	// can always be joined back to the ciphertext it protects.
	ID string `json:"id"`

	// TenantID is the tenant this key belongs to. Every Repository and
	// Provider method is scoped to a tenantID and refuses cross-tenant
	// access — see ErrCrossTenantAccess — implementing "per-tenant key
	// isolation" (task 4): each tenant's CurrentKey resolves to a
	// tenant-specific key, never a shared global one.
	TenantID uuid.UUID `json:"tenant_id"`

	// Version is this key's ordinal position within its tenant's
	// rotation history, starting at 1. Strictly increasing per tenant.
	Version int `json:"version"`

	// State is this key version's lifecycle stage. Required, one of
	// the KeyState constants.
	State KeyState `json:"state"`

	// CreatedAt is when this key version was generated/registered.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt, if non-nil, is when this key version's Active state
	// should be considered stale and due for rotation. Advisory only —
	// this package does not auto-rotate on expiry; see
	// Service.KeysDueForRotation for a query callers can schedule
	// against.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// WrappedKeyRef is an opaque backend-specific reference to where
	// the actual key material lives (e.g. a file path fragment under
	// FileProvider's root, or a cloud KMS key-version ARN/resource
	// name for a future backend). This package never interprets the
	// value itself — Provider implementations do. It is never the raw
	// key material.
	WrappedKeyRef string `json:"wrapped_key_ref,omitempty"`
}

// Validate checks that m has every field required to be persisted: a
// non-empty ID, non-nil TenantID, a positive Version, and a valid
// State.
func (m *KeyMetadata) Validate() error {
	if m == nil {
		return ErrNilKeyMetadata
	}
	if m.ID == "" {
		return ErrEmptyKeyID
	}
	if m.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if m.Version < 1 {
		return wrapf("Validate", ErrInvalidKeyState)
	}
	if !m.State.IsValid() {
		return ErrInvalidKeyState
	}
	return nil
}

// IsUsable reports whether m may still be used to decrypt existing
// ciphertext — true for every state except one that this package
// treats as permanently unusable. Currently all four states remain
// resolvable (see KeyStateRevoked's doc comment on why revocation does
// not retroactively break decryption), so IsUsable always returns
// true; it exists as a single, named extension point so a future
// stricter policy (e.g. "reject Revoked reads") has one place to
// change rather than scattered state comparisons.
func (m *KeyMetadata) IsUsable() bool {
	return m != nil
}

// clone returns a deep copy of m, safe to hand to a caller without
// aliasing the receiver's memory, mirroring
// packages/notifications.cloneNotification's convention.
func (m *KeyMetadata) clone() *KeyMetadata {
	if m == nil {
		return nil
	}
	cp := *m
	if m.ExpiresAt != nil {
		t := *m.ExpiresAt
		cp.ExpiresAt = &t
	}
	return &cp
}
