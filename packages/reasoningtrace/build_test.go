package reasoningtrace_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
	"github.com/YASSERRMD/verdex/packages/reasoningtrace"
)

func TestBuild_EveryStageStepsAndToolCallsAppear(t *testing.T) {
	store := seededStore(t)

	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if trace.CaseID != testCaseID {
		t.Fatalf("CaseID = %q, want %q", trace.CaseID, testCaseID)
	}

	// Four LLM-backed stages, two steps each (see buildAgentResult).
	if len(trace.Steps) != 8 {
		t.Fatalf("len(Steps) = %d, want 8: %+v", len(trace.Steps), trace.Steps)
	}

	wantStages := map[reasoningorchestration.Stage]bool{
		reasoningorchestration.StageIssueFraming:         false,
		reasoningorchestration.StageFirstPartyArguments:  false,
		reasoningorchestration.StageSecondPartyArguments: false,
		reasoningorchestration.StageSynthesis:            false,
	}
	for _, step := range trace.Steps {
		if _, ok := wantStages[step.Stage]; ok {
			wantStages[step.Stage] = true
		}
	}
	for stage, seen := range wantStages {
		if !seen {
			t.Fatalf("no StageStep found for stage %q", stage)
		}
	}
}

func TestBuild_RetrievalEventsCoverEveryToolCall(t *testing.T) {
	store := seededStore(t)

	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(trace.Retrievals) != 4 {
		t.Fatalf("len(Retrievals) = %d, want 4: %+v", len(trace.Retrievals), trace.Retrievals)
	}

	wantTools := map[string]bool{
		agentframework.ToolGetNode:             false,
		agentframework.ToolSearchCaseKnowledge: false,
		agentframework.ToolResolveCitation:     false,
		agentframework.ToolValidationStatus:    false,
	}
	for _, r := range trace.Retrievals {
		if _, ok := wantTools[r.ToolName]; ok {
			wantTools[r.ToolName] = true
		}
		if r.ResultSummary == "" {
			t.Fatalf("RetrievalEvent %+v has empty ResultSummary", r)
		}
	}
	for tool, seen := range wantTools {
		if !seen {
			t.Fatalf("no RetrievalEvent found for tool %q", tool)
		}
	}
}

func TestBuild_NarrativeSegmentsLinkToRealNodeIDs(t *testing.T) {
	store := seededStore(t)

	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if trace.Narrative == "" {
		t.Fatalf("Narrative is empty")
	}
	if len(trace.Segments) != 8 {
		t.Fatalf("len(Segments) = %d, want 8 (one per completed stage): %+v", len(trace.Segments), trace.Segments)
	}

	var synthSegment *reasoningtrace.NarrativeSegment
	for i := range trace.Segments {
		if trace.Segments[i].Stage == reasoningorchestration.StageSynthesis {
			synthSegment = &trace.Segments[i]
		}
	}
	if synthSegment == nil {
		t.Fatalf("no NarrativeSegment for StageSynthesis")
	}
	found := false
	for _, id := range synthSegment.RelatedNodeIDs {
		if id == "issue-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("synthesis segment RelatedNodeIDs = %v, want to contain %q", synthSegment.RelatedNodeIDs, "issue-1")
	}

	if !strings.Contains(trace.Narrative, "synthesis stage concluded") {
		t.Fatalf("Narrative = %q, want it to mention the synthesis stage's conclusion", trace.Narrative)
	}
}

func TestBuild_AuthorityTrailsPopulated(t *testing.T) {
	store := seededStore(t)

	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(trace.AuthorityTrails) != 1 {
		t.Fatalf("len(AuthorityTrails) = %d, want 1", len(trace.AuthorityTrails))
	}
	trail := trace.AuthorityTrails[0]
	if trail.IssueNodeID != "issue-1" {
		t.Fatalf("AuthorityTrail.IssueNodeID = %q, want %q", trail.IssueNodeID, "issue-1")
	}
	if len(trail.Citations) != 1 || trail.Citations[0].RuleID != "rule-1" || !trail.Citations[0].Verified {
		t.Fatalf("AuthorityTrail.Citations = %+v, want one verified rule-1 citation", trail.Citations)
	}
}

func TestBuild_Unauthenticated_ReturnsErrUnauthenticated(t *testing.T) {
	store := seededStore(t)

	_, err := reasoningtrace.Build(unauthedContext(), testCaseID, store)
	if err == nil {
		t.Fatalf("Build with no user on context: got nil error, want ErrUnauthenticated")
	}
	if !strings.Contains(err.Error(), "unauthenticated") {
		t.Fatalf("Build error = %v, want unauthenticated", err)
	}
}

func TestBuild_Forbidden_LacksPermission(t *testing.T) {
	store := seededStore(t)

	// A user with no roles at all holds no permissions, including
	// identity.PermViewCase.
	ctx := identity.WithUser(context.Background(), newTestUser())

	_, err := reasoningtrace.Build(ctx, testCaseID, store)
	if err == nil {
		t.Fatalf("Build with no roles: got nil error, want ErrForbidden")
	}
}

func TestBuild_EmptyCaseID_ReturnsError(t *testing.T) {
	store := seededStore(t)

	_, err := reasoningtrace.Build(authedContext(), "", store)
	if err == nil {
		t.Fatalf("Build with empty case ID: got nil error")
	}
}

func TestBuild_NoRunForCase_ReturnsErrIncompleteRun(t *testing.T) {
	store := reasoningorchestration.NewInMemoryCheckpointStore()

	_, err := reasoningtrace.Build(authedContext(), "unknown-case", store)
	if err == nil {
		t.Fatalf("Build for a case with no run: got nil error")
	}
}

func TestExportJSON_RoundTrips(t *testing.T) {
	store := seededStore(t)
	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	b, err := reasoningtrace.ExportJSON(trace)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	var roundTripped reasoningtrace.Trace
	if err := json.Unmarshal(b, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if roundTripped.CaseID != trace.CaseID {
		t.Fatalf("round-tripped CaseID = %q, want %q", roundTripped.CaseID, trace.CaseID)
	}
	if len(roundTripped.Steps) != len(trace.Steps) {
		t.Fatalf("round-tripped len(Steps) = %d, want %d", len(roundTripped.Steps), len(trace.Steps))
	}
	if len(roundTripped.AuthorityTrails) != len(trace.AuthorityTrails) {
		t.Fatalf("round-tripped len(AuthorityTrails) = %d, want %d", len(roundTripped.AuthorityTrails), len(trace.AuthorityTrails))
	}
}

func TestExportMarkdown_ContainsKeySections(t *testing.T) {
	store := seededStore(t)
	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	md, err := reasoningtrace.ExportMarkdown(trace)
	if err != nil {
		t.Fatalf("ExportMarkdown: %v", err)
	}
	for _, want := range []string{"# Reasoning trace for case", "## Narrative", "## Steps", "## Retrieved nodes and citations", "## Authority trails"} {
		if !strings.Contains(md, want) {
			t.Fatalf("ExportMarkdown output missing %q:\n%s", want, md)
		}
	}
}

func TestIntegrityHash_DetectsTampering(t *testing.T) {
	store := seededStore(t)
	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	hash := reasoningtrace.IntegrityHash(trace)
	if hash == "" {
		t.Fatalf("IntegrityHash returned empty string")
	}
	if !reasoningtrace.VerifyIntegrity(trace, hash) {
		t.Fatalf("VerifyIntegrity(trace, hash) = false, want true for an untampered trace")
	}

	tampered := trace
	tampered.Narrative = trace.Narrative + " TAMPERED"
	if reasoningtrace.VerifyIntegrity(tampered, hash) {
		t.Fatalf("VerifyIntegrity(tampered, hash) = true, want false after mutating Narrative")
	}

	tamperedSteps := trace
	tamperedSteps.Steps = append([]reasoningtrace.StageStep{}, trace.Steps...)
	tamperedSteps.Steps[0].ToolCallCount = trace.Steps[0].ToolCallCount + 1
	if reasoningtrace.VerifyIntegrity(tamperedSteps, hash) {
		t.Fatalf("VerifyIntegrity after mutating a single step field = true, want false")
	}
}

func TestIntegrityHash_DeterministicForSameTrace(t *testing.T) {
	store := seededStore(t)
	trace, err := reasoningtrace.Build(authedContext(), testCaseID, store)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	h1 := reasoningtrace.IntegrityHash(trace)
	h2 := reasoningtrace.IntegrityHash(trace)
	if h1 != h2 {
		t.Fatalf("IntegrityHash not deterministic: %q != %q", h1, h2)
	}
}
