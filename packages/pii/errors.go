package pii

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyInput is returned when a detection or redaction operation is
	// given empty (or whitespace-only, after cleanup) text.
	ErrEmptyInput = errors.New("pii: input is empty")

	// ErrAccessDenied is returned when a caller attempts to reveal a
	// pseudonym mapping without authorization from the configured
	// AccessPolicy (see mapping.go).
	ErrAccessDenied = errors.New("pii: access denied to pseudonym mapping")

	// ErrAlreadyIrreversible is returned when a caller attempts to reveal or
	// otherwise recover the original value behind a match that was redacted
	// using ModeIrreversibleRedact, which discards the original value
	// permanently and stores no mapping (see redact.go).
	ErrAlreadyIrreversible = errors.New("pii: value was irreversibly redacted, no mapping retained")

	// ErrUnknownToken is returned when a caller attempts to reveal a
	// pseudonym token that is not present in the PseudonymMap.
	ErrUnknownToken = errors.New("pii: unknown pseudonym token")

	// ErrInvalidRequest is returned when a request contains invalid or
	// missing fields.
	ErrInvalidRequest = errors.New("pii: invalid request")

	// ErrPolicyViolation is returned by StorageGuard when text fails a
	// storage-boundary PII policy configured to reject rather than redact
	// (see policy.go).
	ErrPolicyViolation = errors.New("pii: text violates storage PII policy")
)
