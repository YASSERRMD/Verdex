package application

import (
	"strings"
	"time"
)

// PrecedentIssueLink explicitly records that a precedent-origin
// OriginatedRule was linked to an irac.IssueNode, distinct from a
// statute linkage (a statute is simply matched/applied — see match.go
// and build.go — with no separate linkage record, since a statute's
// authority does not depend on how it was previously applied to
// analogous facts the way a precedent's does). PrecedentIssueLink is a
// lightweight bookkeeping type: it does not itself create any
// irac.Edge; persist.go creates the underlying Rule--governs-->Issue
// edge regardless of Origin.
type PrecedentIssueLink struct {
	// IssueID is the irac.IssueNode.ID being linked.
	IssueID string

	// Rule is the precedent-origin OriginatedRule linked to the issue.
	Rule OriginatedRule

	// Rationale is a free-text explanation of why this precedent is
	// relevant to the issue (e.g. "same contested element: whether
	// notice was reasonably given").
	Rationale string

	// LinkedAt is when this linkage was recorded.
	LinkedAt time.Time
}

// NewPrecedentIssueLink constructs a PrecedentIssueLink for issue and
// rule. Returns ErrEmptyInput if issueID or rationale is blank, and a
// wrapped ErrIllegalEdge-adjacent validation error
// (errNotPrecedentOrigin) if rule.Origin is not OriginPrecedent — a
// PrecedentIssueLink is, by construction, only meaningful for
// precedent-origin rules (see distinguish.go's DistinguishingFact for
// the analogous OriginPrecedent-only constraint).
func NewPrecedentIssueLink(issueID string, rule OriginatedRule, rationale string, linkedAt time.Time) (PrecedentIssueLink, error) {
	if strings.TrimSpace(issueID) == "" || strings.TrimSpace(rationale) == "" {
		return PrecedentIssueLink{}, ErrEmptyInput
	}
	if rule.Origin != OriginPrecedent {
		return PrecedentIssueLink{}, errNotPrecedentOrigin
	}
	if linkedAt.IsZero() {
		linkedAt = time.Now()
	}
	return PrecedentIssueLink{
		IssueID:   issueID,
		Rule:      rule,
		Rationale: rationale,
		LinkedAt:  linkedAt,
	}, nil
}

// errNotPrecedentOrigin is returned by NewPrecedentIssueLink and
// NewDistinguishingFact when given an OriginatedRule whose Origin is not
// OriginPrecedent.
var errNotPrecedentOrigin = &originError{msg: "application: rule origin is not precedent"}

// originError implements error for origin-constraint validation
// failures raised by this file and distinguish.go.
type originError struct{ msg string }

func (e *originError) Error() string { return e.msg }
