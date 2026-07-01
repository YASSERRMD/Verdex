package provenance

import "errors"

// Sentinel errors returned by the provenance package.
var (
	// ErrProvenanceNotFound is returned when a lookup finds no matching record.
	ErrProvenanceNotFound = errors.New("provenance: record not found")

	// ErrSignatureInvalid is returned when signature verification fails.
	ErrSignatureInvalid = errors.New("provenance: signature invalid")

	// ErrTamperDetected is returned when a stored record no longer matches the
	// original content that was appended.
	ErrTamperDetected = errors.New("provenance: tamper detected")

	// ErrChainBroken is returned when the chain-hash sequence contains a gap
	// or mismatch.
	ErrChainBroken = errors.New("provenance: chain broken")

	// ErrAlreadyDiscarded is returned when an operation is attempted on a
	// record that has already been discarded.
	ErrAlreadyDiscarded = errors.New("provenance: record already discarded")
)
