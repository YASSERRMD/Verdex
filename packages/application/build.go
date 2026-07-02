package application

import (
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// BuildApplicationNode constructs an irac.ApplicationNode representing
// the reasoning step of applying rule to facts in order to resolve
// issue, via irac.NewApplicationNode.
//
// The node's Text summarizes which rule was applied to which facts, in a
// short deterministic template (no ML model, mirroring this package's
// other heuristics). Its Confidence starts at 0 — a placeholder later
// stages (confidence.go) overwrite once matching score and legal-family
// weighting are known; BuildApplicationNode itself has no opinion on
// confidence, since it is only responsible for shaping the node.
//
// Returns ErrEmptyInput if issue.Text or rule.Rule.Text is blank, or if
// facts is empty (an application with no supporting facts is not a
// meaningful reasoning step).
func BuildApplicationNode(issue irac.IssueNode, rule OriginatedRule, facts []irac.FactNode) (irac.ApplicationNode, error) {
	if strings.TrimSpace(issue.Text) == "" || strings.TrimSpace(rule.Rule.Text) == "" {
		return irac.ApplicationNode{}, ErrEmptyInput
	}
	if len(facts) == 0 {
		return irac.ApplicationNode{}, ErrEmptyInput
	}

	id := applicationNodeID(issue, rule)
	text := applicationText(issue, rule, facts)
	createdAt := time.Now()

	node := irac.NewApplicationNode(id, issue.CaseID, text, createdAt, 0, issue.Provenance)
	return node, nil
}

// applicationNodeID derives a stable, deterministic ID for the
// ApplicationNode built from issue and rule, so re-running the pipeline
// over the same issue/rule pair produces the same node ID (CreateNode is
// an idempotent upsert per graph.GraphStore's doc comment).
func applicationNodeID(issue irac.IssueNode, rule OriginatedRule) string {
	return "application-" + issue.ID + "-" + rule.Rule.ID
}

// applicationText renders a short deterministic summary of "rule applied
// to facts" for the ApplicationNode's Text field.
func applicationText(issue irac.IssueNode, rule OriginatedRule, facts []irac.FactNode) string {
	var b strings.Builder
	b.WriteString("Applying rule \"")
	b.WriteString(rule.Rule.Text)
	b.WriteString("\" to issue \"")
	b.WriteString(issue.Text)
	b.WriteString("\" using facts: ")
	for i, f := range facts {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(f.Text)
	}
	return b.String()
}
