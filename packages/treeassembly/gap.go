package treeassembly

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// GapKind classifies the semantic reasoning gap a Gap describes. Unlike
// irac.ValidationIssue (structural problems: dangling edges, illegal
// triples), a Gap describes a tree that is structurally valid but
// incomplete as a piece of legal reasoning — e.g. an issue nobody
// addressed.
type GapKind string

const (
	// GapUnaddressedIssue marks an IssueNode with no ApplicationNode
	// addressing it (no Application --applies_to--> Rule --governs-->
	// this Issue chain, and no direct application reference).
	GapUnaddressedIssue GapKind = "unaddressed_issue"

	// GapUnresolvedApplication marks an ApplicationNode with no
	// ConclusionNode resolving it, but only when the tree contains at
	// least one ConclusionNode overall — a tree with zero conclusions
	// (the common case before Phase 055's ConclusionProvider is wired
	// in) is not itself a gap, since conclusion synthesis is out of
	// this package's scope by design (see compose.go's ConclusionProvider
	// doc comment).
	GapUnresolvedApplication GapKind = "unresolved_application"
)

// Gap is a single semantic reasoning gap detected in an assembled Tree,
// beyond irac.ValidateTree's structural checks.
type Gap struct {
	// Kind classifies the gap.
	Kind GapKind

	// NodeID is the ID of the node the gap concerns (the unaddressed
	// IssueNode, or the unresolved ApplicationNode).
	NodeID string

	// Description is a human-readable explanation of the gap.
	Description string
}

// DetectGaps returns every semantic gap found in tree:
//
//   - an IssueNode with no ApplicationNode addressing it (directly, via
//     an Application --applies_to--> Rule --governs--> Issue chain, or
//     via an Application's Provenance.UpstreamNodeIDs referencing the
//     issue);
//   - when tree contains at least one ConclusionNode, an ApplicationNode
//     with no ConclusionNode resolving it (no Conclusion
//     --concludes_from--> Application edge).
//
// A nil tree yields no gaps. Order is deterministic: unaddressed issues
// first (input order), then unresolved applications (input order).
func DetectGaps(tree *Tree) []Gap {
	if tree == nil {
		return []Gap{}
	}

	gaps := make([]Gap, 0)

	issues, rules, applications, conclusions := partitionNodes(tree.Nodes)

	rulesGoverningIssue := make(map[string]map[string]struct{}) // issueID -> set of ruleIDs governing it
	for _, e := range tree.Edges {
		if e.Type != irac.EdgeGoverns {
			continue
		}
		if rulesGoverningIssue[e.ToID] == nil {
			rulesGoverningIssue[e.ToID] = make(map[string]struct{})
		}
		rulesGoverningIssue[e.ToID][e.FromID] = struct{}{}
	}

	rulesAppliedByApplication := make(map[string]map[string]struct{}) // applicationID -> set of ruleIDs it applies
	for _, e := range tree.Edges {
		if e.Type != irac.EdgeAppliesTo {
			continue
		}
		if _, isRule := ruleIDSet(rules)[e.ToID]; !isRule {
			continue
		}
		if rulesAppliedByApplication[e.FromID] == nil {
			rulesAppliedByApplication[e.FromID] = make(map[string]struct{})
		}
		rulesAppliedByApplication[e.FromID][e.ToID] = struct{}{}
	}

	for _, issue := range issues {
		if !issueIsAddressed(issue.ID, applications, rulesGoverningIssue, rulesAppliedByApplication) {
			gaps = append(gaps, Gap{
				Kind:        GapUnaddressedIssue,
				NodeID:      issue.ID,
				Description: fmt.Sprintf("issue %q has no application addressing it", issue.ID),
			})
		}
	}

	if len(conclusions) > 0 {
		resolvedApplications := make(map[string]struct{})
		for _, e := range tree.Edges {
			if e.Type == irac.EdgeConcludesFrom {
				resolvedApplications[e.ToID] = struct{}{}
			}
		}
		for _, app := range applications {
			if _, ok := resolvedApplications[app.ID]; !ok {
				gaps = append(gaps, Gap{
					Kind:        GapUnresolvedApplication,
					NodeID:      app.ID,
					Description: fmt.Sprintf("application %q has no conclusion resolving it", app.ID),
				})
			}
		}
	}

	return gaps
}

// issueIsAddressed reports whether at least one application addresses
// issueID, either because the application's Provenance.UpstreamNodeIDs
// directly references issueID, or because the application applies a
// rule that governs issueID.
func issueIsAddressed(issueID string, applications []irac.ApplicationNode, rulesGoverningIssue map[string]map[string]struct{}, rulesAppliedByApplication map[string]map[string]struct{}) bool {
	governingRules := rulesGoverningIssue[issueID]

	for _, app := range applications {
		for _, upID := range app.Provenance.UpstreamNodeIDs {
			if upID == issueID {
				return true
			}
		}
		for ruleID := range rulesAppliedByApplication[app.ID] {
			if _, ok := governingRules[ruleID]; ok {
				return true
			}
		}
	}
	return false
}

// ruleIDSet returns the set of rule IDs in rules.
func ruleIDSet(rules []irac.RuleNode) map[string]struct{} {
	out := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		out[r.ID] = struct{}{}
	}
	return out
}

// partitionNodes splits a heterogeneous []irac.NodeLike into its
// concrete typed slices by NodeType. Nodes whose declared GetType()
// doesn't match their concrete Go type (which should never happen for
// nodes produced by this package's own constructors) are silently
// skipped from the corresponding slice.
func partitionNodes(nodes []irac.NodeLike) (issues []irac.IssueNode, rules []irac.RuleNode, applications []irac.ApplicationNode, conclusions []irac.ConclusionNode) {
	for _, n := range nodes {
		switch v := n.(type) {
		case irac.IssueNode:
			issues = append(issues, v)
		case irac.RuleNode:
			rules = append(rules, v)
		case irac.ApplicationNode:
			applications = append(applications, v)
		case irac.ConclusionNode:
			conclusions = append(conclusions, v)
		}
	}
	return issues, rules, applications, conclusions
}
