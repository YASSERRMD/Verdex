package dataresidency_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
)

func TestResidencyPolicy_Validate_RequiresDeploymentID(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{AllowedRegions: []string{"eu"}}
	if err := policy.Validate(); !errors.Is(err, dataresidency.ErrEmptyDeploymentID) {
		t.Fatalf("expected ErrEmptyDeploymentID, got %v", err)
	}
}

func TestResidencyPolicy_Validate_NilPolicy(t *testing.T) {
	var policy *dataresidency.ResidencyPolicy
	if err := policy.Validate(); !errors.Is(err, dataresidency.ErrNilPolicy) {
		t.Fatalf("expected ErrNilPolicy, got %v", err)
	}
}

func TestResidencyPolicy_Validate_RejectsEmptyRegionEntry(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", ""},
	}
	if err := policy.Validate(); err == nil {
		t.Fatal("expected error for blank region entry, got nil")
	}
}

func TestResidencyPolicy_Validate_AcceptsWellFormedPolicy(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
		DataClassRules: []dataresidency.DataClassRule{{DataClass: "pii", AllowedRegions: []string{"eu"}}},
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("expected well-formed policy to validate: %v", err)
	}
}

func TestResidencyPolicy_AllowsRegion_CaseInsensitive(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"EU"},
	}
	if !policy.AllowsRegion("eu") {
		t.Fatal("expected AllowsRegion to be case-insensitive")
	}
}

func TestResidencyPolicy_AllowsRegion_EmptyListAllowsNothing(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{DeploymentID: uuid.New()}
	if policy.AllowsRegion("eu") {
		t.Fatal("expected an empty AllowedRegions list to allow nothing (fail closed)")
	}
}

func TestReport_Passed_EmptyChecksIsNotPassed(t *testing.T) {
	report := &dataresidency.Report{}
	if report.Passed() {
		t.Fatal("expected an empty Report to not be considered passed")
	}
}

func TestReport_Passed_AllChecksPassing(t *testing.T) {
	report := &dataresidency.Report{
		Checks: []dataresidency.CheckResult{
			{Kind: dataresidency.CheckStorageRegion, Passed: true},
			{Kind: dataresidency.CheckProviderRegions, Passed: true},
		},
	}
	if !report.Passed() {
		t.Fatal("expected report with all passing checks to be Passed")
	}
	if len(report.Failures()) != 0 {
		t.Fatalf("expected no failures, got %+v", report.Failures())
	}
}

func TestReport_Failures_ReturnsOnlyFailedChecks(t *testing.T) {
	report := &dataresidency.Report{
		Checks: []dataresidency.CheckResult{
			{Kind: dataresidency.CheckStorageRegion, Passed: true},
			{Kind: dataresidency.CheckProviderRegions, Passed: false},
		},
	}
	fails := report.Failures()
	if len(fails) != 1 || fails[0].Kind != dataresidency.CheckProviderRegions {
		t.Fatalf("expected exactly the CheckProviderRegions failure, got %+v", fails)
	}
}
