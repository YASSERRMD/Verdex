package iac

import (
	"errors"
	"testing"
)

func TestRunDeploymentVerification_AllPass(t *testing.T) {
	checklist := Checklist{Checks: []Check{
		{Kind: CheckKindHealthEndpoint, Name: "gateway /readyz", Run: func() error { return nil }},
		{Kind: CheckKindMigrationVersion, Name: "schema migration v41", Run: func() error { return nil }},
		{Kind: CheckKindGuardrailSmokeTest, Name: "guardrail disclaimer present", Run: func() error { return nil }},
	}}

	report, err := RunDeploymentVerification("deployment-1", StageStaging, checklist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("expected all-passing checklist to produce a passing report, failures: %+v", report.Failures())
	}
	if len(report.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(report.Results))
	}
	if report.DeploymentID != "deployment-1" || report.Stage != StageStaging {
		t.Errorf("report metadata mismatch: %+v", report)
	}
}

// TestRunDeploymentVerification_MixedOutcomes uses a fake checklist
// (task 8's explicit requirement) with one failing check among
// passing ones, and asserts the aggregation correctly reports overall
// failure while still recording every individual result -- not
// stopping at the first failure.
func TestRunDeploymentVerification_MixedOutcomes(t *testing.T) {
	checklist := Checklist{Checks: []Check{
		{Kind: CheckKindHealthEndpoint, Name: "gateway /readyz", Run: func() error { return nil }},
		{Kind: CheckKindMigrationVersion, Name: "schema migration v41", Run: func() error {
			return errors.New("expected v41, found v40")
		}},
		{Kind: CheckKindGuardrailSmokeTest, Name: "guardrail disclaimer present", Run: func() error { return nil }},
	}}

	report, err := RunDeploymentVerification("deployment-1", StageProd, checklist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed() {
		t.Fatal("expected mixed-outcome checklist to produce a failing report")
	}
	if len(report.Results) != 3 {
		t.Fatalf("expected all 3 checks to have run despite one failing, got %d results", len(report.Results))
	}

	failures := report.Failures()
	if len(failures) != 1 {
		t.Fatalf("expected exactly 1 failure, got %d: %+v", len(failures), failures)
	}
	if failures[0].Name != "schema migration v41" {
		t.Errorf("unexpected failing check: %+v", failures[0])
	}
	if failures[0].Detail == "" {
		t.Error("expected failure detail to carry the underlying error message")
	}
}

func TestRunDeploymentVerification_EmptyChecklist(t *testing.T) {
	_, err := RunDeploymentVerification("deployment-1", StageDev, Checklist{})
	if !errors.Is(err, ErrEmptyChecklist) {
		t.Errorf("got %v, want ErrEmptyChecklist", err)
	}
}

func TestRunDeploymentVerification_NilCheckFunc(t *testing.T) {
	checklist := Checklist{Checks: []Check{
		{Kind: CheckKindCustom, Name: "broken check", Run: nil},
	}}
	_, err := RunDeploymentVerification("deployment-1", StageDev, checklist)
	if !errors.Is(err, ErrNilCheckFunc) {
		t.Errorf("got %v, want ErrNilCheckFunc", err)
	}
}

func TestDeploymentVerificationReport_PassedAndFailures_NilAndEmpty(t *testing.T) {
	var nilReport *DeploymentVerificationReport
	if nilReport.Passed() {
		t.Error("nil report should not report Passed")
	}
	if nilReport.Failures() != nil {
		t.Error("nil report should have nil Failures")
	}

	empty := &DeploymentVerificationReport{}
	if empty.Passed() {
		t.Error("report with no results should not report Passed")
	}
}
