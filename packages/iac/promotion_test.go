package iac

import (
	"errors"
	"testing"
)

func passingChecklist() Checklist {
	return Checklist{Checks: []Check{
		{Kind: CheckKindHealthEndpoint, Name: "health", Run: func() error { return nil }},
	}}
}

func failingChecklist() Checklist {
	return Checklist{Checks: []Check{
		{Kind: CheckKindHealthEndpoint, Name: "health", Run: func() error { return errors.New("unreachable") }},
	}}
}

func TestStageIsValidAndTerminal(t *testing.T) {
	if !StageDev.IsValid() || !StageStaging.IsValid() || !StageProd.IsValid() {
		t.Fatal("expected all three named stages to be valid")
	}
	if Stage("bogus").IsValid() {
		t.Fatal("expected bogus stage to be invalid")
	}
	if StageDev.IsTerminal() || StageStaging.IsTerminal() {
		t.Fatal("expected Dev and Staging to not be terminal")
	}
	if !StageProd.IsTerminal() {
		t.Fatal("expected Prod to be terminal")
	}
}

func TestStageNext(t *testing.T) {
	next, ok := StageDev.Next()
	if !ok || next != StageStaging {
		t.Fatalf("Dev.Next() = (%v, %v), want (StageStaging, true)", next, ok)
	}
	next, ok = StageStaging.Next()
	if !ok || next != StageProd {
		t.Fatalf("Staging.Next() = (%v, %v), want (StageProd, true)", next, ok)
	}
	if _, ok := StageProd.Next(); ok {
		t.Fatal("Prod.Next() should report false (terminal)")
	}
}

func TestNewPromotionPipeline(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.CurrentStage != StageDev {
		t.Errorf("new pipeline should start at StageDev, got %v", p.CurrentStage)
	}
	if len(p.History) != 1 || p.History[0] != StageDev {
		t.Errorf("expected history [dev], got %v", p.History)
	}

	if _, err := NewPromotionPipeline(""); !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("empty deployment id: got %v, want ErrEmptyDeploymentID", err)
	}
}

// TestPromote_BlockedWithoutVerification is the exact assertion the
// brief calls for: promotion to Prod (and to any stage) is blocked
// without a passing verification of the CURRENT stage on file.
func TestPromote_BlockedWithoutVerification(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No verification recorded at all yet.
	if err := p.Promote(); !errors.Is(err, ErrStageNotVerified) {
		t.Fatalf("Promote with no verification: got %v, want ErrStageNotVerified", err)
	}
	if p.CurrentStage != StageDev {
		t.Fatalf("pipeline should not have advanced, still at %v", p.CurrentStage)
	}
}

func TestPromote_BlockedByFailingVerification(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := RunDeploymentVerification("deployment-1", StageDev, failingChecklist())
	if err != nil {
		t.Fatalf("unexpected error running verification: %v", err)
	}
	if report.Passed() {
		t.Fatal("expected failing checklist to produce a failing report")
	}
	if err := p.RecordVerification(report); err != nil {
		t.Fatalf("unexpected error recording verification: %v", err)
	}

	if err := p.Promote(); !errors.Is(err, ErrStageNotVerified) {
		t.Fatalf("Promote with failing verification: got %v, want ErrStageNotVerified", err)
	}
	if p.CurrentStage != StageDev {
		t.Fatalf("pipeline should not have advanced, still at %v", p.CurrentStage)
	}
}

// TestPromote_ProdBlockedWithoutPassingStagingVerification is the
// brief's exact scenario: Dev passes and promotes to Staging, but
// Staging itself has NOT been verified -- Promote to Prod must be
// refused.
func TestPromote_ProdBlockedWithoutPassingStagingVerification(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	devReport, err := RunDeploymentVerification("deployment-1", StageDev, passingChecklist())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.RecordVerification(devReport); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.Promote(); err != nil {
		t.Fatalf("Dev -> Staging should succeed with a passing Dev verification: %v", err)
	}
	if p.CurrentStage != StageStaging {
		t.Fatalf("expected CurrentStage Staging, got %v", p.CurrentStage)
	}

	// No Staging verification recorded at all -- Prod promotion must
	// be blocked.
	if err := p.Promote(); !errors.Is(err, ErrStageNotVerified) {
		t.Fatalf("Promote to Prod without Staging verification: got %v, want ErrStageNotVerified", err)
	}
	if p.CurrentStage != StageStaging {
		t.Fatalf("pipeline should still be at Staging, got %v", p.CurrentStage)
	}
}

func TestPromote_FullHappyPath(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, stage := range []Stage{StageDev, StageStaging} {
		report, err := RunDeploymentVerification("deployment-1", stage, passingChecklist())
		if err != nil {
			t.Fatalf("unexpected error verifying %v: %v", stage, err)
		}
		if err := p.RecordVerification(report); err != nil {
			t.Fatalf("unexpected error recording %v: %v", stage, err)
		}
		if err := p.Promote(); err != nil {
			t.Fatalf("promotion from %v should succeed: %v", stage, err)
		}
	}

	if p.CurrentStage != StageProd {
		t.Fatalf("expected pipeline to reach Prod, got %v", p.CurrentStage)
	}
	wantHistory := []Stage{StageDev, StageStaging, StageProd}
	if len(p.History) != len(wantHistory) {
		t.Fatalf("history = %v, want %v", p.History, wantHistory)
	}
	for i, s := range wantHistory {
		if p.History[i] != s {
			t.Errorf("history[%d] = %v, want %v", i, p.History[i], s)
		}
	}

	// Terminal: further promotion attempts must fail even with a
	// passing Prod verification recorded.
	prodReport, err := RunDeploymentVerification("deployment-1", StageProd, passingChecklist())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.RecordVerification(prodReport); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := p.Promote(); !errors.Is(err, ErrTerminalStage) {
		t.Fatalf("Promote from terminal Prod: got %v, want ErrTerminalStage", err)
	}
}

func TestPromote_NilPipeline(t *testing.T) {
	var p *PromotionPipeline
	if err := p.Promote(); !errors.Is(err, ErrNilPipeline) {
		t.Errorf("nil pipeline Promote: got %v, want ErrNilPipeline", err)
	}
	if err := p.RecordVerification(DeploymentVerificationReport{Stage: StageDev}); !errors.Is(err, ErrNilPipeline) {
		t.Errorf("nil pipeline RecordVerification: got %v, want ErrNilPipeline", err)
	}
}

func TestRecordVerification_InvalidStage(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.RecordVerification(DeploymentVerificationReport{Stage: Stage("bogus")})
	if !errors.Is(err, ErrInvalidStage) {
		t.Errorf("got %v, want ErrInvalidStage", err)
	}
}

func TestLatestVerification_NoneRecorded(t *testing.T) {
	p, err := NewPromotionPipeline("deployment-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.LatestVerification(StageStaging); ok {
		t.Error("expected no verification recorded for Staging yet")
	}
}
