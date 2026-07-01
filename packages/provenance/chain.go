package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ComputeChainHash derives the chain hash for current given the previous record.
// The hash is SHA-256(prevChainHash + currentID + currentContentHash).
// When prev is nil (first record in a chain) the previous chain hash is treated
// as the empty string.
func ComputeChainHash(prev *ProvenanceRecord, current ProvenanceRecord) string {
	prevHash := ""
	if prev != nil {
		prevHash = prev.ChainHash
	}

	input := prevHash + current.ID.String() + current.ContentHash
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// ChainBuilder accumulates provenance records and produces a tamper-evident
// chain by assigning ChainHash values in sequence.
type ChainBuilder struct{}

// BuildChain sets the ChainHash on each record in-place (left to right) and
// returns the updated slice. The input slice is modified directly.
func (ChainBuilder) BuildChain(records []ProvenanceRecord) []ProvenanceRecord {
	var prev *ProvenanceRecord
	for i := range records {
		records[i].ChainHash = ComputeChainHash(prev, records[i])
		prev = &records[i]
	}
	return records
}

// VerifyChain recomputes the expected chain hash for every record and compares
// it to the stored ChainHash. It returns:
//
//   - valid=true, brokenAt=-1, err=nil when the chain is intact.
//   - valid=false, brokenAt=i, err=ErrChainBroken when records[i] has an
//     incorrect chain hash.
func (ChainBuilder) VerifyChain(records []ProvenanceRecord) (valid bool, brokenAt int, err error) {
	if len(records) == 0 {
		return true, -1, nil
	}

	var prev *ProvenanceRecord
	for i := range records {
		expected := ComputeChainHash(prev, records[i])
		if records[i].ChainHash != expected {
			return false, i, fmt.Errorf("%w at index %d (id=%s)", ErrChainBroken, i, records[i].ID)
		}
		prev = &records[i]
	}
	return true, -1, nil
}

// BuildChain is a package-level convenience wrapper around ChainBuilder.BuildChain.
func BuildChain(records []ProvenanceRecord) []ProvenanceRecord {
	return ChainBuilder{}.BuildChain(records)
}

// VerifyChain is a package-level convenience wrapper around ChainBuilder.VerifyChain.
func VerifyChain(records []ProvenanceRecord) (valid bool, brokenAt int, err error) {
	return ChainBuilder{}.VerifyChain(records)
}
