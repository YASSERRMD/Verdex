package reasoningprofile

import "math"

// Validate reports whether w is a well-formed Weights value: every field
// must be a finite (non-NaN, non-Inf) number in the inclusive range
// [0, 1]. Returns ErrInvalidWeight (wrapped with the offending field's
// name) on the first violation found, checked in field-declaration order.
func Validate(w Weights) error {
	fields := []struct {
		name  string
		value float64
	}{
		{"TestimonyEmphasis", w.TestimonyEmphasis},
		{"DocumentaryEmphasis", w.DocumentaryEmphasis},
		{"StatuteEmphasis", w.StatuteEmphasis},
		{"PrecedentEmphasis", w.PrecedentEmphasis},
	}

	for _, f := range fields {
		if err := validateWeight(f.value); err != nil {
			return &InvalidWeightError{Field: f.name, Value: f.value, Err: err}
		}
	}
	return nil
}

func validateWeight(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ErrInvalidWeight
	}
	if v < 0 || v > 1 {
		return ErrInvalidWeight
	}
	return nil
}

// InvalidWeightError describes exactly which Weights field failed
// Validate and with what value, while remaining unwrappable to
// ErrInvalidWeight via errors.Is/errors.As.
type InvalidWeightError struct {
	Field string
	Value float64
	Err   error
}

// Error implements the error interface.
func (e *InvalidWeightError) Error() string {
	return "reasoningprofile: invalid weight for " + e.Field + ": " + e.Err.Error()
}

// Unwrap allows errors.Is(err, ErrInvalidWeight) to succeed.
func (e *InvalidWeightError) Unwrap() error {
	return e.Err
}

// ValidateFamily reports whether family is one of the four canonical
// Family constants, returning ErrUnknownFamily if not.
func ValidateFamily(family Family) error {
	if !family.IsValid() {
		return ErrUnknownFamily
	}
	return nil
}
