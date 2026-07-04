package cicdgate

import (
	"fmt"
)

// RollbackReason names why an automated rollback fired. A closed enum:
// the reason taxonomy is a structural property of this package's
// automation (what it knows how to detect), not a per-tenant
// extension point.
type RollbackReason string

const (
	// RollbackReasonErrorRate fires when observed ErrorRate exceeds a
	// RollbackTrigger's MaxErrorRate.
	RollbackReasonErrorRate RollbackReason = "error_rate_exceeded"

	// RollbackReasonLatency fires when observed LatencyP99Ms exceeds a
	// RollbackTrigger's MaxLatencyP99Ms.
	RollbackReasonLatency RollbackReason = "latency_exceeded"
)

// RollbackTrigger describes one automated rollback condition: the
// release/stage it watches, and the error-rate and latency ceilings
// that, once exceeded, should trigger an automatic rollback rather
// than wait for a human to notice (task 7: automated rollback
// triggers).
//
// See RolloutTrigger's doc comment and doc.go / doc/cicd.md for why
// this is a minimal local type rather than a composition with
// packages/iac (which does not exist in this repository yet).
type RollbackTrigger struct {
	// ReleaseName identifies the release this trigger watches.
	ReleaseName string `json:"release_name"`

	// Stage is the rollout stage this trigger watches.
	Stage RolloutStage `json:"stage"`

	// MaxErrorRate is the error-rate ceiling, expressed as a fraction
	// in [0, 1], above which this trigger fires
	// RollbackReasonErrorRate.
	MaxErrorRate float64 `json:"max_error_rate"`

	// MaxLatencyP99Ms is the p99 latency ceiling in milliseconds above
	// which this trigger fires RollbackReasonLatency.
	MaxLatencyP99Ms float64 `json:"max_latency_p99_ms"`
}

// Validate checks t for structural well-formedness.
func (t *RollbackTrigger) Validate() error {
	if t == nil {
		return ErrInvalidRollbackTrigger
	}
	if t.ReleaseName == "" {
		return wrapf("RollbackTrigger.Validate", fmt.Errorf("%w: release_name is required", ErrInvalidRollbackTrigger))
	}
	if !t.Stage.IsValid() {
		return wrapf("RollbackTrigger.Validate", fmt.Errorf("%w: unrecognized stage %q", ErrInvalidRollbackTrigger, t.Stage))
	}
	if t.MaxErrorRate < 0 || t.MaxErrorRate > 1 {
		return wrapf("RollbackTrigger.Validate", fmt.Errorf("%w: max_error_rate must be in [0, 1], got %v", ErrInvalidRollbackTrigger, t.MaxErrorRate))
	}
	if t.MaxLatencyP99Ms <= 0 {
		return wrapf("RollbackTrigger.Validate", fmt.Errorf("%w: max_latency_p99_ms must be > 0, got %v", ErrInvalidRollbackTrigger, t.MaxLatencyP99Ms))
	}
	return nil
}

// RollbackDecision is EvaluateRollback's result when a RollbackTrigger
// fires: which stage to roll back, and why.
type RollbackDecision struct {
	// Stage is the rollout stage the rollback applies to.
	Stage RolloutStage `json:"stage"`

	// Reason is why the rollback fired.
	Reason RollbackReason `json:"reason"`

	// Observed is the StageHealth sample that tripped the trigger.
	Observed StageHealth `json:"observed"`
}

// EvaluateRollback checks the most recent sample in samples (the tail
// element, i.e. samples[len(samples)-1]) against trigger's ceilings
// and reports a RollbackDecision when either is exceeded. Only the
// latest sample is considered -- a single bad observation is
// sufficient grounds for an automatic rollback (unlike promotion in
// rollout.go, which deliberately requires a consecutive run of
// *healthy* samples before promoting forward; rolling back on the
// first sign of trouble, but promoting only after sustained health,
// is the intentionally asymmetric safety posture this function
// encodes).
//
// Returns an error wrapping ErrRollbackConditionNotMet (not a
// RollbackDecision) when samples is empty or the latest sample is
// within both ceilings -- there is nothing to roll back.
func EvaluateRollback(trigger *RollbackTrigger, samples []StageHealth) (RollbackDecision, error) {
	if err := trigger.Validate(); err != nil {
		return RollbackDecision{}, wrapf("EvaluateRollback", err)
	}
	if len(samples) == 0 {
		return RollbackDecision{}, wrapf("EvaluateRollback", fmt.Errorf("%w: no health samples supplied", ErrRollbackConditionNotMet))
	}

	latest := samples[len(samples)-1]

	if latest.ErrorRate > trigger.MaxErrorRate {
		return RollbackDecision{
			Stage:    trigger.Stage,
			Reason:   RollbackReasonErrorRate,
			Observed: latest,
		}, nil
	}
	if latest.LatencyP99Ms > trigger.MaxLatencyP99Ms {
		return RollbackDecision{
			Stage:    trigger.Stage,
			Reason:   RollbackReasonLatency,
			Observed: latest,
		}, nil
	}

	return RollbackDecision{}, wrapf("EvaluateRollback", fmt.Errorf("%w: latest sample (error_rate=%v, latency_p99_ms=%v) is within configured ceilings",
		ErrRollbackConditionNotMet, latest.ErrorRate, latest.LatencyP99Ms))
}
