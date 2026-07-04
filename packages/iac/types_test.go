package iac

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTierIsValid(t *testing.T) {
	tests := []struct {
		tier Tier
		want bool
	}{
		{TierCloud, true},
		{TierOnPrem, true},
		{TierAirgapped, true},
		{Tier("staging"), false},
		{Tier(""), false},
	}
	for _, tc := range tests {
		if got := tc.tier.IsValid(); got != tc.want {
			t.Errorf("Tier(%q).IsValid() = %v, want %v", tc.tier, got, tc.want)
		}
	}
}

func TestTierInfraDir(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierCloud, "infra/cloud"},
		{TierOnPrem, "infra/onprem"},
		{TierAirgapped, "infra/airgapped"},
		{Tier("bogus"), ""},
	}
	for _, tc := range tests {
		if got := tc.tier.InfraDir(); got != tc.want {
			t.Errorf("Tier(%q).InfraDir() = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

func newValidCloudProfile() DeploymentProfile {
	return DeploymentProfile{
		DeploymentID: uuid.New(),
		TenantID:     uuid.New(),
		Tier:         TierCloud,
		Region:       "eu-west-1",
	}
}

func TestDeploymentProfileValidate_Cloud(t *testing.T) {
	p := newValidCloudProfile()
	if err := p.Validate(); err != nil {
		t.Fatalf("valid cloud profile failed validation: %v", err)
	}

	missingRegion := p
	missingRegion.Region = ""
	if err := missingRegion.Validate(); !errors.Is(err, ErrRegionRequiredForCloud) {
		t.Errorf("cloud profile with no region: got %v, want ErrRegionRequiredForCloud", err)
	}

	withAirgapRef := p
	withAirgapRef.AirgapConformanceRef = uuid.New().String()
	if err := withAirgapRef.Validate(); !errors.Is(err, ErrAirgapConformanceRefRequired) {
		t.Errorf("cloud profile with airgap ref: got %v, want ErrAirgapConformanceRefRequired", err)
	}
}

func TestDeploymentProfileValidate_OnPrem(t *testing.T) {
	p := DeploymentProfile{
		DeploymentID: uuid.New(),
		TenantID:     uuid.New(),
		Tier:         TierOnPrem,
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid on-prem profile failed validation: %v", err)
	}

	withRegion := p
	withRegion.Region = "eu-west-1"
	if err := withRegion.Validate(); !errors.Is(err, ErrRegionNotAllowedOutsideCloud) {
		t.Errorf("on-prem profile with region: got %v, want ErrRegionNotAllowedOutsideCloud", err)
	}
}

func TestDeploymentProfileValidate_Airgapped(t *testing.T) {
	p := DeploymentProfile{
		DeploymentID:         uuid.New(),
		TenantID:             uuid.New(),
		Tier:                 TierAirgapped,
		AirgapConformanceRef: uuid.New().String(),
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid airgapped profile failed validation: %v", err)
	}

	missingRef := p
	missingRef.AirgapConformanceRef = ""
	if err := missingRef.Validate(); !errors.Is(err, ErrAirgapConformanceRefRequired) {
		t.Errorf("airgapped profile with no conformance ref: got %v, want ErrAirgapConformanceRefRequired", err)
	}

	withRegion := p
	withRegion.Region = "eu-west-1"
	if err := withRegion.Validate(); !errors.Is(err, ErrRegionNotAllowedOutsideCloud) {
		t.Errorf("airgapped profile with region: got %v, want ErrRegionNotAllowedOutsideCloud", err)
	}
}

func TestDeploymentProfileValidate_MissingIDs(t *testing.T) {
	p := newValidCloudProfile()

	noDeployment := p
	noDeployment.DeploymentID = uuid.Nil
	if err := noDeployment.Validate(); !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("no deployment id: got %v, want ErrEmptyDeploymentID", err)
	}

	noTenant := p
	noTenant.TenantID = uuid.Nil
	if err := noTenant.Validate(); !errors.Is(err, ErrEmptyTenantID) {
		t.Errorf("no tenant id: got %v, want ErrEmptyTenantID", err)
	}
}

func TestDeploymentProfileValidate_InvalidTier(t *testing.T) {
	p := newValidCloudProfile()
	p.Tier = Tier("bogus")
	if err := p.Validate(); !errors.Is(err, ErrInvalidTier) {
		t.Errorf("invalid tier: got %v, want ErrInvalidTier", err)
	}
}

func TestDeploymentProfileValidate_Nil(t *testing.T) {
	var p *DeploymentProfile
	if err := p.Validate(); !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("nil profile: got %v, want ErrEmptyDeploymentID", err)
	}
}
