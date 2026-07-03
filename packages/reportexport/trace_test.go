package reportexport_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
	"github.com/YASSERRMD/verdex/packages/reasoningtrace"
	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func newTestTrace(caseID string) reasoningtrace.Trace {
	return reasoningtrace.Trace{
		CaseID:      caseID,
		Narrative:   "UNIQUENARRATIVETEXT3344 walks through the synthesis stage.",
		GeneratedAt: time.Now().UTC(),
		Segments: []reasoningtrace.NarrativeSegment{
			{Stage: reasoningorchestration.StageSynthesis, Text: "Synthesis narrated here.", RelatedNodeIDs: []string{"issue-1"}},
		},
		AuthorityTrails: []reasoningtrace.AuthorityTrail{
			{
				IssueNodeID:       "issue-1",
				SupportingFactIDs: []string{"fact-1"},
				Citations: []reasoningtrace.CitationTrail{
					{RuleID: "rule-1", Citation: "[2021] UKSC 5", Verified: true, Resolved: true},
				},
			},
		},
	}
}

func TestWithTrace_PopulatesAppendixFromTraceExportMarkdown(t *testing.T) {
	trace := newTestTrace("case-1")

	input, err := reportexport.WithTrace(reportexport.AssembleInput{}, trace)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}

	if !strings.Contains(input.TraceAppendix, "UNIQUENARRATIVETEXT3344") {
		t.Errorf("TraceAppendix missing narrative content: %q", input.TraceAppendix)
	}
	// reasoningtrace.ExportMarkdown's own heading text, proving this
	// package embeds that function's output rather than re-deriving
	// its own narrative.
	if !strings.Contains(input.TraceAppendix, "## Narrative") {
		t.Errorf("TraceAppendix does not look like reasoningtrace.ExportMarkdown output: %q", input.TraceAppendix)
	}
}

func TestWithTrace_PopulatesAuthorityTrailsByIssue(t *testing.T) {
	trace := newTestTrace("case-1")

	input, err := reportexport.WithTrace(reportexport.AssembleInput{}, trace)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}

	cites, ok := input.AuthorityTrailsByIssue["issue-1"]
	if !ok {
		t.Fatalf("AuthorityTrailsByIssue missing issue-1")
	}
	if len(cites) != 1 {
		t.Fatalf("len(cites) = %d, want 1", len(cites))
	}
	if cites[0].RuleID != "rule-1" {
		t.Errorf("RuleID = %q, want rule-1", cites[0].RuleID)
	}
	if cites[0].FormatInput.RawCitation != "[2021] UKSC 5" {
		t.Errorf("RawCitation = %q, want [2021] UKSC 5", cites[0].FormatInput.RawCitation)
	}
	if !cites[0].Verified || !cites[0].Resolved {
		t.Errorf("cites[0] = %+v, want Verified=true Resolved=true", cites[0])
	}
}

func TestAssemble_WithTraceAppendixEndToEnd(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Analysis text.")
	trace := newTestTrace(c.ID.String())

	input, err := reportexport.WithTrace(reportexport.AssembleInput{JurisdictionKey: "common_law"}, trace)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}

	report, err := reportexport.Assemble(c, opinion, input)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	if !strings.Contains(report.TraceAppendix, "UNIQUENARRATIVETEXT3344") {
		t.Errorf("Report.TraceAppendix missing narrative content")
	}

	md, err := reportexport.RenderMarkdown(report)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(md, "UNIQUENARRATIVETEXT3344") {
		t.Errorf("Rendered Markdown missing trace appendix content")
	}
}
