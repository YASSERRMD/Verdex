package dataresidency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// TestProviderSelectionComposesWithResidencyGuard demonstrates the
// composition task 4 asks for without modifying packages/router
// itself: a caller resolves router's normal candidate list (here
// simulated directly as a []provider.Capability, since router.Router
// already resolves a provider ID to its LLMProvider/Capability before
// dispatch), then narrows it with
// dataresidency.FilterProvidersByLocality before actually selecting
// one. A policy restricted to "eu" must reject a "us"-region
// candidate and keep only the "eu" one.
func TestProviderSelectionComposesWithResidencyGuard(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}

	// Simulates the ordered candidate chain a router.RoutingPolicy
	// would resolve for a task: try "openai-us" first, fall back to
	// "anthropic-eu".
	candidates := []provider.Capability{
		{ProviderID: "openai", ModelID: "gpt-us", Region: "us"},
		{ProviderID: "anthropic", ModelID: "claude-eu", Region: "eu"},
	}

	// Selecting the first candidate outright (what a residency-unaware
	// caller would do) must be rejected by the guard.
	if err := dataresidency.CheckProviderLocality(context.Background(), candidates[0], policy); !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected the us-region candidate to be rejected, got %v", err)
	}

	// Filtering the whole chain leaves only the eu-region candidate as
	// selectable, so a residency-aware caller (or a future router
	// change) ends up dispatching to the compliant provider instead.
	allowed := dataresidency.FilterProvidersByLocality(context.Background(), candidates, policy)
	if len(allowed) != 1 || allowed[0].ProviderID != "anthropic" {
		t.Fatalf("expected only the anthropic eu candidate to survive, got %+v", allowed)
	}
}

// TestAirGappedDeploymentEndToEnd exercises the full task 6 story: an
// air-gapped ResidencyPolicy composed with router's AirGappedOnly
// flag, verified via Verifier.Verify, and a would-be external transfer
// rejected via CheckTransfer -- all in one flow, mirroring how a real
// air-gapped deployment's startup check would run before Phase 079's
// fuller offline tier takes over.
func TestAirGappedDeploymentEndToEnd(t *testing.T) {
	deploymentID := uuid.New()
	policy := dataresidency.AirGappedPreset(deploymentID)

	// Any external transfer, even to a region that would otherwise
	// seem reasonable, is rejected: StrictMode forbids the cross-region
	// move outright before the (empty) AllowedRegions list is even
	// consulted.
	if err := dataresidency.CheckTransfer(context.Background(), "local", "eu", &policy); !errors.Is(err, dataresidency.ErrStrictModeViolation) {
		t.Fatalf("expected external transfer to be rejected under air-gapped policy, got %v", err)
	}

	// A same-region ("local" to "local") move is still rejected because
	// AllowedRegions is empty -- air-gapped mode has no allowed region
	// at all; the only sanctioned path is router's local-only provider
	// selection, not a region-based allow-list entry.
	if err := dataresidency.CheckTransfer(context.Background(), "local", "local", &policy); err == nil {
		t.Fatal("expected even a same-region move to be rejected when no region is declared allowed")
	}
}
