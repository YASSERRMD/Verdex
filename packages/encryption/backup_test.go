package encryption_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/encryption"
)

func TestEncryptBackup_DecryptBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	backup := bytes.Repeat([]byte("case-export-blob;"), 100)

	ciphertext, err := encryption.EncryptBackup(ctx, ks, backup)
	if err != nil {
		t.Fatalf("EncryptBackup() error = %v, want nil", err)
	}
	if bytes.Equal(ciphertext, backup) {
		t.Fatal("EncryptBackup() output must not equal the plaintext backup")
	}

	got, err := encryption.DecryptBackup(ctx, ks, ciphertext)
	if err != nil {
		t.Fatalf("DecryptBackup() error = %v, want nil", err)
	}
	if !bytes.Equal(got, backup) {
		t.Fatal("DecryptBackup() output does not match the original backup bytes")
	}
}

func TestDecryptBackup_TamperedFails(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	ciphertext, err := encryption.EncryptBackup(ctx, ks, []byte("a backup blob"))
	if err != nil {
		t.Fatalf("EncryptBackup() error = %v, want nil", err)
	}
	tampered := append([]byte{}, ciphertext...)
	tampered[len(tampered)-1] ^= 0x01

	if _, err := encryption.DecryptBackup(ctx, ks, tampered); err == nil {
		t.Fatal("DecryptBackup() error = nil, want failure for tampered backup ciphertext")
	}
}

// Cross-compatibility: EncryptBackup/DecryptBackup share the same
// envelope format as Encrypt/Decrypt, so either pair can read the
// other's output.
func TestEncryptBackup_InteroperatesWithDecrypt(t *testing.T) {
	ctx := context.Background()
	ks := testKeySource(t)

	ciphertext, err := encryption.EncryptBackup(ctx, ks, []byte("shared envelope format"))
	if err != nil {
		t.Fatalf("EncryptBackup() error = %v, want nil", err)
	}

	got, err := encryption.Decrypt(ctx, ks, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() of EncryptBackup output error = %v, want nil", err)
	}
	if string(got) != "shared envelope format" {
		t.Fatalf("Decrypt() = %q, want %q", got, "shared envelope format")
	}
}
