package encryption

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilKeySource is returned when a function requiring a KeySource
	// is called with a nil one.
	ErrNilKeySource = errors.New("encryption: key source is required")

	// ErrEmptyPlaintext is returned by Encrypt/EncryptBackup when called
	// with a nil or empty plaintext.
	ErrEmptyPlaintext = errors.New("encryption: plaintext is required")

	// ErrEmptyCiphertext is returned by Decrypt/DecryptBackup when
	// called with a nil or empty ciphertext.
	ErrEmptyCiphertext = errors.New("encryption: ciphertext is required")

	// ErrInvalidEnvelope is returned when a ciphertext cannot be parsed
	// as a valid versioned Envelope (wrong magic, truncated, or
	// malformed).
	ErrInvalidEnvelope = errors.New("encryption: invalid envelope")

	// ErrUnsupportedEnvelopeVersion is returned when an Envelope
	// declares a version this package does not know how to decode.
	ErrUnsupportedEnvelopeVersion = errors.New("encryption: unsupported envelope version")

	// ErrKeyNotFound is returned by a KeySource when the requested key
	// ID has no resolvable key.
	ErrKeyNotFound = errors.New("encryption: key not found")

	// ErrAuthenticationFailed is returned by Decrypt/DecryptBackup when
	// GCM authentication fails -- the ciphertext was tampered with, or
	// decrypted with the wrong key.
	ErrAuthenticationFailed = errors.New("encryption: authentication failed")

	// ErrInvalidKeySize is returned when a Key's material is not
	// exactly 32 bytes (AES-256).
	ErrInvalidKeySize = errors.New("encryption: key must be 32 bytes for AES-256")

	// ErrEmptyKeyID is returned when a Key is constructed or looked up
	// with an empty ID.
	ErrEmptyKeyID = errors.New("encryption: key id is required")

	// ErrNotEncryptedAtRest is returned by AssertEncryptedAtRest when
	// the deployment has not declared its backing store is
	// encrypted-at-rest.
	ErrNotEncryptedAtRest = errors.New("encryption: backing store is not declared encrypted-at-rest")

	// ErrInvalidCipherPolicy is returned by CipherPolicy.Validate for a
	// policy with a missing or unsupported field.
	ErrInvalidCipherPolicy = errors.New("encryption: invalid cipher policy")

	// ErrPlaintextLeak is returned by ScanForPlaintext when a
	// //encrypted-tagged field's value does not look like a valid
	// Envelope -- i.e. it appears to still hold plaintext.
	ErrPlaintextLeak = errors.New("encryption: sensitive field holds unencrypted value")
)
