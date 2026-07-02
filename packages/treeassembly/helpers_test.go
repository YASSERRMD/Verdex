package treeassembly

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// testProvenance builds a minimal valid irac.Provenance referencing
// upstream, for use by every synthetic-node builder below.
func testProvenance(upstream ...string) irac.Provenance {
	return irac.Provenance{
		GeneratedBy:     "treeassembly-test",
		GeneratedAt:     time.Now(),
		UpstreamNodeIDs: upstream,
	}
}

// syntheticInput builds a minimal, structurally valid AssemblyInput for
// caseID: one Issue, one Rule governing it, one Fact, and one
// Application applying the Rule to the Fact — matching the legal edge
// triples in packages/irac/edge.go (Rule--governs-->Issue,
// Application--applies_to-->Rule, Application--applies_to-->Fact,
// Fact--supports-->Application).
func syntheticInput(caseID string) AssemblyInput {
	now := time.Now()

	issue := irac.NewIssueNode("issue-1", caseID, "Was the contract breached?", now, 0.9, testProvenance())
	rule := irac.NewRuleNode("rule-1", caseID, "A contract is breached when a party fails to perform.", "us-ny", "common_law", now, 0.9, testProvenance(issue.ID))
	fact := irac.NewFactNode("fact-1", caseID, "The seller did not deliver the goods.", now, 0.9, testProvenance())
	app := irac.NewApplicationNode("app-1", caseID, "The seller's non-delivery satisfies the breach rule.", now, 0.9, testProvenance(rule.ID, fact.ID))

	return AssemblyInput{
		CaseID:       caseID,
		Issues:       []irac.IssueNode{issue},
		Rules:        []irac.RuleNode{rule},
		Facts:        []irac.FactNode{fact},
		Applications: []irac.ApplicationNode{app},
	}
}

// fixedConclusionProvider is a ConclusionProvider test double that
// always returns the supplied conclusions and error.
type fixedConclusionProvider struct {
	conclusions []irac.ConclusionNode
	err         error
}

func (f fixedConclusionProvider) Provide(_ context.Context, _ AssemblyInput) ([]irac.ConclusionNode, error) {
	return f.conclusions, f.err
}
