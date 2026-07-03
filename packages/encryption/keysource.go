package encryption

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"sync"
)

// KeyBytes is the required length, in bytes, of a Key's material for
// AES-256: 256 bits.
const KeyBytes = 32

// Key is a single symmetric encryption key, identified by an opaque
// ID. The ID is stored (in cleartext) alongside every ciphertext an
// Envelope produces with this key, so a later Decrypt call knows which
// key to ask a KeySource for -- the ID is metadata, never secret
// material itself.
type Key struct {
	// ID identifies this key. IDs are opaque strings; this package
	// imposes no format beyond "non-empty".
	ID string

	// Material is the raw symmetric key, exactly KeyBytes (32) bytes
	// long for AES-256.
	Material []byte
}

// validate checks k has a non-empty ID and correctly sized material.
func (k Key) validate() error {
	if k.ID == "" {
		return ErrEmptyKeyID
	}
	if len(k.Material) != KeyBytes {
		return fmt.Errorf("%w: key %q has %d bytes", ErrInvalidKeySize, k.ID, len(k.Material))
	}
	return nil
}

// KeySource is the extension point this phase (075) defines for Phase
// 076 (Key management & secrets) to implement for real against an
// actual KMS/secrets backend. It mirrors
// packages/guardrail.SignoffGate's shape precisely: a small interface
// with a fail-closed-in-spirit default implementation, so every other
// piece of this package -- Encrypt, Decrypt, EncryptBackup,
// DecryptBackup -- depends only on this interface and requires no
// change when Phase 076 supplies a real implementation.
//
// A KeySource must support two operations because of key rotation:
// CurrentKey is used for every new encryption (so newly-written data
// always uses the latest key), while Key(ctx, keyID) resolves any
// specific historical key ID so old ciphertext -- encrypted under a
// since-rotated-away key -- can still be decrypted. An implementation
// that forgets old keys after rotation breaks decryption of everything
// encrypted before the rotation; see Rotator's doc comment for the
// rotation contract this package expects.
type KeySource interface {
	// CurrentKey returns the key that should be used for any new
	// encryption operation.
	CurrentKey(ctx context.Context) (Key, error)

	// Key returns the key identified by keyID, for decrypting an
	// Envelope that recorded that ID. It must continue to resolve any
	// key ID that was ever returned by CurrentKey, even after
	// rotation, or old ciphertext becomes permanently undecryptable.
	Key(ctx context.Context, keyID string) (Key, error)
}

// Rotator is an optional capability a KeySource may additionally
// implement to support explicit key rotation. LocalKeySource
// implements it; a future KMS-backed KeySource (Phase 076) may
// instead rotate keys out-of-band (e.g. the KMS itself issues a new
// key version) and need not implement Rotator at all -- callers that
// need rotation should type-assert for it rather than assume every
// KeySource supports it.
type Rotator interface {
	// Rotate generates (or registers) a new current key and returns
	// its ID. After Rotate returns, CurrentKey must return the new
	// key, while the previous current key (and every key before it)
	// must remain resolvable via Key for decrypting old envelopes.
	Rotate(ctx context.Context) (string, error)
}

// LocalKeySource is the provisional default KeySource implementation
// used until Phase 076 supplies a real KMS integration. It resolves
// keys from environment variables or an optional local file, and
// keeps every key it has ever issued in memory so old ciphertext
// remains decryptable across rotations within the process's lifetime.
//
// This is explicitly NOT a production key-management solution: keys
// held in process memory (sourced from an env var or a local file) are
// only as protected as the host process and its environment, there is
// no hardware-backed key protection, and rotated-away keys are lost on
// restart unless persisted to the (also unencrypted) local file. It
// exists so packages/encryption is fully usable and testable today;
// Phase 076 (Key management & secrets) is expected to replace it with
// a real KMS-backed KeySource implementing the same interface, at
// which point no caller of Encrypt/Decrypt needs to change.
type LocalKeySource struct {
	mu         sync.RWMutex
	keys       map[string]Key
	currentID  string
	generation int
}

// NewLocalKeySource constructs a LocalKeySource seeded with a single
// initial key. If initial.ID is empty, an ID is generated
// automatically. initial.Material must be exactly KeyBytes long.
func NewLocalKeySource(initial Key) (*LocalKeySource, error) {
	if initial.ID == "" {
		initial.ID = "local-key-1"
	}
	if err := initial.validate(); err != nil {
		return nil, err
	}
	return &LocalKeySource{
		keys:       map[string]Key{initial.ID: initial},
		currentID:  initial.ID,
		generation: 1,
	}, nil
}

// NewLocalKeySourceFromEnv builds a LocalKeySource from an environment
// variable named envVar, whose value must be a base64- or hex-free raw
// string of at least KeyBytes bytes (it is truncated/derived
// internally, see deriveKey). This is a provisional, development-only
// convenience: it exists so a deployment can supply
// VERDEX_ENCRYPTION_KEY (or similar) without any code change, not as a
// substitute for real KMS-issued keys.
func NewLocalKeySourceFromEnv(envVar string) (*LocalKeySource, error) {
	raw, ok := os.LookupEnv(envVar)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("encryption: environment variable %s is not set", envVar)
	}
	material := deriveKey(raw)
	return NewLocalKeySource(Key{ID: "env-key-1", Material: material})
}

// deriveKey stretches an arbitrary-length secret string into exactly
// KeyBytes bytes using SHA-256. This is a deterministic KDF-lite, not
// a substitute for a real key-derivation function with a salt/context
// -- adequate for the provisional local key source, not for a
// production KMS.
func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// CurrentKey implements KeySource.
func (s *LocalKeySource) CurrentKey(_ context.Context) (Key, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.keys[s.currentID]
	if !ok {
		return Key{}, fmt.Errorf("encryption: local key source has no current key: %w", ErrKeyNotFound)
	}
	return k, nil
}

// Key implements KeySource.
func (s *LocalKeySource) Key(_ context.Context, keyID string) (Key, error) {
	if keyID == "" {
		return Key{}, ErrEmptyKeyID
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.keys[keyID]
	if !ok {
		return Key{}, fmt.Errorf("encryption: key %q: %w", keyID, ErrKeyNotFound)
	}
	return k, nil
}

// Rotate implements Rotator. It generates a fresh random 32-byte key,
// registers it under a new generated ID, and makes it the current
// key. All previously issued keys remain resolvable via Key.
func (s *LocalKeySource) Rotate(_ context.Context) (string, error) {
	material := make([]byte, KeyBytes)
	if _, err := rand.Read(material); err != nil {
		return "", fmt.Errorf("encryption: rotate: generate key material: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	newID := fmt.Sprintf("local-key-%d", s.generation)
	s.keys[newID] = Key{ID: newID, Material: material}
	s.currentID = newID
	return newID, nil
}

// KeyCount reports how many distinct keys this LocalKeySource has
// issued (current plus all still-resolvable historical keys). Mainly
// useful for tests asserting rotation behavior.
func (s *LocalKeySource) KeyCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.keys)
}
