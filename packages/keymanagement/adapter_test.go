package keymanagement_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/encryption"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
)

// TestAdapter_ImplementsEncryptionKeySource proves (not merely
// asserts via var _) that keymanagement.Adapter satisfies
// encryption.KeySource by using it exactly as a real caller would: as
// the source argument to encryption.Encrypt and encryption.Decrypt.
func TestAdapter_ImplementsEncryptionKeySource(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()

	repo := keymanagement.NewInMemoryRepository()
	provider := newInMemoryProvider(repo)
	if err := seedActiveKey(ctx, provider, tenantID); err != nil {
		t.Fatalf("seedActiveKey: %v", err)
	}

	adapter, err := keymanagement.NewAdapter(provider, tenantID)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	// adapter is used here as a plain encryption.KeySource -- if it did
	// not satisfy the interface, this would fail to compile.
	var source encryption.KeySource = adapter

	plaintext := []byte("filed under seal")
	ciphertext, err := encryption.Encrypt(ctx, source, plaintext)
	if err != nil {
		t.Fatalf("encryption.Encrypt() error = %v, want nil", err)
	}

	decrypted, err := encryption.Decrypt(ctx, source, ciphertext)
	if err != nil {
		t.Fatalf("encryption.Decrypt() error = %v, want nil", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

// TestAdapter_Rotate_PreservesOldKeyDecryptability proves rotation
// through Adapter (which implements encryption.Rotator) keeps
// ciphertext encrypted under the pre-rotation key decryptable, using
// the real packages/encryption Encrypt/Decrypt path end-to-end --
// matching Phase 075's rotation-test expectation
// (packages/encryption.Rotator's doc comment) exactly, now proven
// against a real Phase 076 implementation instead of only
// LocalKeySource.
func TestAdapter_Rotate_PreservesOldKeyDecryptability(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()

	repo := keymanagement.NewInMemoryRepository()
	provider := newInMemoryProvider(repo)
	if err := seedActiveKey(ctx, provider, tenantID); err != nil {
		t.Fatalf("seedActiveKey: %v", err)
	}

	adapter, err := keymanagement.NewAdapter(provider, tenantID)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	plaintext := []byte("encrypted before rotation")
	ciphertext, err := encryption.Encrypt(ctx, adapter, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want nil", err)
	}

	rotator, ok := encryption.KeySource(adapter).(encryption.Rotator)
	if !ok {
		t.Fatal("Adapter does not implement encryption.Rotator")
	}
	newID, err := rotator.Rotate(ctx)
	if err != nil {
		t.Fatalf("Rotate() error = %v, want nil", err)
	}
	if newID == "" {
		t.Fatal("Rotate() returned empty key ID")
	}

	// Old ciphertext, encrypted under the pre-rotation key, must still
	// decrypt successfully.
	decrypted, err := encryption.Decrypt(ctx, adapter, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() of pre-rotation ciphertext error = %v, want nil", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}

	// New encryption must use the new (post-rotation) key.
	newCiphertext, err := encryption.Encrypt(ctx, adapter, []byte("encrypted after rotation"))
	if err != nil {
		t.Fatalf("Encrypt() after rotation error = %v, want nil", err)
	}
	if bytes.Equal(newCiphertext, ciphertext) {
		t.Fatal("post-rotation ciphertext identical to pre-rotation ciphertext")
	}
}

func TestNewAdapter_RejectsNilProvider(t *testing.T) {
	if _, err := keymanagement.NewAdapter(nil, uuid.New()); err == nil {
		t.Fatal("NewAdapter() error = nil, want ErrNilProvider")
	}
}

func TestNewAdapter_RejectsEmptyTenantID(t *testing.T) {
	repo := keymanagement.NewInMemoryRepository()
	provider := newInMemoryProvider(repo)
	if _, err := keymanagement.NewAdapter(provider, uuid.Nil); err == nil {
		t.Fatal("NewAdapter() error = nil, want ErrEmptyTenantID")
	}
}
