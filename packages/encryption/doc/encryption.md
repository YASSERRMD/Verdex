# Encryption

`packages/encryption` implements Phase 075: encryption at rest and in
transit. It is the first phase of Part 7 (Security, Compliance &
Sovereignty) and defines Verdex's core cryptographic primitives and
enforcement points — TLS configuration, an encryption-at-rest
declaration check, authenticated field-level encryption, a key-source
extension point, key rotation, encrypted backups, cipher policy
config, and a sensitive-field plaintext audit.

## Transport encryption: `RequireTLS`, `ValidateTLSConfig`

```go
func RequireTLS(opts ...TLSOption) *tls.Config
func ValidateTLSConfig(cfg *tls.Config) error
```

`RequireTLS` builds a real `*tls.Config`: `MinVersion` defaults to
`MinSupportedTLSVersion` (TLS 1.2), and `CipherSuites` is restricted to
a forward-secret, AEAD-only list (ECDHE key exchange, AES-GCM or
ChaCha20-Poly1305) for connections that negotiate down to TLS 1.2. TLS
1.3's cipher suites are fixed by the Go standard library and are
already AEAD/forward-secret, so they are never restricted further.
`WithMinVersion` lets a caller request a *stronger* floor (e.g. TLS
1.3-only) but clamps any weaker request back up to
`MinSupportedTLSVersion` — there is no way to use `RequireTLS` to
produce a `*tls.Config` that accepts TLS 1.0 or 1.1.

`ValidateTLSConfig` is the inverse check: given an arbitrary
`*tls.Config` (e.g. one constructed elsewhere, or loaded from a
different code path), it verifies the same floor — minimum version,
`InsecureSkipVerify` never set, and (for TLS-1.2-targeting configs) an
approved cipher suite list.

Every Verdex HTTP server or client, and `packages/persistence`'s
Postgres connection, is expected to build its TLS configuration
through `RequireTLS` rather than assembling one by hand, so the
enforced floor lives in exactly one place.

## Encryption at rest: `AssertEncryptedAtRest`

True disk/volume-level encryption at rest is an infrastructure and
deployment concern — an encrypted EBS volume, a managed Postgres
instance with storage encryption enabled — and this package has no way
to reach past the wire and turn it on. What it provides instead is a
fail-closed startup declaration check:

```go
type AtRestConfig struct {
    EncryptedAtRest     bool
    RequireTLSInTransit bool
}

func AssertEncryptedAtRest(cfg AtRestConfig, dsn string) error
```

Unless a deployment explicitly sets `EncryptedAtRest: true`,
`AssertEncryptedAtRest` fails — there is no "unset means assume it's
fine" path. If `RequireTLSInTransit` is also set, the DSN itself is
parsed (supporting both `postgres://...?sslmode=...` URL DSNs and
`key=value` keyword DSNs) and rejected if it requests `sslmode=disable`
or `sslmode=allow`, or specifies no `sslmode` at all.

`packages/persistence.Open` wires this in behind a new
`DatabaseConfig.RequireTLS` flag (defaulting to `false`, so existing
local-development and testcontainer DSNs using `sslmode=disable`
continue to work unmodified): when a deployment profile sets
`database.require_tls: true`, `Open` calls `AssertEncryptedAtRest`
before ever dialing the database.

## Field-level encryption: `Encrypt`, `Decrypt`, `Envelope`

```go
func Encrypt(ctx context.Context, source KeySource, plaintext []byte) ([]byte, error)
func Decrypt(ctx context.Context, source KeySource, envelope []byte) ([]byte, error)
```

Both use AES-256-GCM (authenticated encryption — tamper-evident, not
just confidentiality) with a fresh random nonce generated on every
`Encrypt` call, so encrypting the same plaintext twice never produces
identical ciphertext. The result is a versioned `Envelope`:

```
magic "VDXE" (4B) | version (1B) | key ID length (2B) | key ID |
nonce length (1B) | nonce | ciphertext (with GCM auth tag)
```

Recording the key ID in cleartext alongside the ciphertext (the ID is
an identifier, never secret material) is what makes key rotation
non-breaking: `Decrypt` always knows exactly which historical key to
ask the `KeySource` for, regardless of which key is current at decrypt
time. `ParseEnvelope` exposes these fields without decrypting, for
audit tooling that needs to know which key protects a given record.

`Decrypt` returns `ErrAuthenticationFailed` (wrapping GCM's own
authentication error) for tampered ciphertext or a wrong key — this is
a real security property, not just a parse error, and is covered by
`TestDecrypt_TamperedCiphertextFailsAuthentication`.

## Encrypted backups: `EncryptBackup`, `DecryptBackup`

```go
func EncryptBackup(ctx context.Context, source KeySource, backup []byte) ([]byte, error)
func DecryptBackup(ctx context.Context, source KeySource, envelope []byte) ([]byte, error)
```

Same mechanism as `Encrypt`/`Decrypt` (in fact, they call straight
through), kept as distinct entry points purely for call-site clarity:
"this is a whole backup artifact" reads differently than "this is one
sensitive field," even though both produce and consume the identical
`Envelope` wire format — `Decrypt` can read `EncryptBackup`'s output
and vice versa.

## Key management (provisional): `KeySource`

```go
type KeySource interface {
    CurrentKey(ctx context.Context) (Key, error)
    Key(ctx context.Context, keyID string) (Key, error)
}

type Rotator interface {
    Rotate(ctx context.Context) (string, error)
}
```

**`KeySource` is this phase's forward-looking extension point for
Phase 076 (Key management & secrets)**, mirroring
`packages/guardrail.SignoffGate`'s pattern precisely:
`guardrail.CanFinalize` accepted a `SignoffGate` interface starting at
Phase 068's predecessor state, with `NoSignoffRecordedGate` as the only
(fail-closed) implementation until Phase 068 supplies a real one — no
change to `guardrail`'s own gating logic was required when that
happened. The same shape applies here: `Encrypt`/`Decrypt` depend only
on the `KeySource` interface, so once Phase 076 lands a real
KMS-backed `KeySource` implementation, no change to `field.go`,
`backup.go`, or `envelope.go` is required.

`LocalKeySource` is the only implementation this phase provides. It
resolves keys either from an explicit `Key{ID, Material}` or from an
environment variable (`NewLocalKeySourceFromEnv`, which stretches an
arbitrary-length secret to 32 bytes via SHA-256), keeps every key it
has ever issued resolvable in memory, and implements `Rotator` by
generating a fresh random 32-byte key and making it current while
every prior key remains resolvable for decrypting old ciphertext. It
is **explicitly provisional**: keys held in process memory, sourced
from an env var or unencrypted local file, have no hardware-backed
protection and are lost across restarts unless persisted elsewhere.
Phase 076 is expected to replace or wrap it with a real KMS
integration implementing the same `KeySource` interface.

### Rotation contract, proven by test

`TestLocalKeySource_RotateKeepsOldKeyDecryptable` proves the exact
sequence the phase task list requires: encrypt with key v1, call
`Rotate`, decrypt the v1 ciphertext (still succeeds, using the
recorded key ID), encrypt again (uses the new, now-current key). A
`KeySource` implementation that "forgets" old keys after rotation
would permanently break decryption of everything encrypted before the
rotation — this is why `Key(ctx, keyID)` is a distinct method from
`CurrentKey`, not an accident of interface design.

## Cipher policy: `CipherPolicy`

```go
type CipherPolicy struct {
    Algorithm     string // "AES-256-GCM"
    KeySizeBytes  int    // 32
    MinTLSVersion uint16 // tls.VersionTLS12 or tls.VersionTLS13
}

func DefaultCipherPolicy() CipherPolicy
func (p CipherPolicy) Validate() error
```

`CipherPolicy` is a plain config struct intended to be embedded in a
service's own config type and loaded through `packages/config`'s
existing `Loader`/profile-overlay layering (see
`packages/config/profile.go`) — this package does not reimplement
config loading, only the schema and its `Validate` method. A
deployment can declare (and have validated) a stricter floor — e.g.
`min_tls_version: TLS1.3` — through the same profile mechanism used
for every other config section, without a Go code change.

## Sensitive-field registry: `//encrypted`, `ScanForPlaintext`

```go
type Party struct {
    Name       string
    NationalID string `encrypted:"true"`
}

findings, err := encryption.ScanForPlaintext(&party)
```

The `encrypted:"true"` struct tag is Verdex's convention (referenced
throughout this package as "the `//encrypted` convention") for marking
a field that must hold `Envelope`-encrypted ciphertext at rest,
consistent with `packages/config`'s and `packages/observability`'s
existing `redact:"true"` tag pattern for a different concern (log
redaction, not storage encryption — see `packages/observability`'s
`redact.go` doc comment on that distinction).

`ScanForPlaintext` walks a struct (recursing into nested structs and
non-nil struct pointers) and reports every tagged `string`/`[]byte`
field whose current value does not parse as a valid `Envelope` — i.e.
still looks like plaintext. This is a lint-style audit, not a
guarantee: like `packages/observability`'s redaction being "a
defense-in-depth safety net, not a substitute for simply not logging
PII in the first place," `ScanForPlaintext` catches the obvious
mistake (a sensitive field that was never run through `Encrypt`) but
cannot prove a field's *history* was always encrypted, only its
*current* value's shape.

`packages/pii`'s PII-flagged fields are a natural pairing for this
tag — a field the PII detector flags as sensitive is a good candidate
for `encrypted:"true"` in whatever struct ultimately stores it — but
this package does not require or assume that wiring; `ScanForPlaintext`
works against any struct that uses the tag, independent of `pii`.

## What this package deliberately does not do

- **It does not implement Phase 076's KMS integration.** `KeySource`
  is the seam; `LocalKeySource` is an explicitly provisional
  placeholder, not a production key-management solution.
- **It does not configure actual disk/volume encryption.**
  `AssertEncryptedAtRest` verifies a *declaration*, not the underlying
  infrastructure — an operator who sets `EncryptedAtRest: true`
  without actually enabling storage encryption at the infra layer will
  not be caught by this check. That gap is inherent to app code never
  being able to observe infrastructure-layer disk encryption directly.
- **It does not require every sensitive field in the codebase to be
  tagged today.** The `encrypted:"true"` convention and
  `ScanForPlaintext` are available for any package that wants to adopt
  them; this phase does not retrofit every existing struct across
  Verdex.
- **It does not replace `packages/observability`'s redaction.** Log
  redaction (masking a value before it reaches a log line) and storage
  encryption (this package) are related but distinct concerns — a
  value can be correctly redacted from logs while still stored in
  plaintext, or correctly encrypted at rest while still leaking into a
  log line unredacted. Both mechanisms are needed; neither subsumes
  the other.
