package encryption_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

func TestLocalKeySource_RotateKeepsOldKeyDecryptable(t *testing.T) {
	ctx := context.Background()
	ks, err := encryption.NewLocalKeySource(encryption.Key{
		ID:       "v1",
		Material: bytes.Repeat([]byte{0x01}, encryption.KeyBytes),
	})
	if err != nil {
		t.Fatalf("NewLocalKeySource() error = %v, want nil", err)
	}

	// Encrypt with key v1.
	plaintext := []byte("filed before the rotation")
	ciphertextV1, err := encryption.Encrypt(ctx, ks, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	// Rotate to a new current key.
	newID, err := ks.Rotate(ctx)
	if err != nil {
		t.Fatalf("Rotate() error = %v, want nil", err)
	}
	if newID == "v1" {
		t.Fatal("Rotate() returned the same key ID as before rotation")
	}

	current, err := ks.CurrentKey(ctx)
	if err != nil {
		t.Fatalf("CurrentKey() error = %v, want nil", err)
	}
	if current.ID != newID {
		t.Fatalf("CurrentKey().ID = %q, want %q (the rotated-to key)", current.ID, newID)
	}

	// Old ciphertext must still decrypt.
	got, err := encryption.Decrypt(ctx, ks, ciphertextV1)
	if err != nil {
		t.Fatalf("Decrypt() of pre-rotation ciphertext error = %v, want nil", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}

	// New encryptions must use the new (v2) key.
	ciphertextV2, err := encryption.Encrypt(ctx, ks, []byte("filed after the rotation"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}
	env, err := encryption.ParseEnvelope(ciphertextV2)
	if err != nil {
		t.Fatalf("ParseEnvelope() error = %v, want nil", err)
	}
	if env.KeyID != newID {
		t.Fatalf("post-rotation Encrypt used key ID %q, want %q", env.KeyID, newID)
	}

	// The old envelope must still name the old key.
	envV1, err := encryption.ParseEnvelope(ciphertextV1)
	if err != nil {
		t.Fatalf("ParseEnvelope() error = %v, want nil", err)
	}
	if envV1.KeyID != "v1" {
		t.Fatalf("pre-rotation envelope key ID = %q, want %q", envV1.KeyID, "v1")
	}
}

func TestLocalKeySource_RotateMultipleTimes(t *testing.T) {
	ctx := context.Background()
	ks, err := encryption.NewLocalKeySource(encryption.Key{
		ID:       "gen1",
		Material: bytes.Repeat([]byte{0x02}, encryption.KeyBytes),
	})
	if err != nil {
		t.Fatalf("NewLocalKeySource() error = %v, want nil", err)
	}

	seenIDs := map[string]bool{"gen1": true}
	for i := 0; i < 5; i++ {
		id, err := ks.Rotate(ctx)
		if err != nil {
			t.Fatalf("Rotate() iteration %d error = %v, want nil", i, err)
		}
		if seenIDs[id] {
			t.Fatalf("Rotate() iteration %d reused key ID %q", i, id)
		}
		seenIDs[id] = true
	}

	if got, want := ks.KeyCount(), 6; got != want {
		t.Fatalf("KeyCount() = %d, want %d", got, want)
	}
}

func TestLocalKeySource_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	ks, err := encryption.NewLocalKeySource(encryption.Key{
		ID:       "only-key",
		Material: bytes.Repeat([]byte{0x03}, encryption.KeyBytes),
	})
	if err != nil {
		t.Fatalf("NewLocalKeySource() error = %v, want nil", err)
	}

	if _, err := ks.Key(ctx, "does-not-exist"); err == nil {
		t.Fatal("Key() error = nil, want ErrKeyNotFound for unknown key ID")
	}
}

func TestNewLocalKeySource_RejectsWrongKeySize(t *testing.T) {
	_, err := encryption.NewLocalKeySource(encryption.Key{
		ID:       "short",
		Material: []byte{0x01, 0x02, 0x03},
	})
	if err == nil {
		t.Fatal("NewLocalKeySource() error = nil, want ErrInvalidKeySize for a short key")
	}
}

func TestNewLocalKeySourceFromEnv(t *testing.T) {
	t.Setenv("VERDEX_TEST_ENCRYPTION_KEY", "a-development-secret-value-not-for-prod")

	ks, err := encryption.NewLocalKeySourceFromEnv("VERDEX_TEST_ENCRYPTION_KEY")
	if err != nil {
		t.Fatalf("NewLocalKeySourceFromEnv() error = %v, want nil", err)
	}

	ctx := context.Background()
	if _, err := ks.CurrentKey(ctx); err != nil {
		t.Fatalf("CurrentKey() error = %v, want nil", err)
	}
}

func TestNewLocalKeySourceFromEnv_MissingVar(t *testing.T) {
	_, err := encryption.NewLocalKeySourceFromEnv("VERDEX_DOES_NOT_EXIST_ENV_VAR")
	if err == nil {
		t.Fatal("NewLocalKeySourceFromEnv() error = nil, want error for unset env var")
	}
}
