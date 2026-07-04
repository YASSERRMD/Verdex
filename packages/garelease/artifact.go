package garelease

import (
	"github.com/YASSERRMD/verdex/packages/cicdgate"
)

// BuildReleaseArtifactAttestation is task 7's "prepare release
// artifacts and attestation" -- real composition with
// packages/cicdgate's existing ReleaseArtifact/BuildAttestation/Verify
// (Phase 095), not a reimplementation. packages/cicdgate exposes no
// NewReleaseArtifact/NewBuildAttestation constructor function (verified
// directly against packages/cicdgate/types.go and attestation.go before
// writing this file): both types are plain structs built via literal
// and checked with their own *Validate() methods, so this function
// builds each via a literal exactly as packages/cicdgate's own
// types_test.go does, then calls the package-level cicdgate.Verify
// unmodified -- this package adds no artifact/attestation validation
// logic of its own.
//
// Returns the built ReleaseArtifact, BuildAttestation, and the result
// of cicdgate.Verify(&artifact, &attestation): a non-nil error means
// the two records are not mutually consistent (or either is
// individually malformed), so this release's artifact/attestation
// composition is not trustworthy as-is.
func BuildReleaseArtifactAttestation(artifact cicdgate.ReleaseArtifact, attestation cicdgate.BuildAttestation) (cicdgate.ReleaseArtifact, cicdgate.BuildAttestation, error) {
	if err := cicdgate.Verify(&artifact, &attestation); err != nil {
		return artifact, attestation, wrapf("BuildReleaseArtifactAttestation", err)
	}
	return artifact, attestation, nil
}
