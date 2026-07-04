package backupdr

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ComputeIntegrityHash returns the hex-encoded SHA-256 digest of data,
// mirroring packages/provenance.ProvenanceRecord.ContentHash's exact
// convention (hex-encoded SHA-256, algorithm name always "sha256") so
// a BackupRecord.IntegrityHash and a ProvenanceRecord.ContentHash are
// directly comparable in format even though this package does not
// import packages/provenance. Backup-execution tooling that actually
// reads a backup artifact's bytes calls this to produce the value it
// hands to VerifyIntegrity; this package itself never reads backup
// bytes off Location/Reference.
func ComputeIntegrityHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// VerifyIntegrity compares record's stored IntegrityHash against
// computedHash -- a hash the caller freshly computed from the actual
// backup bytes (e.g. via ComputeIntegrityHash) -- and reports whether
// they match (task 8). This mirrors packages/provenance's
// hash-verification pattern (ComputeChainHash's stored-vs-recomputed
// comparison in chain.go) by reference: same "recompute and compare"
// shape, applied to a single artifact's content hash rather than a
// hash-chain link. Returns ErrIntegrityMismatch (wrapped) if the
// hashes differ, or if record's stored hash is blank (nothing to
// verify against is itself a verification failure, not a vacuous
// pass).
func VerifyIntegrity(record BackupRecord, computedHash string) error {
	stored := strings.TrimSpace(record.IntegrityHash)
	fresh := strings.TrimSpace(computedHash)
	if stored == "" || fresh == "" || !strings.EqualFold(stored, fresh) {
		return wrapf("VerifyIntegrity", ErrIntegrityMismatch)
	}
	return nil
}
