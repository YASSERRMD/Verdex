package airgapped_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/airgapped"
	"github.com/YASSERRMD/verdex/packages/provider"
)

func TestConformance_AllPass(t *testing.T) {
	profile := validProfile(t)
	reg := provider.NewRegistry()
	mustRegister(t, reg, "local:llama3", &stubProvider{id: "local:llama3"})

	input := airgapped.ConformanceInput{
		Registry: reg,
		EgressTargets: []airgapped.EgressTarget{
			{Name: "local-model", Address: "127.0.0.1:11434"},
		},
	}

	report, err := airgapped.Conformance(context.Background(), profile, input)
	if err != nil {
		t.Fatalf("Conformance: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("report.Passed() = false, want true; failures=%v", report.Failures())
	}
	if len(report.Checks) != 4 {
		t.Fatalf("len(Checks) = %d, want 4", len(report.Checks))
	}
}

func TestConformance_FailsOnInvalidProfile(t *testing.T) {
	profile := validProfile(t)
	profile.Routing.AirGappedOnly = false // breaks Validate

	report, err := airgapped.Conformance(context.Background(), profile, airgapped.ConformanceInput{})
	if err != nil {
		t.Fatalf("Conformance: %v", err)
	}
	if report.Passed() {
		t.Fatal("report.Passed() = true, want false for invalid profile")
	}
	failed := false
	for _, c := range report.Failures() {
		if c.Kind == airgapped.CheckProfileValid {
			failed = true
		}
	}
	if !failed {
		t.Fatal("expected CheckProfileValid to be among the failures")
	}
}

func TestConformance_FailsOnNonLocalProvider(t *testing.T) {
	profile := validProfile(t)
	reg := provider.NewRegistry()
	mustRegister(t, reg, "openai:gpt-4", &stubProvider{id: "openai:gpt-4"})

	report, err := airgapped.Conformance(context.Background(), profile, airgapped.ConformanceInput{Registry: reg})
	if err != nil {
		t.Fatalf("Conformance: %v", err)
	}
	if report.Passed() {
		t.Fatal("report.Passed() = true, want false for non-local provider")
	}
	found := false
	for _, c := range report.Failures() {
		if c.Kind == airgapped.CheckProviderAllowlist {
			found = true
		}
	}
	if !found {
		t.Fatal("expected CheckProviderAllowlist to be among the failures")
	}
}

func TestConformance_FailsOnZeroEgressViolation(t *testing.T) {
	profile := validProfile(t)
	input := airgapped.ConformanceInput{
		EgressTargets: []airgapped.EgressTarget{
			{Name: "external-api", Address: "api.openai.com:443"},
		},
	}

	report, err := airgapped.Conformance(context.Background(), profile, input)
	if err != nil {
		t.Fatalf("Conformance: %v", err)
	}
	if report.Passed() {
		t.Fatal("report.Passed() = true, want false for zero-egress violation")
	}
	found := false
	for _, c := range report.Failures() {
		if c.Kind == airgapped.CheckZeroEgress {
			found = true
		}
	}
	if !found {
		t.Fatal("expected CheckZeroEgress to be among the failures")
	}
}

func TestConformance_NilProfile(t *testing.T) {
	_, err := airgapped.Conformance(context.Background(), nil, airgapped.ConformanceInput{})
	if !errors.Is(err, airgapped.ErrNilProfile) {
		t.Fatalf("Conformance(nil profile) = %v, want ErrNilProfile", err)
	}
}

func TestConformance_EmptyInputStillRunsVacuousChecks(t *testing.T) {
	profile := validProfile(t)
	report, err := airgapped.Conformance(context.Background(), profile, airgapped.ConformanceInput{})
	if err != nil {
		t.Fatalf("Conformance: %v", err)
	}
	// No registry and no egress targets means those two checks pass
	// vacuously; profile validity and network policy still run for
	// real.
	if !report.Passed() {
		t.Fatalf("report.Passed() = false, want true; failures=%v", report.Failures())
	}
}

func TestConformanceReport_JSONRoundTrip(t *testing.T) {
	profile := validProfile(t)
	report, err := airgapped.Conformance(context.Background(), profile, airgapped.ConformanceInput{})
	if err != nil {
		t.Fatalf("Conformance: %v", err)
	}
	roundTripJSON(t, &report)
}
