// Package encryption provides the primitives and enforcement points
// Verdex uses to keep stored and transmitted data encrypted: TLS
// configuration for service traffic, an encryption-at-rest assertion
// for the database connection, authenticated field-level encryption
// for sensitive values, and a versioned envelope format that survives
// key rotation.
//
// # Transport encryption
//
// RequireTLS (tls.go) builds a *tls.Config enforcing a minimum TLS
// version and a restricted, modern cipher suite list. Any Verdex HTTP
// server or client — and packages/persistence's Postgres connection —
// is expected to construct its *tls.Config through RequireTLS rather
// than hand-rolling one, so every service shares one enforced floor.
//
// # Encryption at rest
//
// True disk-level encryption at rest is an infrastructure/deployment
// concern (e.g. an encrypted EBS volume or a Postgres instance with
// storage-level encryption enabled) — this package cannot reach past
// the wire to configure it. What it can and does provide is
// AssertEncryptedAtRest (atrest.go): a startup-time check that the
// deployment has explicitly declared its backing store is
// encrypted-at-rest, so an operator who has NOT made that declaration
// gets a loud failure instead of a silent assumption.
//
// # Field-level encryption
//
// Encrypt and Decrypt (field.go) provide authenticated (AES-256-GCM)
// encryption for individual sensitive values -- a password, a national
// ID, a document body destined for a `//encrypted`-tagged struct
// field. Every call generates a fresh random nonce; ciphertexts are
// wrapped in a versioned Envelope (envelope.go) that records which key
// ID encrypted them, so callers never need to track key versions
// themselves and key rotation never breaks old ciphertext.
//
// EncryptBackup and DecryptBackup (backup.go) wrap the same envelope
// format for opaque backup blobs, kept as distinct entry points from
// Encrypt/Decrypt for call-site clarity even though the underlying
// mechanism is identical.
//
// # Key management (provisional)
//
// KeySource (keysource.go) is the extension point this phase defines
// for Phase 076 (Key management & secrets) to implement for real. See
// keysource.go's doc comment for the full rationale; in short, it
// mirrors packages/guardrail.SignoffGate's "extension-point interface,
// fail-closed default" idiom exactly. LocalKeySource, the only
// implementation provided here, resolves keys from environment
// variables or a local file and is explicitly provisional: it is
// intended for development and single-node deployments only, and
// Phase 076 is expected to replace or wrap it with a real KMS
// integration without requiring changes to this package's Encrypt,
// Decrypt, or Envelope logic.
//
// # Cipher policy
//
// CipherPolicy (policy.go) is a validated config struct -- algorithm,
// key size, minimum TLS version -- wired through packages/config's
// profile pattern, so a deployment can declare (and have validated)
// its required cryptographic floor without editing Go code.
//
// # Sensitive-field registry
//
// SensitiveField and the //encrypted convention (registry.go) let a
// caller declare, via a small registry or struct tag, which fields of
// a type must never be stored in plaintext. ScanForPlaintext audits a
// struct value against that registry and reports any tagged field
// whose current value does not look like an Envelope-wrapped
// ciphertext -- a lint-style backstop, not a guarantee, in the same
// spirit as packages/observability's redaction being "defense in
// depth, not a substitute for not logging PII in the first place."
//
// See doc/encryption.md for the full design writeup, including the
// Phase 076 extension point.
package encryption
