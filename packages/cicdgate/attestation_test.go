package cicdgate

import (
	"errors"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	t.Run("valid pair", func(t *testing.T) {
		a := validArtifact()
		att := validAttestation()
		if err := Verify(&a, &att); err != nil {
			t.Fatalf("Verify() error = %v, want nil", err)
		}
	})

	t.Run("artifact built exactly at attestation build timestamp", func(t *testing.T) {
		a := validArtifact()
		att := validAttestation()
		a.BuiltAt = att.BuildTimestamp
		if err := Verify(&a, &att); err != nil {
			t.Errorf("Verify() error = %v, want nil", err)
		}
	})

	t.Run("artifact built within plausible window after attestation", func(t *testing.T) {
		a := validArtifact()
		att := validAttestation()
		a.BuiltAt = att.BuildTimestamp.Add(30 * time.Minute)
		if err := Verify(&a, &att); err != nil {
			t.Errorf("Verify() error = %v, want nil", err)
		}
	})

	t.Run("invalid artifact propagates ErrInvalidArtifact", func(t *testing.T) {
		a := validArtifact()
		a.Name = ""
		att := validAttestation()
		err := Verify(&a, &att)
		if !errors.Is(err, ErrInvalidArtifact) {
			t.Errorf("Verify() error = %v, want wrapping ErrInvalidArtifact", err)
		}
	})

	t.Run("invalid attestation propagates ErrInvalidAttestation", func(t *testing.T) {
		a := validArtifact()
		att := validAttestation()
		att.BuilderID = ""
		err := Verify(&a, &att)
		if !errors.Is(err, ErrInvalidAttestation) {
			t.Errorf("Verify() error = %v, want wrapping ErrInvalidAttestation", err)
		}
	})

	t.Run("artifact built before attestation build timestamp", func(t *testing.T) {
		a := validArtifact()
		att := validAttestation()
		a.BuiltAt = att.BuildTimestamp.Add(-1 * time.Hour)
		err := Verify(&a, &att)
		if !errors.Is(err, ErrAttestationMismatch) {
			t.Errorf("Verify() error = %v, want wrapping ErrAttestationMismatch", err)
		}
	})

	t.Run("artifact built implausibly long after attestation", func(t *testing.T) {
		a := validArtifact()
		att := validAttestation()
		a.BuiltAt = att.BuildTimestamp.Add(48 * time.Hour)
		err := Verify(&a, &att)
		if !errors.Is(err, ErrAttestationMismatch) {
			t.Errorf("Verify() error = %v, want wrapping ErrAttestationMismatch", err)
		}
	})

	t.Run("unsigned artifact skips timestamp consistency check", func(t *testing.T) {
		a := validArtifact()
		a.SignatureState = SignatureStateUnsigned
		a.SignatureRef = ""
		att := validAttestation()
		// Deliberately implausible relative to the attestation -- should
		// still pass because an unsigned artifact makes no attestable
		// claim to check consistency against.
		a.BuiltAt = att.BuildTimestamp.Add(-48 * time.Hour)
		if err := Verify(&a, &att); err != nil {
			t.Errorf("Verify() error = %v, want nil for unsigned artifact", err)
		}
	})

	t.Run("nil artifact", func(t *testing.T) {
		att := validAttestation()
		err := Verify(nil, &att)
		if !errors.Is(err, ErrInvalidArtifact) {
			t.Errorf("Verify() error = %v, want wrapping ErrInvalidArtifact", err)
		}
	})

	t.Run("nil attestation", func(t *testing.T) {
		a := validArtifact()
		err := Verify(&a, nil)
		if !errors.Is(err, ErrInvalidAttestation) {
			t.Errorf("Verify() error = %v, want wrapping ErrInvalidAttestation", err)
		}
	})
}
