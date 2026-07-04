package backupdr_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/backupdr"
)

func TestComputeIntegrityHash_Deterministic(t *testing.T) {
	t.Parallel()
	data := []byte("a backup artifact's bytes")
	h1 := backupdr.ComputeIntegrityHash(data)
	h2 := backupdr.ComputeIntegrityHash(data)
	if h1 != h2 {
		t.Fatalf("ComputeIntegrityHash not deterministic: %q != %q", h1, h2)
	}
	if len(h1) != 64 { // hex-encoded SHA-256 is 64 chars
		t.Fatalf("ComputeIntegrityHash length = %d, want 64 (hex SHA-256)", len(h1))
	}
}

func TestComputeIntegrityHash_DifferentInputsDifferentHashes(t *testing.T) {
	t.Parallel()
	h1 := backupdr.ComputeIntegrityHash([]byte("backup A"))
	h2 := backupdr.ComputeIntegrityHash([]byte("backup B"))
	if h1 == h2 {
		t.Fatal("ComputeIntegrityHash produced the same hash for different inputs")
	}
}

func TestVerifyIntegrity_Match(t *testing.T) {
	t.Parallel()
	data := []byte("a backup artifact's bytes")
	hash := backupdr.ComputeIntegrityHash(data)
	record := backupdr.BackupRecord{IntegrityHash: hash}

	if err := backupdr.VerifyIntegrity(record, hash); err != nil {
		t.Fatalf("VerifyIntegrity() = %v, want nil for matching hash", err)
	}
}

func TestVerifyIntegrity_CaseInsensitiveMatch(t *testing.T) {
	t.Parallel()
	record := backupdr.BackupRecord{IntegrityHash: "ABCDEF0123456789"}
	if err := backupdr.VerifyIntegrity(record, "abcdef0123456789"); err != nil {
		t.Fatalf("VerifyIntegrity() = %v, want nil for case-insensitive match", err)
	}
}

func TestVerifyIntegrity_Mismatch(t *testing.T) {
	t.Parallel()
	record := backupdr.BackupRecord{IntegrityHash: backupdr.ComputeIntegrityHash([]byte("original bytes"))}
	tamperedHash := backupdr.ComputeIntegrityHash([]byte("tampered bytes"))

	err := backupdr.VerifyIntegrity(record, tamperedHash)
	if !errors.Is(err, backupdr.ErrIntegrityMismatch) {
		t.Fatalf("VerifyIntegrity() error = %v, want ErrIntegrityMismatch", err)
	}
}

func TestVerifyIntegrity_BlankStoredHash(t *testing.T) {
	t.Parallel()
	record := backupdr.BackupRecord{IntegrityHash: ""}
	err := backupdr.VerifyIntegrity(record, backupdr.ComputeIntegrityHash([]byte("anything")))
	if !errors.Is(err, backupdr.ErrIntegrityMismatch) {
		t.Fatalf("VerifyIntegrity() error = %v, want ErrIntegrityMismatch for blank stored hash", err)
	}
}

func TestVerifyIntegrity_BlankComputedHash(t *testing.T) {
	t.Parallel()
	record := backupdr.BackupRecord{IntegrityHash: "deadbeef"}
	err := backupdr.VerifyIntegrity(record, "")
	if !errors.Is(err, backupdr.ErrIntegrityMismatch) {
		t.Fatalf("VerifyIntegrity() error = %v, want ErrIntegrityMismatch for blank computed hash", err)
	}
}
