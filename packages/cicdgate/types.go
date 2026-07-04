package cicdgate

// This file defines the core data model for signed builds and release
// provenance: ReleaseArtifact and BuildAttestation, the structured
// records a signed-build and provenance/attestation pipeline produces
// and consumes.

import (
	"strings"
	"time"
)

// DigestAlgorithm names the hash algorithm a ReleaseArtifact's Digest
// was computed with. An open string type (not a closed enum): new
// algorithms are added by convention as tooling evolves, mirroring
// packages/compliance.Framework's "open, not a closed enum" rationale.
type DigestAlgorithm string

const (
	// DigestSHA256 is the default and currently only digest algorithm
	// this package's own tooling produces.
	DigestSHA256 DigestAlgorithm = "sha256"

	// DigestSHA512 is accepted for artifacts produced by tooling that
	// prefers a longer digest.
	DigestSHA512 DigestAlgorithm = "sha512"
)

// IsValid reports whether a is one of the named DigestAlgorithm
// constants.
func (a DigestAlgorithm) IsValid() bool {
	switch a {
	case DigestSHA256, DigestSHA512:
		return true
	}
	return false
}

// expectedHexLen returns the expected hex-encoded digest length for a,
// or 0 if a is not a recognized algorithm.
func (a DigestAlgorithm) expectedHexLen() int {
	switch a {
	case DigestSHA256:
		return 64 // 32 bytes
	case DigestSHA512:
		return 128 // 64 bytes
	}
	return 0
}

// SignatureState describes how confident this pipeline is in a
// ReleaseArtifact's SignatureRef. See doc/cicd.md's "What is simulated
// versus real" section: this phase does not stand up a real
// Sigstore/cosign signing identity (that requires a KMS or OIDC
// identity this environment does not have configured), so
// SignatureStatePlaceholder is the only state a real CI run of this
// phase's workflow step can honestly produce today.
type SignatureState string

const (
	// SignatureStateUnsigned means no signing was attempted.
	SignatureStateUnsigned SignatureState = "unsigned"

	// SignatureStatePlaceholder means a signing step ran and recorded a
	// SignatureRef, but that reference is a simulated/placeholder value
	// -- not a real cryptographic signature verifiable against a real
	// public key or transparency log. See doc/cicd.md.
	SignatureStatePlaceholder SignatureState = "placeholder"

	// SignatureStateVerified means SignatureRef was checked against a
	// real signing identity and transparency log entry. No code path
	// in this package ever produces this value -- it is defined so the
	// type is forward-compatible with a future phase that wires up
	// real cosign/Sigstore verification without needing a breaking
	// change to this enum.
	SignatureStateVerified SignatureState = "verified"
)

// IsValid reports whether s is one of the named SignatureState
// constants.
func (s SignatureState) IsValid() bool {
	switch s {
	case SignatureStateUnsigned, SignatureStatePlaceholder, SignatureStateVerified:
		return true
	}
	return false
}

// ReleaseArtifact records one built, checksummed, (placeholder-)signed
// output of this repository's build pipeline -- a compiled binary, a
// container image, or an npm/go module artifact (task 4: signed builds
// and artifacts).
type ReleaseArtifact struct {
	// Name is the artifact's identifying filename or reference (e.g.
	// "cicdgate-linux-amd64", "packages/cicdgate@v0.1.0").
	Name string `json:"name"`

	// DigestAlgorithm names the hash algorithm Digest was computed
	// with.
	DigestAlgorithm DigestAlgorithm `json:"digest_algorithm"`

	// Digest is the hex-encoded checksum of the artifact's bytes,
	// whose length must match DigestAlgorithm's expected output size.
	Digest string `json:"digest"`

	// SignatureState reports how much trust SignatureRef carries. See
	// SignatureState's doc comment and doc/cicd.md.
	SignatureState SignatureState `json:"signature_state"`

	// SignatureRef is a reference to where a signature for this
	// artifact is recorded -- e.g. a placeholder identifier today, or a
	// real Sigstore/rekor transparency-log URL once a future phase
	// wires up real signing. Empty when SignatureState is
	// SignatureStateUnsigned.
	SignatureRef string `json:"signature_ref,omitempty"`

	// BuiltAt is when this artifact was produced.
	BuiltAt time.Time `json:"built_at"`
}

// Validate checks a for structural well-formedness: non-blank Name, a
// recognized DigestAlgorithm, a Digest of the expected length for that
// algorithm composed only of lowercase hex characters, a recognized
// SignatureState consistent with whether SignatureRef is set, and a
// non-zero BuiltAt.
func (a *ReleaseArtifact) Validate() error {
	if a == nil {
		return ErrInvalidArtifact
	}
	if strings.TrimSpace(a.Name) == "" {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("name is required"))
	}
	if !a.DigestAlgorithm.IsValid() {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("unrecognized digest algorithm %q", a.DigestAlgorithm))
	}
	if !isLowerHex(a.Digest) || len(a.Digest) != a.DigestAlgorithm.expectedHexLen() {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("digest is not a well-formed %d-character lowercase-hex %s digest", a.DigestAlgorithm.expectedHexLen(), a.DigestAlgorithm))
	}
	if !a.SignatureState.IsValid() {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("unrecognized signature state %q", a.SignatureState))
	}
	if a.SignatureState == SignatureStateUnsigned && a.SignatureRef != "" {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("signature_ref must be empty when signature_state is unsigned"))
	}
	if a.SignatureState != SignatureStateUnsigned && strings.TrimSpace(a.SignatureRef) == "" {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("signature_ref is required when signature_state is %q", a.SignatureState))
	}
	if a.BuiltAt.IsZero() {
		return wrapf("ReleaseArtifact.Validate", errInvalidArtifactf("built_at is required"))
	}
	return nil
}

// BuildAttestation records the provenance of a build: which commit it
// came from, which builder identity produced it, when the build ran,
// and a digest over its recorded inputs (task 5: provenance /
// attestation for releases). Modeled after in-toto/SLSA provenance's
// core fields, without importing an in-toto library -- see
// doc/cicd.md for why this phase models the data rather than
// integrating a real attestation framework.
type BuildAttestation struct {
	// SourceCommit is the full git commit SHA the build was produced
	// from.
	SourceCommit string `json:"source_commit"`

	// BuilderID identifies the system that ran the build (e.g.
	// "github-actions/phase-095-cicd-hardening" or a real SLSA builder
	// ID once one is configured).
	BuilderID string `json:"builder_id"`

	// BuildTimestamp is when the build ran.
	BuildTimestamp time.Time `json:"build_timestamp"`

	// InputsDigestAlgorithm names the hash algorithm InputsDigest was
	// computed with.
	InputsDigestAlgorithm DigestAlgorithm `json:"inputs_digest_algorithm"`

	// InputsDigest is a hex-encoded digest computed over the build's
	// declared inputs (e.g. go.sum/package-lock.json contents,
	// workflow file digest) so a consumer can detect whether the build
	// ran against different inputs than declared.
	InputsDigest string `json:"inputs_digest"`
}

// Validate checks att for structural well-formedness: a full 40 or 64
// hex-character SourceCommit (SHA-1 or SHA-256 git object IDs), a
// non-blank BuilderID, a non-zero BuildTimestamp not in the future,
// and a well-formed InputsDigest for InputsDigestAlgorithm.
func (att *BuildAttestation) Validate() error {
	if att == nil {
		return ErrInvalidAttestation
	}
	if !isLowerHex(att.SourceCommit) || (len(att.SourceCommit) != 40 && len(att.SourceCommit) != 64) {
		return wrapf("BuildAttestation.Validate", errInvalidAttestationf("source_commit is not a well-formed 40- or 64-character lowercase-hex git object id"))
	}
	if strings.TrimSpace(att.BuilderID) == "" {
		return wrapf("BuildAttestation.Validate", errInvalidAttestationf("builder_id is required"))
	}
	if att.BuildTimestamp.IsZero() {
		return wrapf("BuildAttestation.Validate", errInvalidAttestationf("build_timestamp is required"))
	}
	if att.BuildTimestamp.After(time.Now().Add(clockSkewTolerance)) {
		return wrapf("BuildAttestation.Validate", errInvalidAttestationf("build_timestamp is in the future"))
	}
	if !att.InputsDigestAlgorithm.IsValid() {
		return wrapf("BuildAttestation.Validate", errInvalidAttestationf("unrecognized inputs digest algorithm %q", att.InputsDigestAlgorithm))
	}
	if !isLowerHex(att.InputsDigest) || len(att.InputsDigest) != att.InputsDigestAlgorithm.expectedHexLen() {
		return wrapf("BuildAttestation.Validate", errInvalidAttestationf("inputs_digest is not a well-formed %d-character lowercase-hex %s digest", att.InputsDigestAlgorithm.expectedHexLen(), att.InputsDigestAlgorithm))
	}
	return nil
}

// clockSkewTolerance is how far into the future a BuildTimestamp or
// other recorded time may plausibly drift due to clock skew between
// the machine that produced a record and the machine validating it,
// before this package treats it as implausible rather than merely
// skewed.
const clockSkewTolerance = 5 * time.Minute

// isLowerHex reports whether s is non-empty and contains only
// lowercase hexadecimal digit characters.
func isLowerHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
