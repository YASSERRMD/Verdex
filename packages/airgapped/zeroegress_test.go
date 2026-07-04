package airgapped_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/airgapped"
)

func TestVerifyZeroEgress_AllAllowed(t *testing.T) {
	profile := validProfile(t)
	targets := []airgapped.EgressTarget{
		{Name: "local-model", Address: "127.0.0.1:11434"},
		{Name: "lan-model", Address: "192.168.1.50:11434"},
	}

	report, err := airgapped.VerifyZeroEgress(context.Background(), profile, targets)
	if err != nil {
		t.Fatalf("VerifyZeroEgress: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("report.Passed() = false, want true; failures=%v", report.Failures())
	}
	if len(report.Checks) != 2 {
		t.Fatalf("len(Checks) = %d, want 2", len(report.Checks))
	}
}

func TestVerifyZeroEgress_DetectsDisallowedTarget(t *testing.T) {
	profile := validProfile(t)
	targets := []airgapped.EgressTarget{
		{Name: "local-model", Address: "127.0.0.1:11434"},
		{Name: "external-api", Address: "api.openai.com:443"},
	}

	report, err := airgapped.VerifyZeroEgress(context.Background(), profile, targets)
	if err != nil {
		t.Fatalf("VerifyZeroEgress: %v", err)
	}
	if report.Passed() {
		t.Fatal("report.Passed() = true, want false")
	}
	failures := report.Failures()
	if len(failures) != 1 || failures[0].Name != "external-api" {
		t.Fatalf("Failures() = %v, want one failure for external-api", failures)
	}
}

func TestVerifyZeroEgress_EmptyTargetsVacuouslyPasses(t *testing.T) {
	profile := validProfile(t)
	report, err := airgapped.VerifyZeroEgress(context.Background(), profile, nil)
	if err != nil {
		t.Fatalf("VerifyZeroEgress: %v", err)
	}
	// No checks at all is considered "not passed" per this phase's
	// fail-closed report convention, mirroring dataresidency.Report.
	if report.Passed() {
		t.Fatal("report.Passed() = true for an empty check list, want false (fail closed)")
	}
}

func TestVerifyZeroEgress_NilProfile(t *testing.T) {
	_, err := airgapped.VerifyZeroEgress(context.Background(), nil, nil)
	if !errors.Is(err, airgapped.ErrNilProfile) {
		t.Fatalf("VerifyZeroEgress(nil profile) = %v, want ErrNilProfile", err)
	}
}

func TestEgressReport_JSONRoundTrip(t *testing.T) {
	profile := validProfile(t)
	report, err := airgapped.VerifyZeroEgress(context.Background(), profile, []airgapped.EgressTarget{
		{Name: "local-model", Address: "127.0.0.1:11434"},
	})
	if err != nil {
		t.Fatalf("VerifyZeroEgress: %v", err)
	}
	roundTripJSON(t, &report)
}
