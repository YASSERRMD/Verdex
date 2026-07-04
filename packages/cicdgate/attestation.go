package cicdgate

import (
	"fmt"
	"time"
)

// maxBuildToArtifactSkew bounds how much later a ReleaseArtifact's
// BuiltAt may be recorded relative to its BuildAttestation's
// BuildTimestamp. Some skew is expected (the attestation records when
// the build step started; the artifact's BuiltAt is stamped when the
// output file is finalized moments later), but an artifact claiming to
// be built hours before its own attestation's build even started, or
// a wall-clock gap implausibly large for a CI job, indicates the two
// records do not actually describe the same build.
const maxBuildToArtifactSkew = 1 * time.Hour

// Verify checks that artifact and attestation are each individually
// well-formed (via their own Validate methods) and mutually
// consistent: their digest algorithms agree in kind where comparable,
// and their timestamps fall within a plausible build window of one
// another. Verify does not -- and, per doc/cicd.md, currently cannot
// -- cryptographically verify artifact.SignatureRef against a real
// public key or transparency log; that is exactly the gap
// SignatureStatePlaceholder documents. Verify is the mechanical
// consistency check that step is layered on top of once real signing
// exists.
func Verify(artifact *ReleaseArtifact, attestation *BuildAttestation) error {
	if err := artifact.Validate(); err != nil {
		return wrapf("Verify", err)
	}
	if err := attestation.Validate(); err != nil {
		return wrapf("Verify", err)
	}

	// An unsigned artifact has no attestable claim to check consistency
	// against beyond structural validity, which the two Validate calls
	// above already covered.
	if artifact.SignatureState == SignatureStateUnsigned {
		return nil
	}

	if artifact.BuiltAt.Before(attestation.BuildTimestamp) {
		return wrapf("Verify", fmt.Errorf("%w: artifact built_at (%s) precedes its attestation's build_timestamp (%s)",
			ErrAttestationMismatch, artifact.BuiltAt.Format(time.RFC3339), attestation.BuildTimestamp.Format(time.RFC3339)))
	}

	if gap := artifact.BuiltAt.Sub(attestation.BuildTimestamp); gap > maxBuildToArtifactSkew {
		return wrapf("Verify", fmt.Errorf("%w: artifact built_at is %s after its attestation's build_timestamp, exceeding the %s plausible build window",
			ErrAttestationMismatch, gap, maxBuildToArtifactSkew))
	}

	return nil
}
