package encryption_test

import (
	"crypto/tls"
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

func TestDefaultCipherPolicy_IsValid(t *testing.T) {
	policy := encryption.DefaultCipherPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil for DefaultCipherPolicy()", err)
	}
}

func TestCipherPolicy_ValidateRejectsUnsupportedAlgorithm(t *testing.T) {
	policy := encryption.DefaultCipherPolicy()
	policy.Algorithm = "DES"

	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for unsupported algorithm")
	}
}

func TestCipherPolicy_ValidateRejectsEmptyAlgorithm(t *testing.T) {
	policy := encryption.DefaultCipherPolicy()
	policy.Algorithm = ""

	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for empty algorithm")
	}
}

func TestCipherPolicy_ValidateRejectsWrongKeySize(t *testing.T) {
	tests := []int{0, 16, 24, 31, 33, 64}
	for _, size := range tests {
		policy := encryption.DefaultCipherPolicy()
		policy.KeySizeBytes = size

		if err := policy.Validate(); err == nil {
			t.Errorf("Validate() error = nil, want error for key_size_bytes=%d", size)
		}
	}
}

func TestCipherPolicy_ValidateRejectsWeakTLSVersion(t *testing.T) {
	policy := encryption.DefaultCipherPolicy()
	policy.MinTLSVersion = tls.VersionTLS11

	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for min_tls_version below TLS 1.2")
	}
}

func TestCipherPolicy_ValidateAcceptsTLS13Floor(t *testing.T) {
	policy := encryption.DefaultCipherPolicy()
	policy.MinTLSVersion = tls.VersionTLS13

	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil for a TLS 1.3 floor", err)
	}
}

func TestCipherPolicy_ValidateRejectsUnrecognizedTLSVersion(t *testing.T) {
	policy := encryption.DefaultCipherPolicy()
	policy.MinTLSVersion = 0x9999

	if err := policy.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error for an unrecognized TLS version constant")
	}
}

func TestCipherPolicy_ValidateJoinsMultipleViolations(t *testing.T) {
	policy := encryption.CipherPolicy{} // every field zero-valued/invalid

	err := policy.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for a fully zero-valued policy")
	}
}
