package keymanagement

import "context"

// KeyMaterial is the raw symmetric key bytes for one key version,
// paired with its metadata. This is the boundary type between
// Provider (which touches real key bytes) and Repository/KeyMetadata
// (which never do) — see types.go, "Metadata vs. material".
type KeyMaterial struct {
	// Metadata describes this key version.
	Metadata KeyMetadata

	// Material is the raw symmetric key, exactly
	// encryption.KeyBytes (32) bytes long for AES-256. Never logged,
	// never persisted to Postgres — see doc/key-management.md.
	Material []byte
}

// Provider is the pluggable secrets/KMS backend abstraction this
// package defines (task 1), mirroring packages/provider.LLMProvider's
// model-agnostic, no-hardcoded-backend convention exactly: business
// logic (Service, Adapter) depends only on this interface, never on a
// concrete backend, so a deployment can swap FileProvider for a real
// cloud KMS implementation without touching a single call site.
//
// Every method is scoped by tenantID, implementing per-tenant key
// isolation (task 4): a Provider implementation must never resolve
// one tenant's CurrentKey/Key call using another tenant's key
// material.
//
// # Implementing a cloud KMS backend
//
// A future cloud-KMS-backed Provider is expected to:
//   - map CurrentKey/Rotate to that KMS's notion of a key's "primary"
//     or "current" version for a tenant-scoped key resource (e.g. one
//     KMS key per tenant, or a per-tenant key-alias convention);
//   - map Key(ctx, tenantID, keyID) to fetching (or having the KMS
//     directly encrypt/decrypt with) that specific key version, so
//     historical ciphertext stays decryptable across rotations exactly
//     as LocalKeySource and FileProvider already guarantee;
//   - perform its own network calls and authentication entirely
//     inside this interface's methods — Service, Adapter, and every
//     other caller in this package remain backend-agnostic;
//   - NOT be registered or imported by this package itself, keeping
//     with the "no hardcoded provider" convention packages/provider
//     established for LLM adapters. A deployment wires its chosen
//     Provider at startup (see doc/key-management.md).
type Provider interface {
	// CurrentKey returns the tenant's current key material — the key
	// that should be used for any new encryption operation. Returns
	// ErrNoActiveKey (wrapped) if the tenant has no Active key yet.
	CurrentKey(ctx context.Context, tenantID string) (KeyMaterial, error)

	// Key returns the key material for keyID, scoped to tenantID, for
	// decrypting an Envelope that recorded that ID. Must continue to
	// resolve any key ID this Provider ever returned from CurrentKey,
	// even after rotation — see Rotate's doc comment.
	Key(ctx context.Context, tenantID, keyID string) (KeyMaterial, error)

	// Rotate generates (or registers) a new current key for tenantID
	// and returns its KeyMetadata. After Rotate returns, CurrentKey
	// must return the new key, while the previous current key (and
	// every key before it) must remain resolvable via Key — matching
	// packages/encryption.Rotator's contract exactly, since Adapter
	// (adapter.go) implements encryption.Rotator by delegating here.
	Rotate(ctx context.Context, tenantID string) (KeyMetadata, error)
}
