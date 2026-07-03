package dataresidency_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
)

func TestRegionPin_ValidateDSN_MatchesHostPattern(t *testing.T) {
	pin := &dataresidency.RegionPin{
		DeploymentID: uuid.New(),
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com"},
	}

	if err := pin.ValidateDSN("postgres://user:pass@eu-west-1.rds.example.com:5432/verdex?sslmode=disable"); err != nil {
		t.Fatalf("expected matching host to validate: %v", err)
	}
}

func TestRegionPin_ValidateDSN_RejectsMismatchedHost(t *testing.T) {
	pin := &dataresidency.RegionPin{
		DeploymentID: uuid.New(),
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com"},
	}

	err := pin.ValidateDSN("postgres://user:pass@us-east-1.rds.example.com:5432/verdex?sslmode=disable")
	if err == nil {
		t.Fatal("expected error for mismatched host, got nil")
	}
	if !errors.Is(err, dataresidency.ErrStorageRegionMismatch) {
		t.Fatalf("expected ErrStorageRegionMismatch, got %v", err)
	}
}

func TestRegionPin_ValidateDSN_EmptyDSN(t *testing.T) {
	pin := &dataresidency.RegionPin{
		DeploymentID: uuid.New(),
		Region:       "eu",
		HostPatterns: []string{"eu-west-1.rds.example.com"},
	}
	if err := pin.ValidateDSN(""); !errors.Is(err, dataresidency.ErrEmptyDSN) {
		t.Fatalf("expected ErrEmptyDSN, got %v", err)
	}
}

func TestRegionPin_Validate_RequiresHostPatterns(t *testing.T) {
	pin := &dataresidency.RegionPin{
		DeploymentID: uuid.New(),
		Region:       "eu",
	}
	if err := pin.Validate(); err == nil {
		t.Fatal("expected error for empty HostPatterns, got nil")
	}
}

func TestRegionPin_ValidateDSN_LocalAirGappedHost(t *testing.T) {
	pin := &dataresidency.RegionPin{
		DeploymentID: uuid.New(),
		Region:       "local",
		HostPatterns: []string{"localhost"},
	}
	if err := pin.ValidateDSN("postgres://user:pass@localhost:5432/verdex?sslmode=disable"); err != nil {
		t.Fatalf("expected localhost DSN to validate against local pin: %v", err)
	}
}
