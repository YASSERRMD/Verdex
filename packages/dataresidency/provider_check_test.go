package dataresidency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/provider"
)

func TestCheckProviderLocality_AllowsInRegionProvider(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	cap := provider.Capability{ProviderID: "anthropic", ModelID: "claude-eu", Region: "eu"}

	if err := dataresidency.CheckProviderLocality(context.Background(), cap, policy); err != nil {
		t.Fatalf("expected eu-region provider to be allowed: %v", err)
	}
}

func TestCheckProviderLocality_RejectsDisallowedRegionProviderSelection(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	// A real provider.Capability, as router would resolve for a
	// candidate provider, declaring a region outside the policy's
	// allow-list.
	cap := provider.Capability{ProviderID: "openai", ModelID: "gpt-us", Region: "us"}

	err := dataresidency.CheckProviderLocality(context.Background(), cap, policy)
	if err == nil {
		t.Fatal("expected error rejecting a disallowed-region provider, got nil")
	}
	if !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected ErrRegionNotAllowed, got %v", err)
	}
}

func TestCheckProviderLocality_RejectsUndeclaredRegion(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	cap := provider.Capability{ProviderID: "mystery", ModelID: "unknown"}

	if err := dataresidency.CheckProviderLocality(context.Background(), cap, policy); !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected undeclared region to be rejected, got %v", err)
	}
}

func TestCheckProviderLocality_StrictModeRejectsEveryRegion(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
		StrictMode:     true,
	}
	cap := provider.Capability{ProviderID: "anthropic", ModelID: "claude-eu", Region: "eu"}

	err := dataresidency.CheckProviderLocality(context.Background(), cap, policy)
	if !errors.Is(err, dataresidency.ErrStrictModeViolation) {
		t.Fatalf("expected ErrStrictModeViolation under strict mode, got %v", err)
	}
}

func TestFilterProvidersByLocality_KeepsOnlyAllowedRegions(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	candidates := []provider.Capability{
		{ProviderID: "a", Region: "eu"},
		{ProviderID: "b", Region: "us"},
		{ProviderID: "c", Region: "eu"},
		{ProviderID: "d", Region: ""},
	}

	got := dataresidency.FilterProvidersByLocality(context.Background(), candidates, policy)
	if len(got) != 2 {
		t.Fatalf("expected 2 eu-region providers to survive filtering, got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.Region != "eu" {
			t.Fatalf("unexpected provider %q with region %q survived filtering", c.ProviderID, c.Region)
		}
	}
}

func TestFilterProvidersByLocality_NoMatchesReturnsEmpty(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	candidates := []provider.Capability{
		{ProviderID: "a", Region: "us"},
		{ProviderID: "b", Region: "cn"},
	}

	got := dataresidency.FilterProvidersByLocality(context.Background(), candidates, policy)
	if len(got) != 0 {
		t.Fatalf("expected no candidates to survive filtering, got %+v", got)
	}
}

func TestFilterProvidersByLocality_EmptyInputReturnsEmpty(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}

	got := dataresidency.FilterProvidersByLocality(context.Background(), nil, policy)
	if len(got) != 0 {
		t.Fatalf("expected empty input to produce empty output, got %+v", got)
	}
}
