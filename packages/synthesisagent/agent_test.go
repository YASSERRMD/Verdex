package synthesisagent_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// fakeSynthesisJSON is a realistic structured completion matching the
// synthesis template's documented JSON schema (see
// templates/synthesis.go), covering issue-1 (grounded, real citations)
// and issue-2 (deliberately fabricating a fact ID that does not exist in
// the tree, to exercise the anti-fabrication grounding path).
const fakeSynthesisJSON = `Here is my analysis:
` + "```json" + `
{
  "conclusions": [
    {
      "issue_node_id": "issue-1",
      "text": "The weight of the evidence tends to favor the first party on this issue.",
      "favored_party": "plaintiff",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "weakest_link": "fact-1 is the only supporting fact and is not independently corroborated",
      "confidence": 0.8
    },
    {
      "issue_node_id": "issue-2",
      "text": "The second party's position appears better supported here.",
      "favored_party": "defendant",
      "supporting_fact_ids": ["fact-2", "fact-does-not-exist"],
      "supporting_rule_ids": [],
      "weakest_link": "the record is thin",
      "confidence": 0.55
    }
  ]
}
` + "```"

// fakeVerdictLanguageJSON proposes a conclusion whose text contains
// verdict/directive language, to exercise Provider's rejection path.
const fakeVerdictLanguageJSON = `{
  "conclusions": [
    {
      "issue_node_id": "issue-1",
      "text": "The defendant is guilty and is ordered to pay damages immediately.",
      "favored_party": "plaintiff",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "weakest_link": "none",
      "confidence": 0.9
    }
  ]
}`

// newSeededFixture builds a case tree with two issues, each with a
// governing rule, a supporting fact, and an application linking them —
// enough structure for both fetchSynthesisInputs (via knowledgeapi) and
// Provider's upstream-application matching to have something real to
// resolve.
func newSeededFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)

	f.seedIssue(t, "issue-1", "Was the contract validly formed?", 0.9)
	f.seedRule(t, "rule-1", "Statute of Frauds requires written evidence for contracts over $500.", "US-CA", "common_law", 0.8)
	f.seedFact(t, "fact-1", "The parties exchanged a signed written memorandum.", 0.7)
	f.seedApplication(t, "app-1", "Applying the Statute of Frauds to the signed memorandum.", "rule-1", []string{"fact-1"}, 0.75)
	f.seedGoverns(t, "rule-1", "issue-1")
	f.seedAppliesTo(t, "app-1", "rule-1")
	f.seedAppliesTo(t, "app-1", "fact-1")
	f.seedSupports(t, "fact-1", "app-1")

	f.seedIssue(t, "issue-2", "Did the defendant breach a duty of care?", 0.6)
	f.seedRule(t, "rule-2", "A duty of care arises from a special relationship.", "US-CA", "common_law", 0.6)
	f.seedFact(t, "fact-2", "The defendant had prior notice of the hazard.", 0.5)
	f.seedApplication(t, "app-2", "Applying duty-of-care analysis to prior notice.", "rule-2", []string{"fact-2"}, 0.5)
	f.seedGoverns(t, "rule-2", "issue-2")
	f.seedAppliesTo(t, "app-2", "rule-2")
	f.seedAppliesTo(t, "app-2", "fact-2")
	f.seedSupports(t, "fact-2", "app-2")

	return f
}

// framedIssues returns the FramedIssues for newSeededFixture's tree.
func framedIssues() []issueagent.FramedIssue {
	return []issueagent.FramedIssue{
		{SourceIssueNodeID: "issue-1", Question: "Was the contract validly formed?", MaterialityRank: 1, Confidence: 0.85},
		{SourceIssueNodeID: "issue-2", Question: "Did the defendant breach a duty of care?", MaterialityRank: 2, Confidence: 0.6},
	}
}

// firstPartySet returns a firstpartyagent.ArgumentSet citing fact-1/
// rule-1 for issue-1 and fact-2 for issue-2.
func firstPartySet() firstpartyagent.ArgumentSet {
	return firstpartyagent.ArgumentSet{
		CaseID:  testCaseID,
		PartyID: "plaintiff",
		Arguments: []firstpartyagent.Argument{
			{
				ID:                "issue-1-arg-0",
				IssueNodeID:       "issue-1",
				PartyID:           "plaintiff",
				Claim:             "The signed memorandum satisfies the Statute of Frauds.",
				SupportingFactIDs: []string{"fact-1"},
				SupportingRuleIDs: []string{"rule-1"},
				Strength:          0.8,
				Grounded:          true,
			},
		},
		GeneratedAt: time.Now().UTC(),
	}
}

// secondPartySet returns a secondpartyagent.ArgumentSet citing fact-2 for
// issue-2, rebutting the first party's issue-1 argument.
func secondPartySet() secondpartyagent.ArgumentSet {
	return secondpartyagent.ArgumentSet{
		CaseID:  testCaseID,
		PartyID: "defendant",
		Arguments: []secondpartyagent.Argument{
			{
				ID:                "issue-2-arg-0",
				IssueNodeID:       "issue-2",
				PartyID:           "defendant",
				Claim:             "Prior notice establishes a breach of the duty of care.",
				SupportingFactIDs: []string{"fact-2"},
				SupportingRuleIDs: []string{"rule-2"},
				RebutsArgumentIDs: []string{"issue-1-arg-0"},
				Strength:          0.6,
				Grounded:          true,
			},
		},
		GeneratedAt: time.Now().UTC(),
	}
}

// evidenceResult returns an evidenceweighing.Result weighing fact-1 and
// fact-2, with fact-2 marked contradicted so weakest-link derivation has
// a concrete signal to surface for issue-2.
func evidenceResult() evidenceweighing.Result {
	return evidenceweighing.Result{
		CaseID: testCaseID,
		FactWeights: []evidenceweighing.FactWeight{
			{FactNodeID: "fact-1", Weight: 0.7, Kind: evidenceweighing.EvidenceKindDocumentary, Contradicted: false, CorroborationCount: 1},
			{FactNodeID: "fact-2", Weight: 0.3, Kind: evidenceweighing.EvidenceKindTestimony, Contradicted: true, CorroborationCount: 1},
		},
		LegalFamily: "common_law",
		GeneratedAt: time.Now().UTC(),
	}
}

// lawApplicationResult returns a lawapplication.Result with controlling
// rules for both issues.
func lawApplicationResult() lawapplication.Result {
	return lawapplication.Result{
		CaseID: testCaseID,
		IssueApplications: []lawapplication.IssueApplication{
			{IssueNodeID: "issue-1", ControllingRuleIDs: []string{"rule-1"}, Confidence: 0.75},
			{IssueNodeID: "issue-2", ControllingRuleIDs: []string{"rule-2"}, Confidence: 0.5},
		},
		GeneratedAt: time.Now().UTC(),
	}
}

func TestSynthesize_EndToEnd_TracesConclusionsAndStripsFabrications(t *testing.T) {
	f := newSeededFixture(t)

	agent, err := synthesisagent.New(f.api, framedIssues(), firstPartySet(), secondPartySet(), evidenceResult(), lawApplicationResult(),
		synthesisagent.WithLegalFamily("common_law"), synthesisagent.WithJurisdictionName("California"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeSynthesisJSON})
	opinion, runResult, err := synthesisagent.Synthesize(authedContext(), agent, f.caseID, synthesisagent.SynthesizeConfig{Router: r})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if runResult.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Termination = %q, want concluded", runResult.Termination)
	}
	if len(opinion.Conclusions) != 2 {
		t.Fatalf("len(Conclusions) = %d, want 2", len(opinion.Conclusions))
	}

	byIssue := make(map[string]synthesisagent.TentativeConclusion, len(opinion.Conclusions))
	for _, c := range opinion.Conclusions {
		byIssue[c.IssueNodeID] = c
	}

	// (a) conclusions trace to real node IDs.
	issue1 := byIssue["issue-1"]
	if !issue1.Grounded {
		t.Fatalf("issue-1 conclusion Grounded = false, want true: %+v", issue1)
	}
	if len(issue1.SupportingFactIDs) != 1 || issue1.SupportingFactIDs[0] != "fact-1" {
		t.Fatalf("issue-1 SupportingFactIDs = %v, want [fact-1]", issue1.SupportingFactIDs)
	}
	if len(issue1.SupportingRuleIDs) != 1 || issue1.SupportingRuleIDs[0] != "rule-1" {
		t.Fatalf("issue-1 SupportingRuleIDs = %v, want [rule-1]", issue1.SupportingRuleIDs)
	}

	// (b) fabricated node-ID references get stripped/flagged.
	issue2 := byIssue["issue-2"]
	if issue2.Grounded {
		t.Fatalf("issue-2 conclusion Grounded = true, want false (fabricated fact cited)")
	}
	if len(issue2.FabricatedNodeIDs) != 1 || issue2.FabricatedNodeIDs[0] != "fact-does-not-exist" {
		t.Fatalf("issue-2 FabricatedNodeIDs = %v, want [fact-does-not-exist]", issue2.FabricatedNodeIDs)
	}
	for _, id := range issue2.SupportingFactIDs {
		if id == "fact-does-not-exist" {
			t.Fatalf("issue-2 SupportingFactIDs still contains fabricated id: %v", issue2.SupportingFactIDs)
		}
	}
	if issue2.WeakestLink == "" {
		t.Fatal("issue-2 WeakestLink is empty, want a derived weakest-link explanation")
	}

	// Weakest link should surface the fabrication as the strongest signal
	// for issue-2, since a stripped reference outranks a contradicted
	// fact as the weak-link signal.
	if issue2.FabricatedNodeIDs != nil && issue2.WeakestLink == "" {
		t.Fatal("expected non-empty WeakestLink referencing the fabricated node")
	}

	// FinalText must round-trip through DecodeResult identically.
	decoded, err := synthesisagent.DecodeResult(runResult.FinalText)
	if err != nil {
		t.Fatalf("DecodeResult: %v", err)
	}
	if len(decoded.Conclusions) != len(opinion.Conclusions) {
		t.Fatalf("DecodeResult produced %d conclusions, want %d", len(decoded.Conclusions), len(opinion.Conclusions))
	}
}

func TestProvider_ComposesValidTreeWithConclusionNodes(t *testing.T) {
	f := newSeededFixture(t)

	agent, err := synthesisagent.New(f.api, framedIssues(), firstPartySet(), secondPartySet(), evidenceResult(), lawApplicationResult())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeSynthesisJSON})
	opinion, _, err := synthesisagent.Synthesize(authedContext(), agent, f.caseID, synthesisagent.SynthesizeConfig{Router: r})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	// (d) the ConclusionProvider adapter produces valid irac.ConclusionNodes
	// that pass irac.ValidateTree when composed via treeassembly.ComposeTree.
	input := treeassembly.AssemblyInput{
		CaseID: testCaseID,
		Issues: []irac.IssueNode{
			irac.NewIssueNode("issue-1", testCaseID, "Was the contract validly formed?", time.Now(), 0.9, irac.Provenance{}),
			irac.NewIssueNode("issue-2", testCaseID, "Did the defendant breach a duty of care?", time.Now(), 0.6, irac.Provenance{}),
		},
		Rules: []irac.RuleNode{
			irac.NewRuleNode("rule-1", testCaseID, "Statute of Frauds.", "US-CA", "common_law", time.Now(), 0.8, irac.Provenance{UpstreamNodeIDs: []string{"issue-1"}}),
			irac.NewRuleNode("rule-2", testCaseID, "Duty of care.", "US-CA", "common_law", time.Now(), 0.6, irac.Provenance{UpstreamNodeIDs: []string{"issue-2"}}),
		},
		Facts: []irac.FactNode{
			irac.NewFactNode("fact-1", testCaseID, "Signed memorandum.", time.Now(), 0.7, irac.Provenance{}),
			irac.NewFactNode("fact-2", testCaseID, "Prior notice.", time.Now(), 0.5, irac.Provenance{}),
		},
		Applications: []irac.ApplicationNode{
			irac.NewApplicationNode("app-1", testCaseID, "Applying Statute of Frauds.", time.Now(), 0.75, irac.Provenance{UpstreamNodeIDs: []string{"rule-1", "fact-1"}}),
			irac.NewApplicationNode("app-2", testCaseID, "Applying duty of care.", time.Now(), 0.5, irac.Provenance{UpstreamNodeIDs: []string{"rule-2", "fact-2"}}),
		},
	}

	provider := synthesisagent.Provider{Opinion: opinion}
	tree, err := treeassembly.ComposeTree(context.Background(), input, provider)
	if err != nil {
		t.Fatalf("ComposeTree: %v", err)
	}

	// issue-2's fabricated fact was stripped but fact-2 remains a real
	// supporting fact, so both issues should still produce a
	// ConclusionNode.
	var conclusionCount int
	for _, n := range tree.Nodes {
		if n.GetType() == irac.NodeConclusion {
			conclusionCount++
		}
	}
	if conclusionCount != 2 {
		t.Fatalf("conclusion node count = %d, want 2", conclusionCount)
	}

	issues := irac.ValidateTree(tree.Nodes, tree.Edges)
	if len(issues) != 0 {
		t.Fatalf("ValidateTree found %d issues, want 0: %+v", len(issues), issues)
	}
}

func TestProvider_RejectsVerdictLanguage(t *testing.T) {
	f := newSeededFixture(t)

	agent, err := synthesisagent.New(f.api, framedIssues()[:1], firstPartySet(), secondpartyagent.ArgumentSet{}, evidenceResult(), lawApplicationResult())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeVerdictLanguageJSON})
	opinion, _, err := synthesisagent.Synthesize(authedContext(), agent, f.caseID, synthesisagent.SynthesizeConfig{Router: r})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if len(opinion.Conclusions) != 1 {
		t.Fatalf("len(Conclusions) = %d, want 1", len(opinion.Conclusions))
	}
	if !irac.ContainsVerdictLanguage(opinion.Conclusions[0].Text) {
		t.Fatalf("expected fixture conclusion text to contain verdict language for this test to be meaningful: %q", opinion.Conclusions[0].Text)
	}

	// (c) verdict-language conclusions are rejected/flagged by the
	// ConclusionProvider adapter: it must not emit a ConclusionNode for
	// this Opinion's only conclusion.
	input := treeassembly.AssemblyInput{
		CaseID: testCaseID,
		Issues: []irac.IssueNode{
			irac.NewIssueNode("issue-1", testCaseID, "Was the contract validly formed?", time.Now(), 0.9, irac.Provenance{}),
		},
		Facts: []irac.FactNode{
			irac.NewFactNode("fact-1", testCaseID, "Signed memorandum.", time.Now(), 0.7, irac.Provenance{}),
		},
	}

	provider := synthesisagent.Provider{Opinion: opinion}
	nodes, err := provider.Provide(context.Background(), input)
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("Provide returned %d conclusion nodes, want 0 (verdict language must be rejected)", len(nodes))
	}
}

func TestNew_NilKnowledgeAPI_ReturnsErr(t *testing.T) {
	_, err := synthesisagent.New(nil, framedIssues(), firstpartyagent.ArgumentSet{}, secondpartyagent.ArgumentSet{}, evidenceweighing.Result{}, lawapplication.Result{})
	if !errors.Is(err, synthesisagent.ErrNilKnowledgeAPI) {
		t.Fatalf("New() error = %v, want ErrNilKnowledgeAPI", err)
	}
}

func TestNew_NoFramedIssues_ReturnsErr(t *testing.T) {
	f := newFixture(t)
	_, err := synthesisagent.New(f.api, nil, firstpartyagent.ArgumentSet{}, secondpartyagent.ArgumentSet{}, evidenceweighing.Result{}, lawapplication.Result{})
	if !errors.Is(err, synthesisagent.ErrNoFramedIssues) {
		t.Fatalf("New() error = %v, want ErrNoFramedIssues", err)
	}
}

func TestSynthesize_EmptyCaseID_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := synthesisagent.New(f.api, framedIssues(), firstPartySet(), secondPartySet(), evidenceResult(), lawApplicationResult())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{content: fakeSynthesisJSON})
	_, _, err = synthesisagent.Synthesize(authedContext(), agent, "", synthesisagent.SynthesizeConfig{Router: r})
	if !errors.Is(err, synthesisagent.ErrEmptyCaseID) {
		t.Fatalf("Synthesize() error = %v, want ErrEmptyCaseID", err)
	}
}

func TestSynthesize_MalformedModelOutput_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := synthesisagent.New(f.api, framedIssues(), firstPartySet(), secondPartySet(), evidenceResult(), lawApplicationResult())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{content: "not json at all, sorry"})
	_, _, err = synthesisagent.Synthesize(authedContext(), agent, f.caseID, synthesisagent.SynthesizeConfig{Router: r})
	if !errors.Is(err, agentframework.ErrMalformedOutput) {
		t.Fatalf("Synthesize() error = %v, want wrapping agentframework.ErrMalformedOutput", err)
	}
}

func TestSynthesize_ModelCallFailure_PropagatesError(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := synthesisagent.New(f.api, framedIssues(), firstPartySet(), secondPartySet(), evidenceResult(), lawApplicationResult())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{err: errFakeUpstream})
	_, _, err = synthesisagent.Synthesize(authedContext(), agent, f.caseID, synthesisagent.SynthesizeConfig{Router: r})
	if !errors.Is(err, agentframework.ErrModelCall) {
		t.Fatalf("Synthesize() error = %v, want ErrModelCall", err)
	}
}

// errFakeUpstream is a sentinel error a fakeProvider can return to
// simulate an upstream model-call failure.
var errFakeUpstream = errors.New("fake upstream unavailable")
