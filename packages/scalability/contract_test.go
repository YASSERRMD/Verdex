package scalability

import (
	"errors"
	"testing"
	"time"
)

func TestContractVerifyEmptyServiceName(t *testing.T) {
	c := NewContract()
	_, err := c.Verify("", ChecklistAnswers{})
	if !errors.Is(err, ErrEmptyServiceName) {
		t.Fatalf("expected ErrEmptyServiceName, got %v", err)
	}
}

func TestContractVerifyAllUnattestedFails(t *testing.T) {
	c := NewContract()
	report, err := c.Verify("caselifecycle", ChecklistAnswers{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed {
		t.Fatal("expected Passed=false for all-zero answers")
	}
	if report.Score != 0 {
		t.Fatalf("expected Score=0, got %v", report.Score)
	}
	if len(report.Failed) != totalChecks {
		t.Fatalf("expected %d failed checks, got %d: %v", totalChecks, len(report.Failed), report.Failed)
	}
}

func TestContractVerifyAllAttestedPasses(t *testing.T) {
	c := NewContract()
	answers := ChecklistAnswers{
		NoInProcessSessionAffinity: true,
		NoLocalDiskOnlyState:       true,
		NoInMemorySingletonState:   true,
		IdempotentRetries:          true,
		ExternalizedConfiguration:  true,
		HealthCheckExposed:         true,
		GracefulShutdownHandled:    true,
	}
	report, err := c.Verify("router", answers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Passed {
		t.Fatalf("expected Passed=true, failed checks: %v", report.Failed)
	}
	if report.Score != 1 {
		t.Fatalf("expected Score=1, got %v", report.Score)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("expected no failed checks, got %v", report.Failed)
	}
	if report.ServiceName != "router" {
		t.Fatalf("expected ServiceName=router, got %q", report.ServiceName)
	}
	if report.EvaluatedAt.IsZero() {
		t.Fatal("expected EvaluatedAt to be set")
	}
}

func TestContractVerifyPartialAttestation(t *testing.T) {
	c := NewContract()
	answers := ChecklistAnswers{
		NoInProcessSessionAffinity: true,
		NoLocalDiskOnlyState:       true,
		IdempotentRetries:          true,
		// remaining four left false
	}
	report, err := c.Verify("ingestion", answers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed {
		t.Fatal("expected Passed=false for partial attestation")
	}
	wantScore := 3.0 / float64(totalChecks)
	if report.Score != wantScore {
		t.Fatalf("expected Score=%v, got %v", wantScore, report.Score)
	}
	wantFailed := []string{"NoInMemorySingletonState", "ExternalizedConfiguration", "HealthCheckExposed", "GracefulShutdownHandled"}
	if len(report.Failed) != len(wantFailed) {
		t.Fatalf("expected %d failed checks, got %d: %v", len(wantFailed), len(report.Failed), report.Failed)
	}
	for i, name := range wantFailed {
		if report.Failed[i] != name {
			t.Fatalf("expected Failed[%d]=%q, got %q", i, name, report.Failed[i])
		}
	}
}

func TestContractVerifyDeterministicClock(t *testing.T) {
	fixed := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	c := &Contract{clock: func() time.Time { return fixed }}
	report, err := c.Verify("svc", ChecklistAnswers{NoInProcessSessionAffinity: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.EvaluatedAt.Equal(fixed) {
		t.Fatalf("expected EvaluatedAt=%v, got %v", fixed, report.EvaluatedAt)
	}
}
