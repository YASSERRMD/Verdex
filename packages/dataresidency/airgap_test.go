package dataresidency_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/router"
)

func TestAirGappedPreset_HasStrictModeAndNoRegions(t *testing.T) {
	deploymentID := uuid.New()
	policy := dataresidency.AirGappedPreset(deploymentID)

	if !policy.StrictMode {
		t.Fatal("expected AirGappedPreset to set StrictMode")
	}
	if len(policy.AllowedRegions) != 0 {
		t.Fatalf("expected AirGappedPreset to have no allowed regions, got %v", policy.AllowedRegions)
	}
	if !dataresidency.IsAirGappedPolicy(&policy) {
		t.Fatal("expected IsAirGappedPolicy to recognize the preset")
	}
}

func TestComposeWithRouterAirGap_RequiresLocalFlag(t *testing.T) {
	deploymentID := uuid.New()
	policy := dataresidency.AirGappedPreset(deploymentID)

	err := dataresidency.ComposeWithRouterAirGap(&policy, router.RoutingPolicy{AirGappedOnly: false})
	if !errors.Is(err, dataresidency.ErrAirGappedRequiresLocalFlag) {
		t.Fatalf("expected ErrAirGappedRequiresLocalFlag, got %v", err)
	}

	if err := dataresidency.ComposeWithRouterAirGap(&policy, router.RoutingPolicy{AirGappedOnly: true}); err != nil {
		t.Fatalf("expected composition to succeed when AirGappedOnly is set: %v", err)
	}
}

func TestComposeWithRouterAirGap_NonAirGappedPolicyIsUnaffected(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}

	// A regionally-scoped (non-air-gapped) policy composes fine
	// regardless of the router flag.
	if err := dataresidency.ComposeWithRouterAirGap(policy, router.RoutingPolicy{AirGappedOnly: false}); err != nil {
		t.Fatalf("expected non-air-gapped policy to compose without the flag: %v", err)
	}
}

func TestComposeWithRouterAirGap_NilPolicy(t *testing.T) {
	if err := dataresidency.ComposeWithRouterAirGap(nil, router.RoutingPolicy{}); !errors.Is(err, dataresidency.ErrNilPolicy) {
		t.Fatalf("expected ErrNilPolicy, got %v", err)
	}
}
