package airgapped_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/airgapped"
)

// testLicenseDeploymentID is the fixed DeploymentID every test license
// in this file is issued for.
const testLicenseDeploymentID = "deployment-1"

func buildSignedLicense(t *testing.T, signer airgapped.Signer, expiresAt time.Time, revoked bool) airgapped.LicenseKey {
	t.Helper()
	lk := airgapped.LicenseKey{
		LicenseID:    "lic-001",
		DeploymentID: testLicenseDeploymentID,
		Features:     []string{"airgapped-tier"},
		IssuedAt:     time.Now().UTC().Add(-24 * time.Hour),
		ExpiresAt:    expiresAt,
		Revoked:      revoked,
	}
	if err := airgapped.SignLicenseKey(context.Background(), signer, &lk); err != nil {
		t.Fatalf("SignLicenseKey: %v", err)
	}
	return lk
}

func TestActivate_ValidLicense(t *testing.T) {
	signer := testSigner()
	lk := buildSignedLicense(t, signer, time.Now().UTC().Add(365*24*time.Hour), false)

	act, err := airgapped.Activate(context.Background(), signer, lk, testLicenseDeploymentID, time.Time{})
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if act.LicenseID != "lic-001" {
		t.Errorf("LicenseID = %q, want lic-001", act.LicenseID)
	}
	if len(act.Features) != 1 || act.Features[0] != "airgapped-tier" {
		t.Errorf("Features = %v, want [airgapped-tier]", act.Features)
	}
}

func TestActivate_InvalidSignatureRejected(t *testing.T) {
	signer := testSigner()
	lk := buildSignedLicense(t, signer, time.Now().UTC().Add(365*24*time.Hour), false)
	lk.Features = append(lk.Features, "tampered-feature") // mutate after signing

	_, err := airgapped.Activate(context.Background(), signer, lk, testLicenseDeploymentID, time.Time{})
	if !errors.Is(err, airgapped.ErrSignatureInvalid) {
		t.Fatalf("Activate(tampered) = %v, want ErrSignatureInvalid", err)
	}
}

func TestActivate_WrongSignerRejected(t *testing.T) {
	signer := testSigner()
	lk := buildSignedLicense(t, signer, time.Now().UTC().Add(365*24*time.Hour), false)

	otherSigner := testSignerWithKey("another-key")
	_, err := airgapped.Activate(context.Background(), otherSigner, lk, testLicenseDeploymentID, time.Time{})
	if !errors.Is(err, airgapped.ErrSignatureInvalid) {
		t.Fatalf("Activate(wrong signer) = %v, want ErrSignatureInvalid", err)
	}
}

func TestActivate_RevokedLicenseRejected(t *testing.T) {
	signer := testSigner()
	lk := buildSignedLicense(t, signer, time.Now().UTC().Add(365*24*time.Hour), true)

	_, err := airgapped.Activate(context.Background(), signer, lk, testLicenseDeploymentID, time.Time{})
	if !errors.Is(err, airgapped.ErrLicenseRevoked) {
		t.Fatalf("Activate(revoked) = %v, want ErrLicenseRevoked", err)
	}
}

func TestActivate_ExpiredLicenseRejected(t *testing.T) {
	signer := testSigner()
	lk := buildSignedLicense(t, signer, time.Now().UTC().Add(-1*time.Hour), false)

	_, err := airgapped.Activate(context.Background(), signer, lk, testLicenseDeploymentID, time.Time{})
	if !errors.Is(err, airgapped.ErrLicenseExpired) {
		t.Fatalf("Activate(expired) = %v, want ErrLicenseExpired", err)
	}
}

func TestActivate_DeploymentMismatchRejected(t *testing.T) {
	signer := testSigner()
	lk := buildSignedLicense(t, signer, time.Now().UTC().Add(365*24*time.Hour), false)

	_, err := airgapped.Activate(context.Background(), signer, lk, "deployment-2", time.Time{})
	if !errors.Is(err, airgapped.ErrSignatureInvalid) {
		t.Fatalf("Activate(mismatched deployment) = %v, want ErrSignatureInvalid", err)
	}
}

func TestActivate_NilSigner(t *testing.T) {
	lk := airgapped.LicenseKey{LicenseID: "lic-001"}
	_, err := airgapped.Activate(context.Background(), nil, lk, testLicenseDeploymentID, time.Time{})
	if !errors.Is(err, airgapped.ErrNilSigner) {
		t.Fatalf("Activate(nil signer) = %v, want ErrNilSigner", err)
	}
}
