package eval

import "errors"

// Package-level sentinel errors.
var (
	// ErrNoGoldenSet is returned when an operation requires a GoldenSet but
	// none has been loaded or seeded.
	ErrNoGoldenSet = errors.New("eval: no golden set available")

	// ErrEvalFailed is returned when a provider call or scoring step fails in
	// a non-recoverable way.
	ErrEvalFailed = errors.New("eval: evaluation failed")

	// ErrRegressionDetected is returned by RegressionGate.Check when one or
	// more providers have dropped below the allowed score threshold relative to
	// the baseline.
	ErrRegressionDetected = errors.New("eval: regression detected")

	// ErrProviderNotScored is returned when a report is queried for a provider
	// that has no results recorded (e.g. was never included in the run).
	ErrProviderNotScored = errors.New("eval: provider not found in report")
)
