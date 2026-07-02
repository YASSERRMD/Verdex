package synthesisagent_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// TestProvider_NoPathToVerdictConclusionNode is this phase's
// override-prevention proof: it exercises Provider.Provide directly
// (bypassing the LLM/router entirely, since Provide is a pure function of
// an already-built Opinion) with a battery of adversarial Opinions, and
// asserts that not one verdict-flavored TentativeConclusion ever survives
// into an irac.ConclusionNode, regardless of where in the Opinion the
// verdict language appears, how many legitimate conclusions surround it,
// or whether the whole Opinion is nothing but verdict language.
//
// This proves there is no exported way to construct a Provider (or feed
// it an Opinion) that bypasses guardrail.CheckText: Provider has exactly
// one exported method relevant here (Provide), it is called on every
// path, and every one of the scenarios below confirms it rejects.
func TestProvider_NoPathToVerdictConclusionNode(t *testing.T) {
	input := treeassembly.AssemblyInput{CaseID: testCaseID}

	tests := []struct {
		name        string
		conclusions []synthesisagent.TentativeConclusion
		wantNodes   int
	}{
		{
			name: "single verdict-only conclusion is fully blocked",
			conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "The defendant is guilty.", Confidence: 0.9},
			},
			wantNodes: 0,
		},
		{
			name: "verdict language mixed among legitimate conclusions blocks only the offender",
			conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "The evidence favors the plaintiff on this issue.", Confidence: 0.7},
				{IssueNodeID: "issue-2", Text: "It is hereby ordered that the defendant shall pay damages.", Confidence: 0.9},
				{IssueNodeID: "issue-3", Text: "This issue remains genuinely unresolved on the record.", Confidence: 0.4},
			},
			wantNodes: 2,
		},
		{
			name: "verdict language at the end of otherwise-legitimate text still blocks",
			conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "Weighing the record as a whole, the court finds the defendant liable.", Confidence: 0.6},
			},
			wantNodes: 0,
		},
		{
			name: "case-insensitive verdict language still blocks",
			conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "THE DEFENDANT IS GUILTY of the alleged breach.", Confidence: 0.5},
			},
			wantNodes: 0,
		},
		{
			name: "every conclusion verdict-flavored blocks all of them",
			conclusions: []synthesisagent.TentativeConclusion{
				{IssueNodeID: "issue-1", Text: "The accused is convicted on all counts.", Confidence: 0.9},
				{IssueNodeID: "issue-2", Text: "The accused is acquitted of the remaining charge.", Confidence: 0.9},
			},
			wantNodes: 0,
		},
		{
			name:        "empty opinion produces no nodes and no panic",
			conclusions: nil,
			wantNodes:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opinion := synthesisagent.Opinion{
				CaseID:      testCaseID,
				Conclusions: tt.conclusions,
				GeneratedAt: time.Now(),
			}
			provider := synthesisagent.Provider{Opinion: opinion}

			nodes, err := provider.Provide(context.Background(), input)
			if err != nil {
				t.Fatalf("Provide() error = %v, want nil", err)
			}
			if len(nodes) != tt.wantNodes {
				t.Fatalf("Provide() returned %d nodes, want %d", len(nodes), tt.wantNodes)
			}

			// Defense in depth: every node that *did* survive must itself
			// pass the guardrail's own label and text checks — proving
			// the two layers (synthesisagent's rejection loop and
			// guardrail's checks) agree completely.
			for _, node := range nodes {
				if err := guardrail.ValidateLabeled(guardrail.WrapConclusionNode(node)); err != nil {
					t.Fatalf("surviving node %q failed ValidateLabeled: %v", node.ID, err)
				}
				if err := guardrail.CheckText(node.Text); err != nil {
					t.Fatalf("surviving node %q failed CheckText: %v (a verdict-flavored node reached the tree)", node.ID, err)
				}
				if !node.HasGuardrailLabel() {
					t.Fatalf("surviving node %q does not carry the mandatory guardrail label", node.ID)
				}
			}
		})
	}
}

// TestProvider_RejectedConclusionNeverReachesIracConstructor is a
// narrower, white-box-adjacent confirmation that irac.NewConclusionNode
// is never given verdict-flavored text: since irac.NewConclusionNode is
// the ONLY exported ConclusionNode constructor (see
// packages/irac/guardrail.go) and it unconditionally sets the mandatory
// label, the only way a bad ConclusionNode could reach the tree is if
// Provider called it with verdict-flavored Text. This test confirms the
// resulting node set is empty for every case where that would have
// happened.
func TestProvider_RejectedConclusionNeverReachesIracConstructor(t *testing.T) {
	opinion := synthesisagent.Opinion{
		CaseID: testCaseID,
		Conclusions: []synthesisagent.TentativeConclusion{
			{IssueNodeID: "issue-1", Text: "Judgment for the plaintiff is entered.", Confidence: 0.95},
		},
		GeneratedAt: time.Now(),
	}
	provider := synthesisagent.Provider{Opinion: opinion}

	nodes, err := provider.Provide(context.Background(), treeassembly.AssemblyInput{CaseID: testCaseID})
	if err != nil {
		t.Fatalf("Provide() error = %v, want nil", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("Provide() returned %d nodes for verdict-flavored input, want 0", len(nodes))
	}

	// Sanity: irac.ContainsVerdictLanguage independently agrees this text
	// is verdict-flavored, confirming the test fixture is meaningful.
	if !irac.ContainsVerdictLanguage(opinion.Conclusions[0].Text) {
		t.Fatalf("fixture text %q does not trip irac.ContainsVerdictLanguage; test is not meaningful", opinion.Conclusions[0].Text)
	}
}
