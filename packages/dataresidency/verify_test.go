package dataresidency_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/router"
)

func newVerifier(t *testing.T, src *fakePolicySource, dsn string, providerRegions []string, routingPolicy router.RoutingPolicy) *dataresidency.Verifier {
	t.Helper()
	live := dataresidency.LiveConfig{
		DatabaseDSN: func(context.Context) (string, error) {
			return dsn, nil
		},
		ProviderRegionsInUse: func(context.Context) ([]string, error) {
			return providerRegions, nil
		},
		RoutingPolicy: routingPolicy,
	}
	v, err := dataresidency.NewVerifier(src, live)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

func TestVerify_PassesWhenLiveConfigSatisfiesPolicy(t *testing.T) {
	deploymentID := uuid.New()
	src := newFakePolicySource()
	src.setPolicy(dataresidency.ResidencyPolicy{
		DeploymentID:   deploymentID,
		AllowedRegions: []string{"eu"},
	})
	src.setPin(dataresidency.RegionPin{
		DeploymentID: deploymentID,
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com"},
	})

	v := newVerifier(t, src, "postgres://user:pass@eu-west-1.rds.example.com:5432/verdex", []string{"eu"}, router.RoutingPolicy{})

	report, err := v.Verify(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("expected report to pass, got failures: %+v", report.Failures())
	}
}

func TestVerify_CatchesLiveStorageRegionMisconfiguration(t *testing.T) {
	deploymentID := uuid.New()
	src := newFakePolicySource()
	src.setPolicy(dataresidency.ResidencyPolicy{
		DeploymentID:   deploymentID,
		AllowedRegions: []string{"eu"},
	})
	src.setPin(dataresidency.RegionPin{
		DeploymentID: deploymentID,
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com"},
	})

	// The DB DSN actually points at a US host, disagreeing with the
	// pinned "eu" region -- a real live misconfiguration.
	v := newVerifier(t, src, "postgres://user:pass@us-east-1.rds.example.com:5432/verdex", []string{"eu"}, router.RoutingPolicy{})

	report, err := v.Verify(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.Passed() {
		t.Fatal("expected report to fail on storage region mismatch")
	}
	fails := report.Failures()
	if len(fails) != 1 || fails[0].Kind != dataresidency.CheckStorageRegion {
		t.Fatalf("expected exactly one CheckStorageRegion failure, got %+v", fails)
	}
}

func TestVerify_CatchesDisallowedProviderRegionInUse(t *testing.T) {
	deploymentID := uuid.New()
	src := newFakePolicySource()
	src.setPolicy(dataresidency.ResidencyPolicy{
		DeploymentID:   deploymentID,
		AllowedRegions: []string{"eu"},
	})
	src.setPin(dataresidency.RegionPin{
		DeploymentID: deploymentID,
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com"},
	})

	v := newVerifier(t, src, "postgres://user:pass@eu-west-1.rds.example.com:5432/verdex", []string{"eu", "cn"}, router.RoutingPolicy{})

	report, err := v.Verify(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.Passed() {
		t.Fatal("expected report to fail on disallowed provider region")
	}
	found := false
	for _, f := range report.Failures() {
		if f.Kind == dataresidency.CheckProviderRegions {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a CheckProviderRegions failure, got %+v", report.Failures())
	}
}

func TestVerify_AirGappedPresetComposesWithRouterFlag(t *testing.T) {
	deploymentID := uuid.New()
	src := newFakePolicySource()
	src.setPolicy(dataresidency.AirGappedPreset(deploymentID))
	src.setPin(dataresidency.RegionPin{
		DeploymentID: deploymentID,
		Region:       "local",
		HostPatterns: []string{"localhost"},
	})

	// Router's AirGappedOnly flag is NOT set -- inconsistent composition.
	vInconsistent := newVerifier(t, src, "postgres://user:pass@localhost:5432/verdex", nil, router.RoutingPolicy{AirGappedOnly: false})
	report, err := vInconsistent.Verify(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.Passed() {
		t.Fatal("expected air-gapped policy without router.AirGappedOnly to fail composition check")
	}

	// Router's AirGappedOnly flag IS set -- consistent composition.
	vConsistent := newVerifier(t, src, "postgres://user:pass@localhost:5432/verdex", nil, router.RoutingPolicy{AirGappedOnly: true})
	report2, err := vConsistent.Verify(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !report2.Passed() {
		t.Fatalf("expected air-gapped policy with router.AirGappedOnly set to pass, got failures: %+v", report2.Failures())
	}
}

func TestVerify_UnknownDeploymentReturnsError(t *testing.T) {
	src := newFakePolicySource()
	v := newVerifier(t, src, "postgres://user:pass@localhost:5432/verdex", nil, router.RoutingPolicy{})

	if _, err := v.Verify(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error for unconfigured deployment, got nil")
	}
}
