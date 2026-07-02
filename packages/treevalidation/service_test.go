package treevalidation

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

func TestTreeValidationService_Validate_CleanTree(t *testing.T) {
	tree := cleanTree("case-1", "us-ny")
	svc := TreeValidationService{CaseJurisdictionCode: "us-ny"}

	report, err := svc.Validate(tree)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.HasCritical() {
		t.Errorf("expected no critical findings on a clean tree, got: %s", report.Summary())
	}
	if report.CaseID != "case-1" {
		t.Errorf("expected CaseID case-1, got %q", report.CaseID)
	}
}

func TestTreeValidationService_Validate_EmptyTree(t *testing.T) {
	svc := TreeValidationService{}
	report, err := svc.Validate(treeassembly.Tree{})
	if !errors.Is(err, ErrEmptyTree) {
		t.Fatalf("expected ErrEmptyTree, got %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report even on error")
	}
}

func TestTreeValidationService_Validate_CriticalFindingBlocksGate(t *testing.T) {
	now := time.Now()
	// A conclusion with no supporting chain at all: fails traceability
	// (critical) and is also an orphan-adjacent case, but specifically we
	// want to exercise the ErrCriticalFindings path end-to-end through
	// the service.
	conclusion := irac.NewConclusionNode("conclusion-1", "case-1", "text", now, 0.9, testProvenance(), testSpan())
	tree := treeassembly.Tree{Nodes: []irac.NodeLike{conclusion}}

	svc := TreeValidationService{}
	report, err := svc.Validate(tree)
	if err == nil {
		t.Fatal("expected an error due to critical findings")
	}
	if !errors.Is(err, ErrCriticalFindings) {
		t.Errorf("expected errors.Is(err, ErrCriticalFindings), got %v", err)
	}
	if !report.HasCritical() {
		t.Error("expected report to contain a critical finding")
	}
}

func TestTreeValidationService_Validate_ComposesStructuralAndGapChecks(t *testing.T) {
	// A dangling edge is irac.ValidateTree's concern (structural), and
	// should surface as a Finding with CodeStructuralIssue without this
	// package reimplementing that check.
	now := time.Now()
	issue := irac.NewIssueNode("issue-1", "case-1", "text", now, 0.9, testProvenance(), testSpan())
	edges := []irac.Edge{
		{FromID: "rule-missing", ToID: issue.ID, Type: irac.EdgeGoverns},
	}
	tree := treeassembly.Tree{Nodes: []irac.NodeLike{issue}, Edges: edges}

	svc := TreeValidationService{}
	report, _ := svc.Validate(tree)

	foundStructural := false
	for _, f := range report.Findings {
		if f.Code == CodeStructuralIssue {
			foundStructural = true
		}
	}
	if !foundStructural {
		t.Errorf("expected a CodeStructuralIssue finding composed from irac.ValidateTree, got: %+v", report.Findings)
	}
}

func TestTreeValidationService_Validate_JurisdictionMismatchOnlyWhenConfigured(t *testing.T) {
	tree := cleanTree("case-1", "uk-eng")

	// Without a configured case jurisdiction, no jurisdiction findings.
	svc := TreeValidationService{}
	report, _ := svc.Validate(tree)
	for _, f := range report.Findings {
		if f.Code == CodeJurisdictionMismatch {
			t.Errorf("did not expect jurisdiction findings when CaseJurisdictionCode is unset")
		}
	}

	// With a configured, mismatched case jurisdiction, expect a finding.
	svc2 := TreeValidationService{CaseJurisdictionCode: "us-ny"}
	report2, err2 := svc2.Validate(tree)
	if err2 == nil {
		t.Fatal("expected an error due to jurisdiction mismatch critical finding")
	}
	found := false
	for _, f := range report2.Findings {
		if f.Code == CodeJurisdictionMismatch {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a CodeJurisdictionMismatch finding, got: %+v", report2.Findings)
	}
}
