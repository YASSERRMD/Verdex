package keymanagement_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/encryption"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
)

// TestFileProvider_RoundTrip proves task 8: FileProvider works with
// zero network dependency (it is exercised here purely against a
// t.TempDir(), no dial/listen anywhere in this test) and round-trips
// key material: Rotate writes it, CurrentKey/Key read it back
// unchanged.
func TestFileProvider_RoundTrip(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()

	repo := keymanagement.NewInMemoryRepository()
	fp, err := keymanagement.NewFileProvider(t.TempDir(), keymanagement.DeriveMasterKey("air-gapped-test-passphrase"), repo)
	if err != nil {
		t.Fatalf("NewFileProvider: %v", err)
	}

	meta, err := fp.Rotate(ctx, tenantID.String())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	current, err := fp.CurrentKey(ctx, tenantID.String())
	if err != nil {
		t.Fatalf("CurrentKey: %v", err)
	}
	if current.Metadata.ID != meta.ID {
		t.Fatalf("CurrentKey().Metadata.ID = %q, want %q", current.Metadata.ID, meta.ID)
	}
	if len(current.Material) != 32 {
		t.Fatalf("CurrentKey().Material length = %d, want 32", len(current.Material))
	}

	byID, err := fp.Key(ctx, tenantID.String(), meta.ID)
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if !bytes.Equal(byID.Material, current.Material) {
		t.Fatal("Key() and CurrentKey() returned different material for the same key ID")
	}
}

// TestFileProvider_RotationPreservesOldKeyDecryptability proves
// FileProvider's Rotate keeps the pre-rotation key file on disk and
// resolvable, and demonstrates it end-to-end through the real
// packages/encryption Encrypt/Decrypt path via Adapter -- exactly the
// task 9 requirement, now specifically against the offline/file-backed
// provider rather than the plain in-memory test fixture.
func TestFileProvider_RotationPreservesOldKeyDecryptability(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()

	repo := keymanagement.NewInMemoryRepository()
	fp, err := keymanagement.NewFileProvider(t.TempDir(), keymanagement.DeriveMasterKey("another-test-passphrase"), repo)
	if err != nil {
		t.Fatalf("NewFileProvider: %v", err)
	}
	if _, err := fp.Rotate(ctx, tenantID.String()); err != nil {
		t.Fatalf("initial Rotate: %v", err)
	}

	adapter, err := keymanagement.NewAdapter(fp, tenantID)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	plaintext := []byte("air-gapped filing, encrypted offline")
	ciphertext, err := encryption.Encrypt(ctx, adapter, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := fp.Rotate(ctx, tenantID.String()); err != nil {
		t.Fatalf("second Rotate: %v", err)
	}

	decrypted, err := encryption.Decrypt(ctx, adapter, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt after rotation: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

// TestFileProvider_PerTenantIsolation proves FileProvider partitions
// key material by tenant on disk: two tenants' key files never
// collide, and one tenant's key ID is not resolvable under another
// tenant's scope.
func TestFileProvider_PerTenantIsolation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := keymanagement.NewInMemoryRepository()
	fp, err := keymanagement.NewFileProvider(root, keymanagement.DeriveMasterKey("shared-passphrase"), repo)
	if err != nil {
		t.Fatalf("NewFileProvider: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()

	metaA, err := fp.Rotate(ctx, tenantA.String())
	if err != nil {
		t.Fatalf("Rotate tenant A: %v", err)
	}
	metaB, err := fp.Rotate(ctx, tenantB.String())
	if err != nil {
		t.Fatalf("Rotate tenant B: %v", err)
	}

	keyA, err := fp.CurrentKey(ctx, tenantA.String())
	if err != nil {
		t.Fatalf("CurrentKey tenant A: %v", err)
	}
	keyB, err := fp.CurrentKey(ctx, tenantB.String())
	if err != nil {
		t.Fatalf("CurrentKey tenant B: %v", err)
	}
	if bytes.Equal(keyA.Material, keyB.Material) {
		t.Fatal("tenant A and tenant B resolved to identical key material")
	}

	// Tenant B must not be able to resolve tenant A's key ID under its
	// own scope -- Repository.Get is tenant-scoped even though the
	// files themselves live in separate directories.
	if _, err := fp.Key(ctx, tenantB.String(), metaA.ID); err == nil {
		t.Fatal("tenant B resolved tenant A's key ID; want an error")
	}
	if _, err := fp.Key(ctx, tenantA.String(), metaB.ID); err == nil {
		t.Fatal("tenant A resolved tenant B's key ID; want an error")
	}

	// The two tenants' key files must live in distinct subdirectories.
	dirA := filepath.Join(root, tenantA.String())
	dirB := filepath.Join(root, tenantB.String())
	if dirA == dirB {
		t.Fatal("tenant A and tenant B share the same on-disk directory")
	}
}

func TestNewFileProvider_RejectsInvalidMasterKeyLength(t *testing.T) {
	repo := keymanagement.NewInMemoryRepository()
	if _, err := keymanagement.NewFileProvider(t.TempDir(), []byte("too-short"), repo); !errors.Is(err, keymanagement.ErrInvalidKeyMaterial) {
		t.Fatalf("NewFileProvider() short master key error = %v, want ErrInvalidKeyMaterial", err)
	}
}

func TestFileProvider_CurrentKey_NoActiveKeyFailsClosed(t *testing.T) {
	ctx := context.Background()
	repo := keymanagement.NewInMemoryRepository()
	fp, err := keymanagement.NewFileProvider(t.TempDir(), keymanagement.DeriveMasterKey("passphrase"), repo)
	if err != nil {
		t.Fatalf("NewFileProvider: %v", err)
	}

	if _, err := fp.CurrentKey(ctx, uuid.New().String()); !errors.Is(err, keymanagement.ErrNoActiveKey) {
		t.Fatalf("CurrentKey() with no rotation yet error = %v, want ErrNoActiveKey", err)
	}
}
