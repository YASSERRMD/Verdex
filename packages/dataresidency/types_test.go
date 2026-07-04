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

func TestResidencyPolicy_AllowedRegionsFor_FallsBackWhenRuleHasNoRegions(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
		DataClassRules: []dataresidency.DataClassRule{
			// A rule naming a data class but with an empty region list
			// falls back to the policy-level AllowedRegions rather than
			// being treated as "no regions allowed for this class".
			{DataClass: "case_document"},
		},
	}

	got := policy.AllowedRegionsFor("case_document")
	if len(got) != 2 {
		t.Fatalf("expected fallback to policy-level AllowedRegions, got %v", got)
	}
}

func TestResidencyPolicy_AllowedRegionsFor_MatchesCaseInsensitively(t *testing.T) {
	policy := &dataresidency.ResidencyPolicy{
		DeploymentID:   uuid.New(),
		AllowedRegions: []string{"eu", "us"},
		DataClassRules: []dataresidency.DataClassRule{
			{DataClass: "PII", AllowedRegions: []string{"eu"}},
		},
	}

	got := policy.AllowedRegionsFor("pii")
	if len(got) != 1 || got[0] != "eu" {
		t.Fatalf("expected case-insensitive data class match, got %v", got)
	}
}

func TestRegionPin_ValidateDSN_MatchesAnyOfMultiplePatterns(t *testing.T) {
	pin := &dataresidency.RegionPin{
		DeploymentID: uuid.New(),
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com", "eu-central-1.rds.example.com"},
	}

	if err := pin.ValidateDSN("postgres://user:pass@eu-central-1.rds.example.com:5432/verdex?sslmode=disable"); err != nil {
		t.Fatalf("expected second pattern to match: %v", err)
	}
}
