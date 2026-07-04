package garelease_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/cicdgate"
	"github.com/YASSERRMD/verdex/packages/garelease"
)

func TestBuildReleaseArtifactAttestation_ConsistentPair(t *testing.T) {
	buildTime := time.Now().UTC().Add(-time.Hour)
	artifact := cicdgate.ReleaseArtifact{
		Name:            "verdex-platform@1.100.0",
		DigestAlgorithm: cicdgate.DigestSHA256,
		Digest:          strings.Repeat("a", 64),
		SignatureState:  cicdgate.SignatureStatePlaceholder,
		SignatureRef:    "placeholder-ref-001",
		BuiltAt:         buildTime.Add(time.Minute),
	}
	attestation := cicdgate.BuildAttestation{
		SourceCommit:          strings.Repeat("b", 40),
		BuilderID:             "github-actions/phase-100-ga-release",
		BuildTimestamp:        buildTime,
		InputsDigestAlgorithm: cicdgate.DigestSHA256,
		InputsDigest:          strings.Repeat("c", 64),
	}

	gotArtifact, gotAttestation, err := garelease.BuildReleaseArtifactAttestation(artifact, attestation)
	if err != nil {
		t.Fatalf("BuildReleaseArtifactAttestation: %v", err)
	}
	if gotArtifact.Name != artifact.Name {
		t.Errorf("gotArtifact.Name = %q, want %q", gotArtifact.Name, artifact.Name)
	}
	if gotAttestation.SourceCommit != attestation.SourceCommit {
		t.Errorf("gotAttestation.SourceCommit = %q, want %q", gotAttestation.SourceCommit, attestation.SourceCommit)
	}
}

func TestBuildReleaseArtifactAttestation_InconsistentPairRejected(t *testing.T) {
	buildTime := time.Now().UTC()
	artifact := cicdgate.ReleaseArtifact{
		Name:            "verdex-platform@1.100.0",
		DigestAlgorithm: cicdgate.DigestSHA256,
		Digest:          strings.Repeat("a", 64),
		SignatureState:  cicdgate.SignatureStatePlaceholder,
		SignatureRef:    "placeholder-ref-001",
		// BuiltAt precedes the attestation's BuildTimestamp -- an
		// artifact cannot have been finished building before its own
		// attestation's build even started.
		BuiltAt: buildTime.Add(-time.Hour),
	}
	attestation := cicdgate.BuildAttestation{
		SourceCommit:          strings.Repeat("b", 40),
		BuilderID:             "github-actions/phase-100-ga-release",
		BuildTimestamp:        buildTime,
		InputsDigestAlgorithm: cicdgate.DigestSHA256,
		InputsDigest:          strings.Repeat("c", 64),
	}

	_, _, err := garelease.BuildReleaseArtifactAttestation(artifact, attestation)
	if !errors.Is(err, cicdgate.ErrAttestationMismatch) {
		t.Fatalf("BuildReleaseArtifactAttestation with an inconsistent pair = %v, want ErrAttestationMismatch", err)
	}
}

func TestBuildReleaseArtifactAttestation_MalformedArtifactRejected(t *testing.T) {
	artifact := cicdgate.ReleaseArtifact{} // zero value: fails Validate
	attestation := cicdgate.BuildAttestation{
		SourceCommit:          strings.Repeat("b", 40),
		BuilderID:             "github-actions/phase-100-ga-release",
		BuildTimestamp:        time.Now().UTC(),
		InputsDigestAlgorithm: cicdgate.DigestSHA256,
		InputsDigest:          strings.Repeat("c", 64),
	}

	_, _, err := garelease.BuildReleaseArtifactAttestation(artifact, attestation)
	if !errors.Is(err, cicdgate.ErrInvalidArtifact) {
		t.Fatalf("BuildReleaseArtifactAttestation with a malformed artifact = %v, want ErrInvalidArtifact", err)
	}
}
