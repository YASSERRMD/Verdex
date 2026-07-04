package dataresidency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
)

func TestCheckTransfer_AllowsInRegionOperation(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
	}

	if err := dataresidency.CheckTransfer(context.Background(), "eu", "eu", policy); err != nil {
		t.Fatalf("CheckTransfer same-region: %v", err)
	}
}

func TestCheckTransfer_BlocksCrossBorderToDisallowedRegion(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}

	err := dataresidency.CheckTransfer(context.Background(), "eu", "cn", policy)
	if err == nil {
		t.Fatal("expected error for disallowed destination region, got nil")
	}
	if !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected ErrRegionNotAllowed, got %v", err)
	}
}

func TestCheckTransfer_AllowsCrossRegionWithinAllowList(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
	}

	if err := dataresidency.CheckTransfer(context.Background(), "eu", "us", policy); err != nil {
		t.Fatalf("CheckTransfer within allow-list: %v", err)
	}
}

func TestCheckTransfer_StrictModeBlocksAnyCrossRegionMove(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
		StrictMode:     true,
	}

	err := dataresidency.CheckTransfer(context.Background(), "eu", "us", policy)
	if err == nil {
		t.Fatal("expected error under strict mode for cross-region move, got nil")
	}
	if !errors.Is(err, dataresidency.ErrStrictModeViolation) {
		t.Fatalf("expected ErrStrictModeViolation, got %v", err)
	}
}

func TestCheckTransfer_StrictModeAllowsSameRegion(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
		StrictMode:     true,
	}

	if err := dataresidency.CheckTransfer(context.Background(), "eu", "eu", policy); err != nil {
		t.Fatalf("CheckTransfer same-region under strict mode: %v", err)
	}
}

func TestCheckTransfer_NilPolicy(t *testing.T) {
	err := dataresidency.CheckTransfer(context.Background(), "eu", "eu", nil)
	if !errors.Is(err, dataresidency.ErrNilPolicy) {
		t.Fatalf("expected ErrNilPolicy, got %v", err)
	}
}

func TestCheckTransfer_EmptyRegionFailsClosed(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu"},
	}
	if err := dataresidency.CheckTransfer(context.Background(), "", "eu", policy); err == nil {
		t.Fatal("expected error for empty source region, got nil")
	}
	if err := dataresidency.CheckTransfer(context.Background(), "eu", "", policy); err == nil {
		t.Fatal("expected error for empty dest region, got nil")
	}
}

func TestCheckTransferForDataClass_NarrowsToDataClassRule(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
		DataClassRules: []dataresidency.DataClassRule{
			{DataClass: "pii", AllowedRegions: []string{"eu"}},
		},
	}

	// "us" is allowed at policy level but not for the "pii" data class.
	if err := dataresidency.CheckTransferForDataClass(context.Background(), "eu", "us", "pii", policy); !errors.Is(err, dataresidency.ErrRegionNotAllowed) {
		t.Fatalf("expected ErrRegionNotAllowed for pii to us, got %v", err)
	}
	if err := dataresidency.CheckTransferForDataClass(context.Background(), "eu", "eu", "pii", policy); err != nil {
		t.Fatalf("expected pii to eu to be allowed: %v", err)
	}
	// A data class with no explicit rule falls back to policy-level.
	if err := dataresidency.CheckTransferForDataClass(context.Background(), "eu", "us", "case_document", policy); err != nil {
		t.Fatalf("expected case_document to us to be allowed via fallback: %v", err)
	}
}
