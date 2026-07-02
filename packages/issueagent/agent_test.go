package issueagent_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/issueagent"
)

// fakeFramingJSON is a realistic structured completion matching the
// issue-framing template's documented JSON schema (see
// templates/issue_framing.go), covering both seeded issues.
const fakeFramingJSON = `Here is my analysis:
` + "```json" + `
{
  "framed_issues": [
    {
      "source_issue_node_id": "issue-1",
      "materiality_score": 0.9,
      "governing_questions": ["Was the contract validly formed under the Statute of Frauds?"],
      "ambiguities": [],
      "confidence": 0.85
    },
    {
      "source_issue_node_id": "issue-2",
      "materiality_score": 0.3,
      "governing_questions": ["Did the defendant breach a duty of care?"],
      "ambiguities": ["facts about causation are thin"],
      "confidence": 0.5
    }
  ]
}
` + "```"

func newSeededFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.seedIssue(t, "issue-1", "Was the contract validly formed?", 0.9)
	f.seedRule(t, "rule-1", "Statute of Frauds requires written evidence for contracts over $500.", "US-CA", "common_law", 0.8)
	f.seedGoverns(t, "rule-1", "issue-1")

	f.seedIssue(t, "issue-2", "Did the defendant breach a duty of care?", 0.3)
	return f
}

func TestAnalyze_EndToEnd_RanksAndFramesIssues(t *testing.T) {
	f := newSeededFixture(t)

	agent, err := issueagent.New(f.api, issueagent.WithJurisdictionName("California"), issueagent.WithLegalFamily("common_law"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeFramingJSON})

	result, runResult, err := issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if runResult.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Termination = %q, want concluded", runResult.Termination)
	}
	if result.CaseID != f.caseID {
		t.Fatalf("CaseID = %q, want %q", result.CaseID, f.caseID)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("len(Issues) = %d, want 2", len(result.Issues))
	}

	// issue-1 has a governing rule and a higher model materiality_score,
	// so it must rank strictly ahead of issue-2 (which has no governing
	// rule and a lower score).
	if result.Issues[0].SourceIssueNodeID != "issue-1" {
		t.Fatalf("Issues[0].SourceIssueNodeID = %q, want issue-1", result.Issues[0].SourceIssueNodeID)
	}
	if result.Issues[0].MaterialityRank != 1 {
		t.Fatalf("Issues[0].MaterialityRank = %d, want 1", result.Issues[0].MaterialityRank)
	}
	if result.Issues[1].SourceIssueNodeID != "issue-2" || result.Issues[1].MaterialityRank != 2 {
		t.Fatalf("Issues[1] = %+v, want issue-2 rank 2", result.Issues[1])
	}

	first := result.Issues[0]
	if len(first.GoverningQuestions) == 0 || first.GoverningQuestions[0] == "" {
		t.Fatalf("Issues[0].GoverningQuestions is empty, want a governing question")
	}
	if first.RuleLinkageCount != 1 {
		t.Fatalf("Issues[0].RuleLinkageCount = %d, want 1", first.RuleLinkageCount)
	}
	if len(first.Ambiguities) != 0 {
		t.Fatalf("Issues[0].Ambiguities = %v, want empty (has rule linkage and high confidence)", first.Ambiguities)
	}

	second := result.Issues[1]
	if second.RuleLinkageCount != 0 {
		t.Fatalf("Issues[1].RuleLinkageCount = %d, want 0", second.RuleLinkageCount)
	}
	if len(second.Ambiguities) == 0 {
		t.Fatalf("Issues[1].Ambiguities is empty, want at least the missing-rule-linkage flag")
	}

	if result.JurisdictionCode != "" {
		t.Fatalf("JurisdictionCode = %q, want empty (WithJurisdictionCode not set)", result.JurisdictionCode)
	}
	if result.LegalFamily != "common_law" {
		t.Fatalf("LegalFamily = %q, want common_law", result.LegalFamily)
	}

	for _, fi := range result.Issues {
		if fi.Confidence < 0 || fi.Confidence > 1 {
			t.Fatalf("issue %s Confidence = %f, out of [0,1]", fi.SourceIssueNodeID, fi.Confidence)
		}
		if fi.MaterialityScore < 0 || fi.MaterialityScore > 1 {
			t.Fatalf("issue %s MaterialityScore = %f, out of [0,1]", fi.SourceIssueNodeID, fi.MaterialityScore)
		}
	}

	// FinalText must round-trip through DecodeResult identically, since
	// Analyze itself only wraps that call.
	decoded, err := issueagent.DecodeResult(runResult.FinalText)
	if err != nil {
		t.Fatalf("DecodeResult: %v", err)
	}
	if len(decoded.Issues) != len(result.Issues) {
		t.Fatalf("DecodeResult produced %d issues, want %d", len(decoded.Issues), len(result.Issues))
	}
}

func TestAnalyze_JurisdictionCodeThreadedIntoResult(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := issueagent.New(f.api, issueagent.WithJurisdictionCode("US-CA"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeFramingJSON})
	result, _, err := issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.JurisdictionCode != "US-CA" {
		t.Fatalf("JurisdictionCode = %q, want US-CA", result.JurisdictionCode)
	}
}

func TestAnalyze_NoIssueNodes_ReturnsErr(t *testing.T) {
	f := newFixture(t) // no issues seeded
	agent, err := issueagent.New(f.api)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: fakeFramingJSON})
	_, runResult, err := issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	// BuildRequest's error is classified by agentframework.Runner as
	// ErrMalformedOutput (see runner.go's runStep), which does not
	// preserve the original issueagent.ErrNoIssueNodes for errors.Is —
	// only the outer wrap is guaranteed. The underlying cause is still
	// visible in the error's message and in the Scratchpad's Step.Err for
	// a caller that wants to distinguish it.
	if !errors.Is(err, agentframework.ErrMalformedOutput) {
		t.Fatalf("Analyze() error = %v, want wrapping agentframework.ErrMalformedOutput", err)
	}
	if runResult.Termination != agentframework.TerminationError {
		t.Fatalf("Termination = %q, want error", runResult.Termination)
	}
}

func TestAnalyze_MalformedModelOutput_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := issueagent.New(f.api)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{content: "not json at all, sorry"})
	_, _, err = issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	// Interpret's parse failure is classified by agentframework.Runner as
	// ErrMalformedOutput (see runner.go's runStep); see the comment on
	// TestAnalyze_NoIssueNodes_ReturnsErr for why the more specific
	// issueagent.ErrMalformedModelOutput is not preserved through errors.Is
	// here.
	if !errors.Is(err, agentframework.ErrMalformedOutput) {
		t.Fatalf("Analyze() error = %v, want wrapping agentframework.ErrMalformedOutput", err)
	}
}

func TestAnalyze_PartialModelResponse_StillFramesEveryIssue(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := issueagent.New(f.api)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Model response covers only issue-1; issue-2 must still appear in
	// the result via heuristic-only framing.
	partial := `{"framed_issues": [{"source_issue_node_id": "issue-1", "materiality_score": 0.7, "governing_questions": ["Q?"], "confidence": 0.6}]}`
	r := newTestRouter(t, &fakeProvider{content: partial})

	result, _, err := issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("len(Issues) = %d, want 2 (issue-2 must still be framed)", len(result.Issues))
	}

	var sawIssue2 bool
	for _, fi := range result.Issues {
		if fi.SourceIssueNodeID == "issue-2" {
			sawIssue2 = true
			if fi.MaterialityScore <= 0 {
				t.Fatalf("issue-2 MaterialityScore = %f, want > 0 from heuristic floor", fi.MaterialityScore)
			}
		}
	}
	if !sawIssue2 {
		t.Fatal("issue-2 missing from result")
	}
}

func TestAnalyze_ModelCallFailure_PropagatesError(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := issueagent.New(f.api)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r := newTestRouter(t, &fakeProvider{err: errFakeUpstream})
	_, _, err = issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	if !errors.Is(err, agentframework.ErrModelCall) {
		t.Fatalf("Analyze() error = %v, want ErrModelCall", err)
	}
}

func TestAnalyze_EmptyCaseID_ReturnsErr(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := issueagent.New(f.api)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{content: fakeFramingJSON})
	_, _, err = issueagent.Analyze(authedContext(), agent, "", issueagent.AnalyzeConfig{Router: r})
	if !errors.Is(err, issueagent.ErrEmptyCaseID) {
		t.Fatalf("Analyze() error = %v, want ErrEmptyCaseID", err)
	}
}

func TestNew_NilKnowledgeAPI_ReturnsErr(t *testing.T) {
	_, err := issueagent.New(nil)
	if !errors.Is(err, issueagent.ErrNilKnowledgeAPI) {
		t.Fatalf("New() error = %v, want ErrNilKnowledgeAPI", err)
	}
}

func TestResult_JSONRoundTrip(t *testing.T) {
	f := newSeededFixture(t)
	agent, err := issueagent.New(f.api)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r := newTestRouter(t, &fakeProvider{content: fakeFramingJSON})
	result, _, err := issueagent.Analyze(authedContext(), agent, f.caseID, issueagent.AnalyzeConfig{Router: r})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded issueagent.IssueAnalysisResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(decoded.Issues) != len(result.Issues) {
		t.Fatalf("round trip changed issue count: got %d, want %d", len(decoded.Issues), len(result.Issues))
	}
}

// errFakeUpstream is a sentinel error a fakeProvider can return to
// simulate an upstream model-call failure.
var errFakeUpstream = errors.New("fake upstream unavailable")
