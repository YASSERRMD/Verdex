package citation

import (
	"context"
	"errors"

	"github.com/YASSERRMD/verdex/packages/graph"
)

// KnownNodeIDs is a caller-supplied set of node IDs known to have existed
// for a case at some prior point (e.g. captured from a previous
// successful Verify pass, or from an audit log/Repository snapshot — see
// store.go). DetectBroken uses this to distinguish "this node was
// verified before and is now missing" (broken: deleted or moved) from
// "this node was never verified in the first place" (hallucinated: it may
// simply never have existed). Without this evidence, Verify alone cannot
// tell the two apart, since graph.GraphStore.GetNode's ErrNodeNotFound
// carries no history.
type KnownNodeIDs map[string]struct{}

// Contains reports whether id is present in k. A nil KnownNodeIDs
// contains nothing.
func (k KnownNodeIDs) Contains(id string) bool {
	_, ok := k[id]
	return ok
}

// BrokenReason classifies why DetectBroken assigned StatusBroken to a
// CitedUnit.
type BrokenReason string

const (
	// BrokenReasonNone means DetectBroken did not classify the unit as
	// broken.
	BrokenReasonNone BrokenReason = ""

	// BrokenReasonDeleted means the unit's target node previously existed
	// (per KnownNodeIDs) for the claimed case but is no longer present in
	// the GraphStore — e.g. removed by GraphStore.DeleteTree, or replaced
	// by a later irac.TreeRevision that dropped the node.
	BrokenReasonDeleted BrokenReason = "deleted"

	// BrokenReasonStale means the unit's target node still exists, but
	// its current Text no longer matches the CitedUnit's recorded Text —
	// the source the citation was built from has since been edited or
	// re-extracted, so the span offsets and quoted text can no longer be
	// trusted to point at the same claim.
	BrokenReasonStale BrokenReason = "stale"
)

// BrokenResult is the outcome of running DetectBroken against a single
// CitedUnit.
type BrokenResult struct {
	// Unit is the CitedUnit that was checked.
	Unit CitedUnit

	// Reason classifies why (or whether) the unit was found broken.
	Reason BrokenReason
}

// Broken reports whether r's Reason is anything other than
// BrokenReasonNone.
func (r BrokenResult) Broken() bool {
	return r.Reason != BrokenReasonNone
}

// DetectBroken checks unit for the two broken-citation failure modes this
// package distinguishes from "never existed" (StatusHallucinated):
//
//   - Deleted/moved: unit.NodeID is absent from store for unit.CaseID,
//     but is present in known (evidence it existed before). Reported as
//     BrokenReasonDeleted.
//   - Stale: unit.NodeID is present in store, but its current Text
//     differs from unit.Text (the text the citation/spans were built
//     from). Reported as BrokenReasonStale.
//
// If neither applies, Reason is BrokenReasonNone — this includes the case
// where the node is missing and NOT in known (that is a plain
// StatusHallucinated per Verify, not a broken citation: there is no
// evidence it ever existed).
//
// DetectBroken returns ErrNilGraphStore if store is nil.
func DetectBroken(ctx context.Context, store graph.GraphStore, known KnownNodeIDs, unit CitedUnit) (BrokenResult, error) {
	if store == nil {
		return BrokenResult{}, ErrNilGraphStore
	}
	if unit.NodeID == "" {
		return BrokenResult{}, ErrEmptyNodeID
	}

	node, err := store.GetNode(ctx, unit.NodeID)
	if err != nil {
		if errors.Is(err, graph.ErrNodeNotFound) {
			if known.Contains(unit.NodeID) {
				return BrokenResult{Unit: unit, Reason: BrokenReasonDeleted}, nil
			}
			return BrokenResult{Unit: unit, Reason: BrokenReasonNone}, nil
		}
		return BrokenResult{}, err
	}

	if node.CaseID == unit.CaseID && unit.Text != "" && node.Text != unit.Text {
		return BrokenResult{Unit: unit, Reason: BrokenReasonStale}, nil
	}

	return BrokenResult{Unit: unit, Reason: BrokenReasonNone}, nil
}

// DetectBrokenAll runs DetectBroken over every unit in units, collecting
// every result. It returns the first unexpected error immediately,
// alongside whatever results were already collected.
func DetectBrokenAll(ctx context.Context, store graph.GraphStore, known KnownNodeIDs, units []CitedUnit) ([]BrokenResult, error) {
	out := make([]BrokenResult, 0, len(units))
	for _, u := range units {
		result, err := DetectBroken(ctx, store, known, u)
		if err != nil {
			return out, err
		}
		out = append(out, result)
	}
	return out, nil
}

// FindingsFromBroken translates a BrokenResult into zero or one Finding:
// BrokenReasonNone produces no Finding, BrokenReasonDeleted produces a
// SeverityCritical Finding (the citation now points at nothing), and
// BrokenReasonStale produces a SeverityWarning Finding (the node still
// exists, but the quoted text has drifted — worth a reviewer's attention
// but not proof of hallucination).
func FindingsFromBroken(result BrokenResult) []Finding {
	unit := result.Unit
	switch result.Reason {
	case BrokenReasonNone:
		return nil
	case BrokenReasonDeleted:
		return []Finding{{
			Severity: SeverityCritical,
			Code:     CodeBrokenDeleted,
			Message:  "citation target was deleted or moved after this citation was created",
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		}}
	case BrokenReasonStale:
		return []Finding{{
			Severity: SeverityWarning,
			Code:     CodeBrokenStale,
			Message:  "citation target's text no longer matches the text this citation was built from",
			NodeID:   unit.NodeID,
			CaseID:   unit.CaseID,
		}}
	default:
		return nil
	}
}
