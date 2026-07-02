package secondpartyagent_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

// fakeArgumentJSON is a realistic structured completion matching the
// argument-rebuttal template's documented JSON schema, referencing only
// node IDs newSeededFixture actually seeds into the tree and the real
// first-party argument ID supplied by seededOpposingArgumentSet.
const fakeArgumentJSON = `Here is my analysis:
` + "```json" + `
{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The contract was not validly formed because the writing was never signed.",
      "supporting_fact_ids": ["fact-2"],
      "supporting_rule_ids": ["rule-1"],
      "rebuts_argument_ids": ["issue-1-arg-0"],
      "counterarguments": ["The parties may have ratified the agreement by later conduct."],
      "confidence": 0.8
    }
  ]
}
` + "```"

// fakeFabricatedArgumentJSON proposes one argument citing a node ID
// ("fact-does-not-exist") that was never seeded into the tree at all,
// alongside one real supporting fact — this must survive grounding with
// the fabricated ID stripped and Grounded=false.
const fakeFabricatedArgumentJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The contract was not validly formed.",
      "supporting_fact_ids": ["fact-2", "fact-does-not-exist"],
      "supporting_rule_ids": ["rule-1"],
      "rebuts_argument_ids": ["issue-1-arg-0"],
      "counterarguments": [],
      "confidence": 0.7
    }
  ]
}`

// fakeAllFabricatedArgumentJSON proposes an argument citing only
// fabricated IDs for issue-1, so it must be dropped entirely, leaving
// issue-1 in SkippedIssueNodeIDs.
const fakeAllFabricatedArgumentJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "A claim built on nothing real.",
      "supporting_fact_ids": ["fact-imaginary"],
      "supporting_rule_ids": ["rule-imaginary"],
      "rebuts_argument_ids": [],
      "counterarguments": [],
      "confidence": 0.9
    }
  ]
}`

// fakeFabricatedRebuttalJSON proposes an argument with real supporting
// evidence but a rebuttal target ID that does not correspond to any real
// first-party argument — this must survive grounding with the fabricated
// rebuttal ID stripped and recorded, without affecting Grounded (which
// tracks only fact/rule fabrication).
const fakeFabricatedRebuttalJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The contract was not validly formed.",
      "supporting_fact_ids": ["fact-2"],
      "supporting_rule_ids": ["rule-1"],
      "rebuts_argument_ids": ["opposing-arg-does-not-exist"],
      "counterarguments": [],
      "confidence": 0.7
    }
  ]
}`

func newSeededFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.seedIssue(t, "issue-1", "Was the contract validly formed?", 0.9)
	f.seedRule(t, "rule-1", "Statute of Frauds requires written evidence for contracts over $500.", "US-CA", "common_law", 0.8)
	f.seedGoverns(t, "rule-1", "issue-1")
	f.seedFact(t, "fact-1", "The parties signed a written memorandum on March 1.", 0.9)
	f.seedFact(t, "fact-2", "The memorandum produced at trial bears no signature of either party.", 0.85)
	f.seedApplication(t, "application-1", "Applying the Statute of Frauds to the disputed memorandum.", 0.8)
	f.seedAppliesToRule(t, "application-1", "rule-1")
	f.seedSupports(t, "fact-1", "application-1")
	f.seedSupports(t, "fact-2", "application-1")
	return f
}

func seededFramedIssues() []issueagent.FramedIssue {
	return []issueagent.FramedIssue{
		{
			SourceIssueNodeID:  "issue-1",
			Question:           "Was the contract validly formed?",
			MaterialityRank:    1,
			MaterialityScore:   0.9,
			GoverningQuestions: []string{"Was the contract validly formed under the Statute of Frauds?"},
			Confidence:         0.85,
			RuleLinkageCount:   1,
		},
	}
}

// seededOpposingArgumentSet returns a firstpartyagent.ArgumentSet with
// exactly one real Argument (ID "issue-1-arg-0"), the rebuttal target
// every fixture in this file expects the second-party agent to reference.
func seededOpposingArgumentSet() firstpartyagent.ArgumentSet {
	return firstpartyagent.ArgumentSet{
		CaseID:  testCaseID,
		PartyID: "party-plaintiff",
		Arguments: []firstpartyagent.Argument{
			{
				ID:                "issue-1-arg-0",
				IssueNodeID:       "issue-1",
				PartyID:           "party-plaintiff",
				Claim:             "The contract was validly formed because it was in writing.",
				SupportingFactIDs: []string{"fact-1"},
				SupportingRuleIDs: []string{"rule-1"},
				Counterarguments:  []string{"The writing may not have been signed by both parties."},
				Grounded:          true,
				Strength:          0.7,
			},
		},
	}
}

func TestArgue_EndToEnd_GroundedArgumentWithRebuttal(t *testing.T) {
	f := newSeededFixture(t)

	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet(),
		secondpartyagent.WithJurisdictionName("California"),
		secondpartyagent.WithLegalFamily("common_law"),
		secondpartyagent.WithPartyLabel("the defendant"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeArgumentJSON})
	result, runResult, err := secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if err != nil {
		t.Fatalf("Argue: %v", err)
	}
	if runResult.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Termination = %q, want concluded", runResult.Termination)
	}
	if result.CaseID != f.caseID {
		t.Fatalf("CaseID = %q, want %q", result.CaseID, f.caseID)
	}
	if result.PartyID != "party-defendant" {
		t.Fatalf("PartyID = %q, want party-defendant", result.PartyID)
	}
	if len(result.Arguments) != 1 {
		t.Fatalf("len(Arguments) = %d, want 1", len(result.Arguments))
	}

	arg := result.Arguments[0]
	if !arg.Grounded {
		t.Fatalf("Grounded = false, want true (all cited IDs are real): fabricated=%v", arg.FabricatedNodeIDs)
	}
	if len(arg.FabricatedNodeIDs) != 0 {
		t.Fatalf("FabricatedNodeIDs = %v, want empty", arg.FabricatedNodeIDs)
	}
	if len(arg.SupportingFactIDs) != 1 || arg.SupportingFactIDs[0] != "fact-2" {
		t.Fatalf("SupportingFactIDs = %v, want [fact-2]", arg.SupportingFactIDs)
	}
	if len(arg.SupportingRuleIDs) != 1 || arg.SupportingRuleIDs[0] != "rule-1" {
		t.Fatalf("SupportingRuleIDs = %v, want [rule-1]", arg.SupportingRuleIDs)
	}

	// Rebuttal linkage: the argument must reference the real first-party
	// argument ID supplied as input.
	if len(arg.RebutsArgumentIDs) != 1 || arg.RebutsArgumentIDs[0] != "issue-1-arg-0" {
		t.Fatalf("RebutsArgumentIDs = %v, want [issue-1-arg-0]", arg.RebutsArgumentIDs)
	}
	if len(arg.FabricatedRebuttalIDs) != 0 {
		t.Fatalf("FabricatedRebuttalIDs = %v, want empty", arg.FabricatedRebuttalIDs)
	}

	if len(arg.Counterarguments) == 0 {
		t.Fatal("Counterarguments is empty, want at least one anticipated rebuttal")
	}
	if len(arg.Citations) != 1 {
		t.Fatalf("len(Citations) = %d, want 1 (citation resolved for rule-1)", len(arg.Citations))
	}
	if !arg.Citations[0].Verified {
		t.Fatalf("Citations[0].Verified = false, want true (rule-1 exists in the tree)")
	}
	if arg.Strength <= 0 || arg.Strength > 1 {
		t.Fatalf("Strength = %f, want in (0,1]", arg.Strength)
	}
	if len(result.SkippedIssueNodeIDs) != 0 {
		t.Fatalf("SkippedIssueNodeIDs = %v, want empty", result.SkippedIssueNodeIDs)
	}

	// FinalText must round-trip through DecodeResult identically.
	decoded, err := secondpartyagent.DecodeResult(runResult.FinalText)
	if err != nil {
		t.Fatalf("DecodeResult: %v", err)
	}
	if len(decoded.Arguments) != len(result.Arguments) {
		t.Fatalf("DecodeResult produced %d arguments, want %d", len(decoded.Arguments), len(result.Arguments))
	}
}

func TestArgue_FabricatedRebuttalTarget_StripsRebuttalButKeepsArgument(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeFabricatedRebuttalJSON})
	result, _, err := secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if err != nil {
		t.Fatalf("Argue: %v", err)
	}
	if len(result.Arguments) != 1 {
		t.Fatalf("len(Arguments) = %d, want 1", len(result.Arguments))
	}

	arg := result.Arguments[0]
	if !arg.Grounded {
		t.Fatal("Grounded = false, want true (fabrication was in RebutsArgumentIDs, not supporting IDs)")
	}
	if len(arg.RebutsArgumentIDs) != 0 {
		t.Fatalf("RebutsArgumentIDs = %v, want empty (fabricated target stripped)", arg.RebutsArgumentIDs)
	}
	if len(arg.FabricatedRebuttalIDs) != 1 || arg.FabricatedRebuttalIDs[0] != "opposing-arg-does-not-exist" {
		t.Fatalf("FabricatedRebuttalIDs = %v, want [opposing-arg-does-not-exist]", arg.FabricatedRebuttalIDs)
	}
}

func TestArgue_PartiallyFabricatedArgument_StripsFabricationButKeepsArgument(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeFabricatedArgumentJSON})
	result, _, err := secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if err != nil {
		t.Fatalf("Argue: %v", err)
	}
	if len(result.Arguments) != 1 {
		t.Fatalf("len(Arguments) = %d, want 1", len(result.Arguments))
	}

	arg := result.Arguments[0]
	if arg.Grounded {
		t.Fatal("Grounded = true, want false (one cited fact ID was fabricated)")
	}
	if len(arg.FabricatedNodeIDs) != 1 || arg.FabricatedNodeIDs[0] != "fact-does-not-exist" {
		t.Fatalf("FabricatedNodeIDs = %v, want [fact-does-not-exist]", arg.FabricatedNodeIDs)
	}
	if len(arg.SupportingFactIDs) != 1 || arg.SupportingFactIDs[0] != "fact-2" {
		t.Fatalf("SupportingFactIDs = %v, want [fact-2] (fabricated ID stripped)", arg.SupportingFactIDs)
	}
	for _, id := range arg.SupportingFactIDs {
		if id == "fact-does-not-exist" {
			t.Fatalf("fabricated ID %q leaked into SupportingFactIDs", id)
		}
	}
}

func TestArgue_AllFabricatedArgument_SkipsIssue(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeAllFabricatedArgumentJSON})
	result, runResult, err := secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if err != nil {
		t.Fatalf("Argue: %v", err)
	}
	if runResult.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Termination = %q, want concluded", runResult.Termination)
	}
	if len(result.Arguments) != 0 {
		t.Fatalf("len(Arguments) = %d, want 0 (every cited ID was fabricated)", len(result.Arguments))
	}
	if len(result.SkippedIssueNodeIDs) != 1 || result.SkippedIssueNodeIDs[0] != "issue-1" {
		t.Fatalf("SkippedIssueNodeIDs = %v, want [issue-1]", result.SkippedIssueNodeIDs)
	}
}

func TestArgue_EmptyOpposingArgumentSet_StillConstructsAffirmativeCase(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), firstpartyagent.ArgumentSet{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// The model attempts to cite a rebuttal target even though none was
	// supplied — it must be stripped, but the argument survives on its
	// own supporting evidence.
	r := newTestRouter(t, &fakeProvider{content: fakeArgumentJSON})
	result, _, err := secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if err != nil {
		t.Fatalf("Argue: %v", err)
	}
	if len(result.Arguments) != 1 {
		t.Fatalf("len(Arguments) = %d, want 1", len(result.Arguments))
	}
	arg := result.Arguments[0]
	if len(arg.RebutsArgumentIDs) != 0 {
		t.Fatalf("RebutsArgumentIDs = %v, want empty (no opposing arguments were supplied)", arg.RebutsArgumentIDs)
	}
	if len(arg.FabricatedRebuttalIDs) != 1 || arg.FabricatedRebuttalIDs[0] != "issue-1-arg-0" {
		t.Fatalf("FabricatedRebuttalIDs = %v, want [issue-1-arg-0]", arg.FabricatedRebuttalIDs)
	}
}

func TestArgue_MalformedModelOutput_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: "not json at all, sorry"})
	_, _, err = secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if !errors.Is(err, agentframework.ErrMalformedOutput) {
		t.Fatalf("Argue() error = %v, want wrapping agentframework.ErrMalformedOutput", err)
	}
}

func TestArgue_ModelCallFailure_PropagatesError(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{err: errFakeUpstream})
	_, _, err = secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if !errors.Is(err, agentframework.ErrModelCall) {
		t.Fatalf("Argue() error = %v, want ErrModelCall", err)
	}
}

func TestArgue_EmptyCaseID_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{content: fakeArgumentJSON})
	_, _, err = secondpartyagent.Argue(authedContext(), agent, "", secondpartyagent.ArgueConfig{Router: r})
	if !errors.Is(err, secondpartyagent.ErrEmptyCaseID) {
		t.Fatalf("Argue() error = %v, want ErrEmptyCaseID", err)
	}
}

func TestNew_NilKnowledgeAPI_ReturnsErr(t *testing.T) {
	_, err := secondpartyagent.New(nil, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if !errors.Is(err, secondpartyagent.ErrNilKnowledgeAPI) {
		t.Fatalf("New() error = %v, want ErrNilKnowledgeAPI", err)
	}
}

func TestNew_EmptyPartyID_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	_, err := secondpartyagent.New(f.api, "", seededFramedIssues(), seededOpposingArgumentSet())
	if !errors.Is(err, secondpartyagent.ErrEmptyPartyID) {
		t.Fatalf("New() error = %v, want ErrEmptyPartyID", err)
	}
}

func TestNew_NoFramedIssues_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	_, err := secondpartyagent.New(f.api, "party-defendant", nil, seededOpposingArgumentSet())
	if !errors.Is(err, secondpartyagent.ErrNoFramedIssues) {
		t.Fatalf("New() error = %v, want ErrNoFramedIssues", err)
	}
}

func TestResult_JSONRoundTrip(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := secondpartyagent.New(f.api, "party-defendant", seededFramedIssues(), seededOpposingArgumentSet())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{content: fakeArgumentJSON})
	result, _, err := secondpartyagent.Argue(authedContext(), agent, f.caseID, secondpartyagent.ArgueConfig{Router: r})
	if err != nil {
		t.Fatalf("Argue: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded secondpartyagent.ArgumentSet
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(decoded.Arguments) != len(result.Arguments) {
		t.Fatalf("round trip changed argument count: got %d, want %d", len(decoded.Arguments), len(result.Arguments))
	}
}

// errFakeUpstream is a sentinel error a fakeProvider can return to
// simulate an upstream model-call failure.
var errFakeUpstream = errors.New("fake upstream unavailable")
