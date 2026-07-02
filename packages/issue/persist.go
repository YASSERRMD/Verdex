package issue

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// defaultGeneratedBy is the irac.Provenance.GeneratedBy label attached to
// every irac.IssueNode this package persists, identifying this package's
// extraction pipeline as the generating process (mirroring
// packages/irac/provenance.go's GeneratedBy convention, e.g.
// "irac-issue-extractor-v1").
const defaultGeneratedBy = "verdex-issue-extractor-v1"

// ToIssueNode converts a CandidateIssue into an irac.IssueNode via
// irac.NewIssueNode, stamping caseID, createdAt, and a Provenance
// identifying this package as the generating process.
//
// upstreamNodeIDs is forwarded to irac.Provenance.UpstreamNodeIDs as-is
// (e.g. the FactNode/segment IDs an IssueLink associated with this
// candidate — see link.go); pass nil when there is nothing upstream to
// record.
func ToIssueNode(candidate CandidateIssue, caseID string, createdAt time.Time, upstreamNodeIDs []string) irac.IssueNode {
	provenance := irac.Provenance{
		GeneratedBy:     defaultGeneratedBy,
		GeneratedAt:     createdAt,
		UpstreamNodeIDs: upstreamNodeIDs,
	}
	return irac.NewIssueNode(
		candidate.ID,
		caseID,
		candidate.Text,
		createdAt,
		candidate.Confidence,
		provenance,
		candidate.SourceSpans...,
	)
}

// PersistIssues converts every CandidateIssue in issues to an
// irac.IssueNode (via ToIssueNode) and persists each via
// store.CreateNode, returning the persisted nodes in the same order as
// issues.
//
// linksByIndex optionally supplies the IssueLink for each issue, keyed by
// its index in issues (see LinkIssues), so each persisted node's
// Provenance.UpstreamNodeIDs records the fact/segment IDs it was linked
// to. A nil or incomplete map is fine; issues with no matching link get
// no UpstreamNodeIDs.
//
// irac's edge-constraint table (packages/irac/edge.go) has no legal edge
// whose source and target are both NodeIssue, and no RuleNode exists yet
// at this stage of the pipeline (RuleNodes are a later phase's concern),
// so PersistIssues persists nodes only — no edges. A future phase that
// produces RuleNodes is expected to create the Rule--governs-->Issue
// edges once both endpoints exist.
//
// Returns ErrPersistFailed (wrapping the underlying store error) if any
// CreateNode call fails; nodes already persisted before the failing call
// are not rolled back.
func PersistIssues(ctx context.Context, store graph.GraphStore, issues []CandidateIssue, caseID string, createdAt time.Time, linksByIndex map[int]IssueLink) ([]irac.IssueNode, error) {
	nodes := make([]irac.IssueNode, 0, len(issues))
	for i, candidate := range issues {
		var upstream []string
		if link, ok := linksByIndex[i]; ok {
			upstream = link.FactIDs
		}

		node := ToIssueNode(candidate, caseID, createdAt, upstream)
		if err := store.CreateNode(ctx, node.Node); err != nil {
			return nodes, wrapPersistError(err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// wrapPersistError wraps err with ErrPersistFailed so callers can test
// errors.Is(err, ErrPersistFailed) regardless of the underlying
// graph.GraphStore implementation's own error value.
func wrapPersistError(err error) error {
	return &persistError{underlying: err}
}

// persistError implements error and errors.Unwrap so both ErrPersistFailed
// and the underlying store error can be matched via errors.Is.
type persistError struct {
	underlying error
}

func (e *persistError) Error() string {
	return ErrPersistFailed.Error() + ": " + e.underlying.Error()
}

func (e *persistError) Unwrap() []error {
	return []error{ErrPersistFailed, e.underlying}
}
