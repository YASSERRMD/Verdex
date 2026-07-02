package citation

import (
	"context"
	"errors"

	"github.com/YASSERRMD/verdex/packages/graph"
)

// VerificationStatus classifies the outcome of verifying a single
// CitedUnit against a GraphStore.
type VerificationStatus string

const (
	// StatusVerified marks a CitedUnit whose NodeID exists in the
	// GraphStore under the claimed CaseID. This is the only status a
	// caller should treat as "safe to present as a real citation".
	StatusVerified VerificationStatus = "verified"

	// StatusHallucinated marks a CitedUnit whose NodeID does not exist in
	// the GraphStore at all — the citation was never real (the
	// anti-hallucination case this package exists to catch).
	StatusHallucinated VerificationStatus = "hallucinated"

	// StatusWrongCase marks a CitedUnit whose NodeID exists in the
	// GraphStore, but under a different case than claimed — a
	// cross-case citation leak, distinct from a citation that never
	// existed anywhere.
	StatusWrongCase VerificationStatus = "wrong_case"

	// StatusBroken marks a CitedUnit whose target once existed for the
	// claimed case but no longer does (deleted/moved), or whose recorded
	// Text no longer matches the node's current text (stale). See
	// broken.go for how this status is assigned; Verify alone never
	// returns StatusBroken (it cannot distinguish "never existed" from
	// "deleted" using GetNode's ErrNodeNotFound alone) — DetectBroken
	// upgrades a StatusHallucinated result to StatusBroken when the
	// caller can prove prior existence.
	StatusBroken VerificationStatus = "broken"
)

// allVerificationStatuses is the exhaustive set of recognized
// VerificationStatus values.
var allVerificationStatuses = map[VerificationStatus]struct{}{
	StatusVerified:     {},
	StatusHallucinated: {},
	StatusWrongCase:    {},
	StatusBroken:       {},
}

// IsValid reports whether s is one of the recognized VerificationStatus
// constants.
func (s VerificationStatus) IsValid() bool {
	_, ok := allVerificationStatuses[s]
	return ok
}

// VerificationResult is the outcome of verifying one CitedUnit against a
// GraphStore: whether the cited node actually exists for the claimed
// case, and if not, why.
type VerificationResult struct {
	// Unit is the CitedUnit that was verified.
	Unit CitedUnit

	// Status classifies the verification outcome.
	Status VerificationStatus

	// ActualCaseID is the CaseID the node was actually found under, when
	// Status is StatusWrongCase. Empty otherwise.
	ActualCaseID string
}

// Verified reports whether r's Status is StatusVerified.
func (r VerificationResult) Verified() bool {
	return r.Status == StatusVerified
}

// Verify confirms unit's underlying node actually exists in store under
// unit.CaseID — the core anti-hallucination check this package provides.
// It returns ErrNilGraphStore if store is nil, and ErrEmptyNodeID/
// ErrEmptyCaseID if unit is missing either identifier (a CitedUnit that
// can't even be checked is treated as a caller error, not a verification
// failure).
//
// Verify never returns StatusBroken: distinguishing "never existed" from
// "existed and was later deleted" requires information Verify alone does
// not have (see DetectBroken in broken.go, which callers should run
// first, or in combination, when they can supply prior-existence
// evidence).
func Verify(ctx context.Context, store graph.GraphStore, unit CitedUnit) (VerificationResult, error) {
	if store == nil {
		return VerificationResult{}, ErrNilGraphStore
	}
	if unit.NodeID == "" {
		return VerificationResult{}, ErrEmptyNodeID
	}
	if unit.CaseID == "" {
		return VerificationResult{}, ErrEmptyCaseID
	}

	node, err := store.GetNode(ctx, unit.NodeID)
	if err != nil {
		if errors.Is(err, graph.ErrNodeNotFound) {
			return VerificationResult{Unit: unit, Status: StatusHallucinated}, nil
		}
		return VerificationResult{}, err
	}

	if node.CaseID != unit.CaseID {
		return VerificationResult{Unit: unit, Status: StatusWrongCase, ActualCaseID: node.CaseID}, nil
	}

	return VerificationResult{Unit: unit, Status: StatusVerified}, nil
}

// VerifyAll runs Verify over every unit in units, collecting every result
// (unlike ResolveAll, VerifyAll does not stop at the first
// non-StatusVerified result — a caller auditing a whole retrieval batch
// wants every finding, not just the first). It returns the first
// unexpected error (a store failure other than ErrNodeNotFound, or a
// malformed CitedUnit) immediately, alongside whatever results were
// already collected.
func VerifyAll(ctx context.Context, store graph.GraphStore, units []CitedUnit) ([]VerificationResult, error) {
	out := make([]VerificationResult, 0, len(units))
	for _, u := range units {
		result, err := Verify(ctx, store, u)
		if err != nil {
			return out, err
		}
		out = append(out, result)
	}
	return out, nil
}
