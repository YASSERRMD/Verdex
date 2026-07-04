package airgapped

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

// LicenseKey is an offline-verifiable license grant for an air-gapped
// deployment (task 6): no license-server callout is ever made. A
// LicenseKey is issued (signed) out of band by the vendor and
// distributed to the deployment alongside its update bundles.
type LicenseKey struct {
	// LicenseID uniquely identifies this license grant.
	LicenseID string `json:"license_id"`

	// DeploymentID restricts this license to a single deployment.
	DeploymentID string `json:"deployment_id"`

	// Features lists the feature/tier names this license unlocks (e.g.
	// "airgapped-tier", "unlimited-seats").
	Features []string `json:"features"`

	// IssuedAt and ExpiresAt bound the license's validity window.
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`

	// Revoked marks a license as no longer valid even if ExpiresAt has
	// not yet passed (e.g. a compromised or replaced license).
	Revoked bool `json:"revoked"`

	// Signature is the hex-encoded Signer signature over the
	// canonical encoding of every field above.
	Signature string `json:"signature"`
}

// canonicalLicensePayload returns the deterministic byte encoding of
// lk's signed fields, excluding Signature itself.
func canonicalLicensePayload(lk LicenseKey) []byte {
	features := append([]string(nil), lk.Features...)
	sort.Strings(features)
	payload := struct {
		LicenseID    string    `json:"license_id"`
		DeploymentID string    `json:"deployment_id"`
		Features     []string  `json:"features"`
		IssuedAt     time.Time `json:"issued_at"`
		ExpiresAt    time.Time `json:"expires_at"`
		Revoked      bool      `json:"revoked"`
	}{
		LicenseID:    lk.LicenseID,
		DeploymentID: lk.DeploymentID,
		Features:     features,
		IssuedAt:     lk.IssuedAt.UTC(),
		ExpiresAt:    lk.ExpiresAt.UTC(),
		Revoked:      lk.Revoked,
	}
	data, _ := json.Marshal(payload)
	return data
}

// SignLicenseKey computes and sets lk.Signature using signer, for
// issuing a license offline before distribution.
func SignLicenseKey(ctx context.Context, signer Signer, lk *LicenseKey) error {
	if signer == nil {
		return ErrNilSigner
	}
	sig, err := signer.Sign(ctx, canonicalLicensePayload(*lk))
	if err != nil {
		return wrapf("SignLicenseKey", err)
	}
	lk.Signature = sig
	return nil
}

// Activation is the local, offline record produced by activating a
// LicenseKey against a deployment: it records which license was
// verified valid for which deployment at which time, with no
// license-server callout.
type Activation struct {
	LicenseID    string    `json:"license_id"`
	DeploymentID string    `json:"deployment_id"`
	ActivatedAt  time.Time `json:"activated_at"`
	Features     []string  `json:"features"`
}

// Activate verifies lk's signature via signer and, if valid,
// unexpired, not revoked, and issued for deploymentID, returns an
// Activation record. It never makes a network call -- verification is
// a pure local signature check (task 6), reusing packages/provenance's
// Signer shape (see Signer in update.go) rather than inventing new
// crypto.
//
// Returns ErrSignatureInvalid if the signature does not verify,
// ErrLicenseRevoked if lk.Revoked is true, ErrLicenseExpired if
// lk.ExpiresAt has passed, and ErrDeploymentProfileRequired-adjacent
// mismatch errors are surfaced via wrapf as a generic activation
// failure when the license's DeploymentID does not match.
func Activate(ctx context.Context, signer Signer, lk LicenseKey, deploymentID string, now time.Time) (Activation, error) {
	if signer == nil {
		return Activation{}, ErrNilSigner
	}
	ok, err := signer.Verify(ctx, canonicalLicensePayload(lk), lk.Signature)
	if err != nil {
		return Activation{}, wrapf("Activate", err)
	}
	if !ok {
		return Activation{}, wrapf("Activate", ErrSignatureInvalid)
	}
	if lk.Revoked {
		return Activation{}, wrapf("Activate", ErrLicenseRevoked)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !lk.ExpiresAt.IsZero() && now.After(lk.ExpiresAt) {
		return Activation{}, wrapf("Activate", ErrLicenseExpired)
	}
	if lk.DeploymentID != deploymentID {
		return Activation{}, wrapf("Activate", ErrSignatureInvalid)
	}
	return Activation{
		LicenseID:    lk.LicenseID,
		DeploymentID: lk.DeploymentID,
		ActivatedAt:  now,
		Features:     append([]string(nil), lk.Features...),
	}, nil
}
