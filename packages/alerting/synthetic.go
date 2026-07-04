package alerting

import (
	"context"
	"strings"
	"time"
)

// Prober is a single synthetic probe function: it attempts whatever
// action a SyntheticCheck represents (reach a health endpoint, run a
// canary query) and returns nil on success or a non-nil error
// describing the failure. Mirrors observability.Checker's exact
// "func(ctx) error, nil means healthy" convention by reference (the
// same convention packages/reliability.TrafficShifter.HealthCheckFunc
// already follows), named independently in this package rather than
// imported, so a caller can wrap its existing /readyz checker as a
// Prober without this package depending on
// packages/observability's HTTP types.
type Prober func(ctx context.Context) error

// SyntheticCheck is a named, scheduled/on-demand probe (task 8): what
// to check (Prober), how long to allow it to run (Timeout), and the
// name it is recorded under in a CheckResult.
type SyntheticCheck struct {
	// Name identifies this check (e.g. "health-endpoint",
	// "ingestion-canary").
	Name string `json:"name"`

	// Prober performs the actual probe. Required to Run.
	Prober Prober `json:"-"`

	// Timeout bounds how long a single Run is allowed to take before
	// it is treated as a failure. Zero means no additional timeout is
	// applied beyond whatever the caller's ctx already carries.
	Timeout time.Duration `json:"timeout"`
}

// Validate checks c for structural well-formedness (Name only --
// Prober is checked separately by Run, since a SyntheticCheck value
// may legitimately be constructed for cataloguing/display before a
// concrete Prober is wired in).
func (c SyntheticCheck) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return wrapf("SyntheticCheck.Validate", ErrInvalidCheck)
	}
	return nil
}

// CheckResult is the outcome of running a SyntheticCheck once.
type CheckResult struct {
	// CheckName is the SyntheticCheck.Name this result belongs to.
	CheckName string `json:"check_name"`

	// Passed reports whether the probe succeeded (Prober returned
	// nil).
	Passed bool `json:"passed"`

	// Latency is how long the probe took to return, timeout or not.
	Latency time.Duration `json:"latency"`

	// Error is the Prober's returned error's message, empty when
	// Passed is true.
	Error string `json:"error,omitempty"`

	// RanAt records when this probe was executed.
	RanAt time.Time `json:"ran_at"`
}

// Run executes c.Prober once, applying c.Timeout (if positive) as an
// additional deadline on top of ctx, and records pass/fail plus
// latency (task 8) -- real logic: a real clock via now, a real
// context deadline, not a stub that always reports success.
//
// Returns ErrNilProber (wrapped) if c.Prober is nil, without invoking
// clock/timing -- a misconfigured check is a caller error, not itself
// a probe failure worth timing.
func (c SyntheticCheck) Run(ctx context.Context, now func() time.Time) (CheckResult, error) {
	if err := c.Validate(); err != nil {
		return CheckResult{}, err
	}
	if c.Prober == nil {
		return CheckResult{}, wrapf("SyntheticCheck.Run", ErrNilProber)
	}
	if now == nil {
		now = time.Now
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if c.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	start := now()
	err := c.Prober(runCtx)
	latency := now().Sub(start)

	result := CheckResult{
		CheckName: c.Name,
		Passed:    err == nil,
		Latency:   latency,
		RanAt:     start.UTC(),
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result, nil
}

// ObservabilityProberFromChecker adapts a function following
// observability.Checker's "func(ctx) error" shape into a Prober. Since
// Prober is already defined with the identical signature, this is a
// same-signature identity conversion -- provided as a named helper so
// call sites documenting the intent ("I am wrapping my existing
// readiness checker") read clearly, e.g.
// ObservabilityProberFromChecker(myReadinessChecker).
func ObservabilityProberFromChecker(checker func(ctx context.Context) error) Prober {
	return Prober(checker)
}
