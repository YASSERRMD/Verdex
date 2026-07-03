package encryption_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

func testKeySource(t *testing.T) *encryption.LocalKeySource {
	t.Helper()
	ks, err := encryption.NewLocalKeySource(encryption.Key{
		ID:       "test-key-1",
		Material: bytes.Repeat([]byte{0x42}, encryption.KeyBytes),
	})
	if err != nil {
		t.Fatalf("NewLocalKeySource() error = %v, want nil", err)
	}
	return ks
}

func TestEncrypt_DecryptRoundTrip(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	plaintext := []byte("national-id: 784-1985-1234567-1")

	ciphertext, err := encryption.Encrypt(ctx, ks, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	got, err := encryption.Decrypt(ctx, ks, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v, want nil", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestEncrypt_CiphertextDiffersFromPlaintext(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	plaintext := []byte("this is extremely sensitive case content about Jane Doe")

	ciphertext, err := encryption.Encrypt(ctx, ks, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext must not contain the plaintext as a substring")
	}
	if strings.Contains(string(ciphertext), "Jane Doe") {
		t.Fatal("ciphertext must not leak recognizable plaintext substrings")
	}
}

func TestEncrypt_NonceIsRandomPerCall(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)
	plaintext := []byte("same plaintext every time")

	first, err := encryption.Encrypt(ctx, ks, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}
	second, err := encryption.Encrypt(ctx, ks, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("two Encrypt calls on identical plaintext must not produce identical ciphertext (nonce reuse)")
	}
}

func TestDecrypt_TamperedCiphertextFailsAuthentication(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	ciphertext, err := encryption.Encrypt(ctx, ks, []byte("integrity-protected value"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err = encryption.Decrypt(ctx, ks, tampered)
	if err == nil {
		t.Fatal("Decrypt() error = nil, want authentication failure for tampered ciphertext")
	}
	if !errors.Is(err, encryption.ErrAuthenticationFailed) {
		t.Fatalf("Decrypt() error = %v, want ErrAuthenticationFailed", err)
	}
}

func TestDecrypt_WrongKeyFailsAuthentication(t *testing.T) {
	ctx := context.Background()
	ks1 := testKeySource(t)
	ks2, err := encryption.NewLocalKeySource(encryption.Key{
		ID:       "test-key-1", // same ID, different material
		Material: bytes.Repeat([]byte{0x99}, encryption.KeyBytes),
	})
	if err != nil {
		t.Fatalf("NewLocalKeySource() error = %v, want nil", err)
	}

	ciphertext, err := encryption.Encrypt(ctx, ks1, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	_, err = encryption.Decrypt(ctx, ks2, ciphertext)
	if err == nil {
		t.Fatal("Decrypt() error = nil, want authentication failure when decrypting with the wrong key material")
	}
}

func TestEncrypt_EmptyPlaintextRejected(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	if _, err := encryption.Encrypt(ctx, ks, nil); err == nil {
		t.Fatal("Encrypt(nil) error = nil, want error")
	}
	if _, err := encryption.Encrypt(ctx, ks, []byte{}); err == nil {
		t.Fatal("Encrypt([]byte{}) error = nil, want error")
	}
}

func TestEncrypt_NilKeySourceRejected(t *testing.T) {
	ctx := context.Background()
	if _, err := encryption.Encrypt(ctx, nil, []byte("x")); err == nil {
		t.Fatal("Encrypt() error = nil, want ErrNilKeySource for nil source")
	}
}

func TestDecrypt_EmptyCiphertextRejected(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)
	if _, err := encryption.Decrypt(ctx, ks, nil); err == nil {
		t.Fatal("Decrypt(nil) error = nil, want error")
	}
}

func TestDecrypt_InvalidEnvelopeRejected(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)
	if _, err := encryption.Decrypt(ctx, ks, []byte("not an envelope at all")); err == nil {
		t.Fatal("Decrypt() error = nil, want ErrInvalidEnvelope for garbage input")
	}
}
