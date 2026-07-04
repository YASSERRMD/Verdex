package cicdgate

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is, mirroring the
// convention established by packages/compliance/errors.go and
// packages/category/errors.go.
var (
	// ErrInvalidBranchName is returned when a branch name does not
	// match this repository's phase-NNN-slug or fix-* naming policy
	// (see CONTRIBUTING.md's "Branching" section and
	// .github/branch-protection.md).
	ErrInvalidBranchName = errors.New("cicdgate: invalid branch name")

	// ErrInsufficientCommits is returned when a pull request's commit
	// count is below this repository's documented minimum (see
	// CONTRIBUTING.md's "Minimum 10 atomic commits per phase").
	ErrInsufficientCommits = errors.New("cicdgate: pull request has too few commits")

	// ErrInvalidArtifact is returned when a ReleaseArtifact fails
	// structural validation.
	ErrInvalidArtifact = errors.New("cicdgate: invalid release artifact")

	// ErrInvalidAttestation is returned when a BuildAttestation fails
	// structural validation.
	ErrInvalidAttestation = errors.New("cicdgate: invalid build attestation")

	// ErrAttestationMismatch is returned by Verify when an artifact and
	// its attestation are each individually well-formed but internally
	// inconsistent with each other (e.g. digest algorithm mismatch,
	// attestation timestamp precedes any plausible build window).
	ErrAttestationMismatch = errors.New("cicdgate: artifact and attestation are inconsistent")

	// ErrInvalidRolloutTrigger is returned when a RolloutTrigger fails
	// structural validation.
	ErrInvalidRolloutTrigger = errors.New("cicdgate: invalid rollout trigger")

	// ErrInvalidRollbackTrigger is returned when a RollbackTrigger fails
	// structural validation.
	ErrInvalidRollbackTrigger = errors.New("cicdgate: invalid rollback trigger")

	// ErrRollbackConditionNotMet is returned by Evaluate when none of a
	// RollbackTrigger's configured conditions are satisfied by the
	// supplied StageHealth snapshot.
	ErrRollbackConditionNotMet = errors.New("cicdgate: rollback condition not met")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages (see
// packages/compliance/errors.go's identical helper).
func wrapf(fn string, err error) error {
	return fmt.Errorf("cicdgate: %s: %w", fn, err)
}
