package irac

import "time"

// TreeRevision identifies one immutable snapshot of a case's IRAC
// reasoning tree. A tree is never mutated in place: each change (adding a
// node, re-running an Application, correcting a Conclusion) produces a
// new TreeRevision that supersedes, but does not overwrite, its parent.
// This mirrors packages/category's "human correction is first-class, not
// a patch" principle, extended to the whole tree rather than a single
// assignment.
type TreeRevision struct {
	// RevisionNumber is this revision's sequence number within its case,
	// starting at 1 and incrementing by 1 for each subsequent revision.
	RevisionNumber int `json:"revision_number"`

	// CaseID identifies the case this revision's tree belongs to.
	CaseID string `json:"case_id"`

	// CreatedAt is the timestamp this revision was created.
	CreatedAt time.Time `json:"created_at"`

	// ParentRevision is the RevisionNumber of the revision this one
	// supersedes, or nil if this is the first revision for CaseID.
	ParentRevision *int `json:"parent_revision,omitempty"`
}

// IsInitial reports whether r is the first revision in its case's
// revision sequence (i.e. it has no ParentRevision).
func (r TreeRevision) IsInitial() bool {
	return r.ParentRevision == nil
}

// NewInitialRevision constructs the first TreeRevision for caseID:
// RevisionNumber 1, no ParentRevision.
func NewInitialRevision(caseID string, createdAt time.Time) TreeRevision {
	return TreeRevision{
		RevisionNumber: 1,
		CaseID:         caseID,
		CreatedAt:      createdAt,
	}
}

// NextRevision constructs the TreeRevision that immediately follows prev
// in the same case's revision sequence: RevisionNumber prev.RevisionNumber
// + 1, ParentRevision pointing back at prev.RevisionNumber.
func NextRevision(prev TreeRevision, createdAt time.Time) TreeRevision {
	parent := prev.RevisionNumber
	return TreeRevision{
		RevisionNumber: prev.RevisionNumber + 1,
		CaseID:         prev.CaseID,
		CreatedAt:      createdAt,
		ParentRevision: &parent,
	}
}

// IsValidSuccessorOf reports whether r is a well-formed direct successor
// of prev: same CaseID, RevisionNumber exactly one greater, and
// ParentRevision pointing at prev.RevisionNumber.
func (r TreeRevision) IsValidSuccessorOf(prev TreeRevision) bool {
	if r.CaseID != prev.CaseID {
		return false
	}
	if r.RevisionNumber != prev.RevisionNumber+1 {
		return false
	}
	if r.ParentRevision == nil {
		return false
	}
	return *r.ParentRevision == prev.RevisionNumber
}
