package garelease

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is, mirroring the
// convention established by packages/compliance/errors.go and
// packages/cicdgate/errors.go.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("garelease: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a release-readiness operation requires.
	ErrForbidden = errors.New("garelease: actor lacks required permission")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing repository.
	ErrNilStore = errors.New("garelease: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("garelease: audit sink must not be nil")

	// ErrInvalidVersion is returned when a candidate/release version
	// string fails semantic-versioning validation.
	ErrInvalidVersion = errors.New("garelease: invalid semantic version")

	// ErrInvalidCommitSHA is returned when a commit SHA is not a
	// well-formed 40- or 64-character lowercase-hex git object ID.
	ErrInvalidCommitSHA = errors.New("garelease: invalid commit sha")

	// ErrInvalidReadinessCheck is returned when a ReadinessCheck fails
	// structural validation.
	ErrInvalidReadinessCheck = errors.New("garelease: invalid readiness check")

	// ErrReadinessNotReady is returned by FreezeReleaseCandidate when
	// the supplied ReleaseReadiness snapshot is not Ready -- freezing an
	// unready candidate is refused outright, never merely warned about.
	ErrReadinessNotReady = errors.New("garelease: release readiness is not ready")

	// ErrCandidateNotFound is returned when a referenced
	// ReleaseCandidate ID does not resolve to any stored record.
	ErrCandidateNotFound = errors.New("garelease: release candidate not found")

	// ErrInvalidCandidate is returned when a ReleaseCandidate fails
	// structural validation.
	ErrInvalidCandidate = errors.New("garelease: invalid release candidate")

	// ErrCandidateNotFrozen is returned by CutRelease when the
	// referenced ReleaseCandidate has not completed FreezeReleaseCandidate
	// (should not occur through this package's own API, since
	// FreezeReleaseCandidate always persists a frozen candidate, but
	// guards against a caller supplying a hand-built, never-frozen
	// candidate ID).
	ErrCandidateNotFrozen = errors.New("garelease: release candidate is not frozen")

	// ErrCandidateReadinessStale is returned by CutRelease when the
	// candidate's frozen ReleaseReadiness snapshot is no longer Ready --
	// a candidate is only cuttable while its readiness, as last
	// evaluated, still holds.
	ErrCandidateReadinessStale = errors.New("garelease: release candidate readiness is no longer ready")

	// ErrReleaseNotFound is returned when a referenced Release ID does
	// not resolve to any stored record.
	ErrReleaseNotFound = errors.New("garelease: release not found")

	// ErrInvalidRelease is returned when a Release fails structural
	// validation.
	ErrInvalidRelease = errors.New("garelease: invalid release")

	// ErrAlreadyReleased is returned by CutRelease when the referenced
	// ReleaseCandidate has already been promoted to a Release --
	// CutRelease is not idempotent-by-retry; a second cut of the same
	// candidate is refused rather than silently returning a duplicate
	// Release.
	ErrAlreadyReleased = errors.New("garelease: release candidate already cut to a release")

	// ErrNilGuardrailCheck is returned by VerifyGuardrails if its
	// internal fixture wiring is misconfigured (defensive; should not
	// occur through this package's own exported API).
	ErrNilGuardrailCheck = errors.New("garelease: guardrail verification check is nil")

	// ErrNilAuditStore is returned by VerifyAuditTrail when called with
	// a nil auditlog.Store-shaped dependency.
	ErrNilAuditStore = errors.New("garelease: audit store must not be nil")

	// ErrEmptyRepresentativeTenantID is returned by VerifyAuditTrail
	// when called with a uuid.Nil representativeTenantID -- there is no
	// tenant history to query/verify against.
	ErrEmptyRepresentativeTenantID = errors.New("garelease: representative tenant id is required")

	// ErrEmptyChecklist is returned by RunPostReleaseChecklist when
	// given no checks to run.
	ErrEmptyChecklist = errors.New("garelease: post-release checklist has no checks")

	// ErrNilCheckFunc is returned by RunPostReleaseChecklist if any
	// PostReleaseCheck.Run is nil.
	ErrNilCheckFunc = errors.New("garelease: post-release check has a nil run function")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("garelease: %s: %w", fn, err)
}
