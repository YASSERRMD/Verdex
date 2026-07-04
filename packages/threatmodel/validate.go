package threatmodel

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file is task 2's hardening library: a real, testable
// Validator/Sanitize set of building-block functions -- size limits,
// charset/structure checks, control-character rejection -- usable
// wherever untrusted input is accepted across the platform. It is
// deliberately lower-level and protocol-agnostic compared to
// packages/gateway.ValidateRequest/validateStruct (Phase 009), which
// already handles HTTP-request-body JSON decoding and
// `validate:"required"` struct-tag checking; this package does not
// re-implement that middleware, and callers who already sit behind
// packages/gateway's HTTP layer should keep using it for request-body
// shape. This library exists for the many other places untrusted
// strings enter the system below the HTTP layer -- ingested document
// text, prompt template values, mitigation reference tags, and so on
// -- where no HTTP request object exists at all.

// DefaultMaxInputBytes is the byte-length ceiling ValidateSize applies
// when a caller does not specify its own limit -- generous enough for
// a large paragraph of case text, small enough that a single field
// cannot be used to exhaust memory.
const DefaultMaxInputBytes = 64 * 1024

// ValidateSize returns ErrInputTooLarge if s exceeds maxBytes (measured
// in bytes, not runes, since that is what actually consumes memory and
// wire bandwidth). A maxBytes of zero or less uses DefaultMaxInputBytes
// instead of disabling the check -- there is no supported way to
// request an unbounded input size from this function; a caller that
// genuinely needs a different bound should pass it explicitly.
func ValidateSize(s string, maxBytes int) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxInputBytes
	}
	if len(s) > maxBytes {
		return fmt.Errorf("%w: %d bytes exceeds limit of %d", ErrInputTooLarge, len(s), maxBytes)
	}
	return nil
}

// ValidateCharset returns ErrInputInvalidCharset if s contains invalid
// UTF-8, or any C0/C1 control character other than tab, newline, and
// carriage return (the same allow-list packages/prompts.SanitizeValue
// uses for ordinary whitespace, applied here as a hard rejection
// rather than a silent strip -- see SanitizeControlChars below for the
// strip variant). Rejecting rather than silently dropping is the
// right default for anything the caller intends to validate (as
// opposed to sanitize): a caller that wants to reject malformed input
// outright should use this function; a caller that wants to clean it
// up and continue should use SanitizeControlChars.
func ValidateCharset(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: input is not valid UTF-8", ErrInputInvalidCharset)
	}
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: contains control character %U", ErrInputInvalidCharset, r)
		}
	}
	return nil
}

// SanitizeControlChars strips every C0/C1 control character from s
// except tab, newline, and carriage return, returning the cleaned
// string. Unlike ValidateCharset, this never fails -- there is no
// counterfactual "reject" outcome, mirroring
// packages/guardrail.RequireDisclaimer's "always succeeds, no
// rejection path" shape for a transformation with an obvious safe
// default. Any byte sequence that is not valid UTF-8 is replaced with
// the Unicode replacement character by the range-over-string decode
// this function performs, so the result is always valid UTF-8.
func SanitizeControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ValidateNonBlank returns ErrInputInvalidStructure if s is empty
// after trimming leading/trailing whitespace -- the structural
// well-formedness floor almost every catalogued field in this
// codebase's Validate() methods already applies inline
// (strings.TrimSpace(x) == ""); this function exists so hardening
// call sites outside a domain type's own Validate() method (e.g. a
// handler validating a raw query parameter) can reuse the identical
// check without duplicating the TrimSpace idiom themselves.
func ValidateNonBlank(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%w: input is blank", ErrInputInvalidStructure)
	}
	return nil
}

// ValidateMaxRunes returns ErrInputTooLarge if s contains more than
// maxRunes runes -- a rune-based counterpart to ValidateSize for
// callers who care about display/character length rather than wire
// byte-length (e.g. a title field with a human-facing character
// limit), mirroring packages/prompts.SanitizeValue's rune-based
// (rather than byte-based) length check.
func ValidateMaxRunes(s string, maxRunes int) error {
	if maxRunes <= 0 {
		return nil
	}
	if n := utf8.RuneCountInString(s); n > maxRunes {
		return fmt.Errorf("%w: %d runes exceeds limit of %d", ErrInputTooLarge, n, maxRunes)
	}
	return nil
}

// ValidatorOptions configures Validate's combined check, letting a
// caller opt into exactly the checks relevant to one input field
// rather than always running every hardening check unconditionally.
type ValidatorOptions struct {
	// MaxBytes bounds input size in bytes; zero uses
	// DefaultMaxInputBytes. Negative disables the size check entirely
	// (unlike ValidateSize's zero-or-less-means-default behavior, so a
	// caller that has already bounded size some other way -- e.g. an
	// HTTP body-size limit upstream -- can skip a redundant check
	// explicitly rather than being forced to accept
	// DefaultMaxInputBytes).
	MaxBytes int

	// MaxRunes bounds input length in runes if positive; zero or
	// negative disables this check.
	MaxRunes int

	// RequireNonBlank, if true, rejects input that is empty after
	// trimming whitespace.
	RequireNonBlank bool

	// RejectControlChars, if true, rejects (rather than silently
	// strips) disallowed control characters via ValidateCharset.
	RejectControlChars bool
}

// Validate runs every check opt enables against s, in a fixed
// deterministic order (size, charset, non-blank, max-runes), returning
// the first failure encountered rather than collecting every failure
// -- callers needing a full multi-field report (like
// packages/gateway.ValidationError's aggregated FieldErrors) should
// call the individual Validate* functions themselves per field and
// aggregate as their own call site sees fit; this combined entry point
// is for the common case of one input, one pass/fail outcome.
func Validate(s string, opt ValidatorOptions) error {
	maxBytes := opt.MaxBytes
	if maxBytes >= 0 {
		if err := ValidateSize(s, maxBytes); err != nil {
			return wrapf("Validate", err)
		}
	}
	if opt.RejectControlChars {
		if err := ValidateCharset(s); err != nil {
			return wrapf("Validate", err)
		}
	}
	if opt.RequireNonBlank {
		if err := ValidateNonBlank(s); err != nil {
			return wrapf("Validate", err)
		}
	}
	if opt.MaxRunes > 0 {
		if err := ValidateMaxRunes(s, opt.MaxRunes); err != nil {
			return wrapf("Validate", err)
		}
	}
	return nil
}
