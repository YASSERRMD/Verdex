package encryption

import (
	"crypto/tls"
	"errors"
	"fmt"
)

// Supported CipherPolicy.Algorithm values.
const (
	// AlgorithmAES256GCM is the only field-level encryption algorithm
	// this package implements (see field.go). It is also, today, the
	// only value CipherPolicy.Validate accepts.
	AlgorithmAES256GCM = "AES-256-GCM"
)

// CipherPolicy declares the cryptographic floor a Verdex deployment
// requires: which field-level encryption algorithm and key size to
// use, and the minimum TLS version acceptable for transport. It is
// intended to be embedded in a service's config struct and loaded via
// packages/config's Loader/profile layering (see
// packages/config/profile.go) exactly like any other config section --
// this package does not reimplement config loading, it only defines
// the schema and its validation.
type CipherPolicy struct {
	// Algorithm names the field-level encryption algorithm required.
	// Only AlgorithmAES256GCM is supported today; the field exists (as
	// opposed to being implicit) so a future algorithm addition is a
	// config-schema change, not a silent behavior change.
	Algorithm string `yaml:"algorithm"`

	// KeySizeBytes is the required symmetric key size, in bytes. Must
	// equal KeyBytes (32, for AES-256) -- the field is still explicit
	// (rather than hardcoded) so Validate can catch a misconfigured
	// deployment that thinks it is running a different key size than
	// this package actually enforces.
	KeySizeBytes int `yaml:"key_size_bytes"`

	// MinTLSVersion is the minimum acceptable TLS protocol version,
	// expressed as a crypto/tls version constant (e.g.
	// tls.VersionTLS12, tls.VersionTLS13). Must be at least
	// MinSupportedTLSVersion.
	MinTLSVersion uint16 `yaml:"min_tls_version"`
}

// DefaultCipherPolicy returns the CipherPolicy this package's own
// primitives satisfy out of the box: AES-256-GCM, 32-byte keys, TLS
// 1.2 minimum. Services that do not need a stricter floor can use this
// directly as their config default.
func DefaultCipherPolicy() CipherPolicy {
	return CipherPolicy{
		Algorithm:     AlgorithmAES256GCM,
		KeySizeBytes:  KeyBytes,
		MinTLSVersion: MinSupportedTLSVersion,
	}
}

// Validate checks p for a missing or unsupported field, returning
// ErrInvalidCipherPolicy (joined with every individual violation) if
// invalid.
func (p CipherPolicy) Validate() error {
	var errs []error

	if p.Algorithm == "" {
		errs = append(errs, errors.New("cipher_policy: algorithm must not be empty"))
	} else if p.Algorithm != AlgorithmAES256GCM {
		errs = append(errs, fmt.Errorf("cipher_policy: algorithm %q is not supported (only %q)", p.Algorithm, AlgorithmAES256GCM))
	}

	if p.KeySizeBytes != KeyBytes {
		errs = append(errs, fmt.Errorf("cipher_policy: key_size_bytes must be %d, got %d", KeyBytes, p.KeySizeBytes))
	}

	if p.MinTLSVersion < MinSupportedTLSVersion {
		errs = append(errs, fmt.Errorf("cipher_policy: min_tls_version %#x is below the required floor %#x (TLS 1.2)", p.MinTLSVersion, MinSupportedTLSVersion))
	}
	if p.MinTLSVersion > tls.VersionTLS13 {
		errs = append(errs, fmt.Errorf("cipher_policy: min_tls_version %#x is not a recognized TLS version", p.MinTLSVersion))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrInvalidCipherPolicy, errors.Join(errs...))
}
