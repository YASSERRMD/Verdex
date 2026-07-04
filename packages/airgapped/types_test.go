package airgapped_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/airgapped"
	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/router"
)

func TestProfile_Validate_Valid(t *testing.T) {
	profile := validProfile(t)
	if err := profile.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestProfile_Validate_NilProfile(t *testing.T) {
	var p *airgapped.Profile
	if err := p.Validate(); !errors.Is(err, airgapped.ErrNilProfile) {
		t.Fatalf("Validate() = %v, want ErrNilProfile", err)
	}
}

func TestProfile_Validate_WrongConfigProfile(t *testing.T) {
	profile := validProfile(t)
	profile.ConfigProfile = "production"
	if err := profile.Validate(); !errors.Is(err, airgapped.ErrDeploymentProfileRequired) {
		t.Fatalf("Validate() = %v, want ErrDeploymentProfileRequired", err)
	}
}

func TestProfile_Validate_ResidencyNotAirGapped(t *testing.T) {
	profile := validProfile(t)
	// A regionally-scoped policy (non-empty AllowedRegions) is not the
	// air-gapped preset shape.
	profile.Residency = dataresidency.ResidencyPolicy{
		DeploymentID:   profile.DeploymentID,
		AllowedRegions: []string{"eu"},
		StrictMode:     true,
	}
	if err := profile.Validate(); !errors.Is(err, airgapped.ErrResidencyNotAirGapped) {
		t.Fatalf("Validate() = %v, want ErrResidencyNotAirGapped", err)
	}
}

func TestProfile_Validate_RoutingNotAirGappedOnly(t *testing.T) {
	profile := validProfile(t)
	profile.Routing.AirGappedOnly = false
	if err := profile.Validate(); !errors.Is(err, airgapped.ErrRoutingNotAirGapped) {
		t.Fatalf("Validate() = %v, want ErrRoutingNotAirGapped", err)
	}
}

func TestProfile_Validate_KeyProviderMissing(t *testing.T) {
	profile := validProfile(t)
	profile.KeyProviderConfigured = false
	if err := profile.Validate(); !errors.Is(err, airgapped.ErrKeyProviderRequired) {
		t.Fatalf("Validate() = %v, want ErrKeyProviderRequired", err)
	}
}

func TestNewProfile_NilKeyProviderNotConfigured(t *testing.T) {
	deploymentID := uuid.New()
	residency := dataresidency.AirGappedPreset(deploymentID)
	routing := router.RoutingPolicy{AirGappedOnly: true}
	profile := airgapped.NewProfile(deploymentID, residency, routing, nil, nil)
	if profile.KeyProviderConfigured {
		t.Fatal("KeyProviderConfigured = true, want false for nil key provider")
	}
	if err := profile.Validate(); !errors.Is(err, airgapped.ErrKeyProviderRequired) {
		t.Fatalf("Validate() = %v, want ErrKeyProviderRequired", err)
	}
}

func TestProfile_Validate_EmptyDeploymentID(t *testing.T) {
	profile := validProfile(t)
	profile.DeploymentID = uuid.Nil
	if err := profile.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error for empty deployment id")
	}
}
