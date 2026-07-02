package guardrail

import (
	"context"
	"fmt"
)

// SignoffStatus is the human sign-off state of a case's reasoning output,
// as recorded by whatever workflow tracks sign-off decisions.
type SignoffStatus int

// SignoffStatus values.
const (
	// SignoffPending means no human reviewer has yet approved or
	// rejected the case's reasoning output. This is the fail-closed
	// default: finalization is blocked until a decision is recorded.
	SignoffPending SignoffStatus = iota

	// SignoffApproved means a human reviewer has approved the case's
	// reasoning output for finalization. This is the only status
	// CanFinalize accepts.
	SignoffApproved

	// SignoffRejected means a human reviewer has explicitly rejected
	// the case's reasoning output. Like SignoffPending, this blocks
	// finalization, but is recorded distinctly so a caller (or an
	// audit trail) can tell "nobody has looked at this yet" apart from
	// "somebody looked and said no."
	SignoffRejected
)

// String returns a human-readable name for status.
func (s SignoffStatus) String() string {
	switch s {
	case SignoffPending:
		return "pending"
	case SignoffApproved:
		return "approved"
	case SignoffRejected:
		return "rejected"
	default:
		return fmt.Sprintf("signoff_status(%d)", int(s))
	}
}

// SignoffGate is the forward-looking extension point this phase defines
// for Phase 068 (Human sign-off workflow, Part 6 of the implementation
// plan) to implement. Today, no sign-off feature exists anywhere in the
// codebase; NoSignoffRecordedGate (below) is the only implementation,
// and it always reports SignoffPending — fail-closed, not fail-open, so
// CanFinalize blocks every case until Phase 068 lands a real
// implementation that can report SignoffApproved.
//
// This mirrors packages/treeassembly.ConclusionProvider's extension-point
// pattern precisely: treeassembly.ComposeTree accepted a
// ConclusionProvider interface starting at Phase 039, with
// NoOpConclusionProvider as the only implementation until Phase 055
// supplied packages/synthesisagent.Provider — no change to
// treeassembly's own composition logic was required when that happened.
// The same shape applies here: once Phase 068 exists, it need only
// implement SignoffGate and be wired into CanFinalize's caller; no
// change to this package is required.
type SignoffGate interface {
	// Status reports caseID's current human sign-off status. An
	// implementation backed by a real workflow (Phase 068) is expected
	// to look up a persisted decision; NoSignoffRecordedGate always
	// returns SignoffPending.
	Status(ctx context.Context, caseID string) (SignoffStatus, error)
}

// NoSignoffRecordedGate is the default SignoffGate implementation used
// until Phase 068 supplies a real one. It always reports SignoffPending
// for any case, regardless of caseID — there is, today, no mechanism
// anywhere in the codebase by which a case could ever have an approved
// sign-off, so reporting anything else would be a fail-open lie.
type NoSignoffRecordedGate struct{}

// Status implements SignoffGate by always returning SignoffPending, nil.
func (NoSignoffRecordedGate) Status(_ context.Context, caseID string) (SignoffStatus, error) {
	if caseID == "" {
		return SignoffPending, ErrEmptyCaseID
	}
	return SignoffPending, nil
}

// CanFinalize is the hard gate requiring an approved human sign-off
// before a case's reasoning output may be finalized (e.g. exported,
// filed, or otherwise treated as complete). It returns false and a
// descriptive error for any status other than SignoffApproved,
// including when gate itself errors.
//
// CanFinalize is distinct from, and does not replace,
// packages/treevalidation.CanFinalize: that gate blocks on tree
// STRUCTURAL integrity (critical validation findings); this gate blocks
// on human sign-off STATE. A caller that needs both guarantees calls
// both gates — this package does not import treevalidation, and
// treevalidation does not import this package.
func CanFinalize(ctx context.Context, caseID string, gate SignoffGate) (bool, error) {
	if caseID == "" {
		return false, ErrEmptyCaseID
	}
	if gate == nil {
		return false, ErrNilSignoffGate
	}

	status, err := gate.Status(ctx, caseID)
	if err != nil {
		return false, fmt.Errorf("guardrail: signoff status lookup failed for case %q: %w", caseID, err)
	}
	if status != SignoffApproved {
		return false, fmt.Errorf("%w: case %q has status %q", ErrSignoffNotApproved, caseID, status)
	}
	return true, nil
}
