package auditlog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// contentDigest derives a stable string representation of e's
// tamper-sensitive fields, used as an input to ComputeChainHash. Only
// fields that must not change undetected are included; ID and
// ChainHash itself are handled separately by ComputeChainHash's
// caller conventions (ID is included there, ChainHash is the output).
func contentDigest(e Event) string {
	var b strings.Builder
	b.WriteString(e.TenantID.String())
	b.WriteByte('|')
	b.WriteString(e.Time.UTC().Format("2006-01-02T15:04:05.000000000Z"))
	b.WriteByte('|')
	b.WriteString(e.Actor)
	b.WriteByte('|')
	b.WriteString(e.Action)
	b.WriteByte('|')
	b.WriteString(e.Target)
	b.WriteByte('|')
	b.WriteString(e.Outcome)
	b.WriteByte('|')
	b.WriteString(string(e.Kind))
	b.WriteByte('|')
	b.WriteString(e.CaseID.String())
	b.WriteByte('|')
	b.WriteString(e.Detail)
	return b.String()
}

// ComputeChainHash derives the chain hash for current given the
// previous event's chain hash. The hash is
// SHA-256(prevChainHash + currentID + contentDigest(current)),
// mirroring packages/provenance.ComputeChainHash's
// SHA-256(prevChainHash + currentID + currentContentHash) exactly —
// same shape, applied to audit events instead of provenance records.
func ComputeChainHash(prevChainHash string, current Event) string {
	input := prevChainHash + current.ID.String() + strconv.Itoa(len(contentDigest(current))) + contentDigest(current)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// ChainBuilder accumulates Events and produces a tamper-evident chain
// by assigning PrevHash/ChainHash values in sequence, exactly
// mirroring packages/provenance.ChainBuilder's role for
// ProvenanceRecord.
type ChainBuilder struct{}

// BuildChain sets PrevHash and ChainHash on each event in-place (left
// to right, i.e. events must already be in chronological/insertion
// order) and returns the updated slice.
func (ChainBuilder) BuildChain(events []Event) []Event {
	prevHash := ""
	for i := range events {
		events[i].PrevHash = prevHash
		events[i].ChainHash = ComputeChainHash(prevHash, events[i])
		prevHash = events[i].ChainHash
	}
	return events
}

// VerifyChain recomputes the expected chain hash for every event (in
// the order given, which callers must ensure is chain order) and
// compares it to the stored ChainHash and PrevHash. It returns:
//
//   - valid=true, brokenAt=-1, err=nil when the chain is intact.
//   - valid=false, brokenAt=i, err=ErrChainBroken when events[i] has an
//     incorrect PrevHash or ChainHash — i.e. tampering with any single
//     stored event (or reordering/deleting one) is detectable at the
//     first event whose linkage no longer matches.
//
// VerifyChain anchors on events[0].PrevHash (rather than assuming the
// empty string) so that a legitimately Purge-truncated tail — where
// the oldest surviving event's PrevHash still points at a
// now-deleted, retention-expired predecessor — verifies successfully
// as a segment. Passing the full, never-purged history for a tenant
// (starting from its genesis event, whose real PrevHash is "") gives
// the strongest end-to-end guarantee; passing a Query'd or
// Purge-survived subset still detects any tampering within that
// subset.
func (ChainBuilder) VerifyChain(events []Event) (valid bool, brokenAt int, err error) {
	if len(events) == 0 {
		return true, -1, nil
	}

	prevHash := events[0].PrevHash
	for i := range events {
		if events[i].PrevHash != prevHash {
			return false, i, fmt.Errorf("%w at index %d (id=%s): prev_hash mismatch", ErrChainBroken, i, events[i].ID)
		}
		expected := ComputeChainHash(prevHash, events[i])
		if events[i].ChainHash != expected {
			return false, i, fmt.Errorf("%w at index %d (id=%s): chain_hash mismatch", ErrChainBroken, i, events[i].ID)
		}
		prevHash = events[i].ChainHash
	}
	return true, -1, nil
}

// BuildChain is a package-level convenience wrapper around
// ChainBuilder.BuildChain.
func BuildChain(events []Event) []Event {
	return ChainBuilder{}.BuildChain(events)
}

// VerifyChain is a package-level convenience wrapper around
// ChainBuilder.VerifyChain.
func VerifyChain(events []Event) (valid bool, brokenAt int, err error) {
	return ChainBuilder{}.VerifyChain(events)
}

// VerifyGenesisChain is VerifyChain plus one extra check: the first
// event's PrevHash must be exactly "" (the true chain start, never
// purged away). Use this when the caller can guarantee events is a
// tenant's *complete*, unpurged history — e.g. immediately in tests,
// or in a from-scratch archival export before any retention purge has
// ever run. ListAll's result after a Purge will correctly fail this
// stricter check (its first surviving event's PrevHash points at a
// deleted predecessor), which is why Store.VerifyTenantChain uses
// VerifyChain, not this function, in the general (post-purge) case.
func VerifyGenesisChain(events []Event) (valid bool, brokenAt int, err error) {
	if len(events) > 0 && events[0].PrevHash != "" {
		return false, 0, fmt.Errorf("%w at index 0 (id=%s): not a genesis chain (prev_hash=%q)", ErrChainBroken, events[0].ID, events[0].PrevHash)
	}
	return VerifyChain(events)
}
