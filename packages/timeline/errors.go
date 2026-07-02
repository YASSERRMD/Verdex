package timeline

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required text input.
	ErrEmptyInput = errors.New("timeline: input is empty")

	// ErrCaseNotFound is returned when a lookup by case ID finds no
	// matching record in a TimelineStore.
	ErrCaseNotFound = errors.New("timeline: case not found")

	// ErrPartyNotFound is returned when an operation references a
	// Party.ID that has not been registered for the case.
	ErrPartyNotFound = errors.New("timeline: party not found")

	// ErrEventNotFound is returned when an operation references an
	// Event.ID that has not been registered for the case.
	ErrEventNotFound = errors.New("timeline: event not found")

	// ErrInvalidParty is returned when a Party fails basic validation
	// (e.g. empty ID, empty Name, or an unrecognized PartyRole).
	ErrInvalidParty = errors.New("timeline: invalid party")

	// ErrInvalidClaim is returned when a Claim fails basic validation
	// (e.g. empty ID, unknown PartyID, or no supporting entries).
	ErrInvalidClaim = errors.New("timeline: invalid claim")
)
