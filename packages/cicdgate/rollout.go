package cicdgate

import (
	"fmt"
	"strings"
)

// RolloutStage names one step of a staged rollout, ordered from
// smallest to largest blast radius. A closed enum (unlike
// Framework/DigestAlgorithm elsewhere in this package): a rollout
// pipeline's stage sequence is a structural property of this package's
// automation, not something a deployment should be able to redefine
// per tenant.
type RolloutStage string

const (
	// StageCanary is the first stage: a small percentage of traffic or
	// a single deployment target.
	StageCanary RolloutStage = "canary"

	// StagePartial is an intermediate stage covering a larger, but
	// still incomplete, portion of the fleet.
	StagePartial RolloutStage = "partial"

	// StageFull is the final stage: the change is live everywhere.
	StageFull RolloutStage = "full"
)

// rolloutStageOrder fixes the sequence StagePlan.Validate and
// NextStage walk through.
var rolloutStageOrder = []RolloutStage{StageCanary, StagePartial, StageFull}

// IsValid reports whether s is one of the named RolloutStage
// constants.
func (s RolloutStage) IsValid() bool {
	switch s {
	case StageCanary, StagePartial, StageFull:
		return true
	}
	return false
}

// index returns s's position in rolloutStageOrder, or -1 if s is not a
// recognized stage.
func (s RolloutStage) index() int {
	for i, st := range rolloutStageOrder {
		if st == s {
			return i
		}
	}
	return -1
}

// RolloutTrigger describes one automated staged-rollout step: which
// stage to promote a release to, and the minimum bake time and health
// signal required before this package's automation is willing to
// trigger it (task 6: staged rollout automation).
//
// See doc.go / doc/cicd.md: packages/iac (Phase 094) does not exist in
// this repository yet, so RolloutTrigger is a minimal, local type
// rather than a composition with packages/iac.RolloutStrategy /
// PromotionPipeline. Once packages/iac lands, it is the natural
// long-term home for staged-rollout orchestration across this
// platform generally; this package's RolloutTrigger models only the
// release-pipeline-specific slice (which artifact, which stage, what
// gate) that this phase needs today.
type RolloutTrigger struct {
	// ReleaseName identifies the release this trigger promotes (e.g.
	// matches a ReleaseArtifact.Name).
	ReleaseName string `json:"release_name"`

	// TargetStage is the stage this trigger promotes ReleaseName to.
	TargetStage RolloutStage `json:"target_stage"`

	// RequiredHealthySamples is the minimum number of consecutive
	// healthy StageHealth observations required at the current stage
	// before promoting to TargetStage.
	RequiredHealthySamples int `json:"required_healthy_samples"`
}

// Validate checks t for structural well-formedness.
func (t *RolloutTrigger) Validate() error {
	if t == nil {
		return ErrInvalidRolloutTrigger
	}
	if strings.TrimSpace(t.ReleaseName) == "" {
		return wrapf("RolloutTrigger.Validate", fmt.Errorf("%w: release_name is required", ErrInvalidRolloutTrigger))
	}
	if !t.TargetStage.IsValid() {
		return wrapf("RolloutTrigger.Validate", fmt.Errorf("%w: unrecognized target stage %q", ErrInvalidRolloutTrigger, t.TargetStage))
	}
	if t.RequiredHealthySamples < 1 {
		return wrapf("RolloutTrigger.Validate", fmt.Errorf("%w: required_healthy_samples must be >= 1, got %d", ErrInvalidRolloutTrigger, t.RequiredHealthySamples))
	}
	return nil
}

// NextStage returns the stage immediately after current in
// rolloutStageOrder, and false if current is StageFull (nothing
// further to promote to) or not a recognized stage.
func NextStage(current RolloutStage) (RolloutStage, bool) {
	i := current.index()
	if i < 0 || i+1 >= len(rolloutStageOrder) {
		return "", false
	}
	return rolloutStageOrder[i+1], true
}

// StageHealth is one observed health sample for a release at a given
// rollout stage -- the signal RolloutTrigger and RollbackTrigger
// evaluate against. A real deployment of this automation would
// populate this from packages/observability metrics/alerts; this
// package only models the shape of the signal and the decision logic
// over it, not the metrics pipeline itself.
type StageHealth struct {
	// Stage is the rollout stage this sample was observed at.
	Stage RolloutStage `json:"stage"`

	// ErrorRate is the observed error rate at Stage, expressed as a
	// fraction in [0, 1].
	ErrorRate float64 `json:"error_rate"`

	// LatencyP99Ms is the observed p99 latency in milliseconds at
	// Stage.
	LatencyP99Ms float64 `json:"latency_p99_ms"`
}

// IsHealthy reports whether h is within the bounds a RolloutTrigger
// requires to promote, i.e. ErrorRate and LatencyP99Ms each at or
// below the corresponding threshold.
func (h StageHealth) IsHealthy(maxErrorRate, maxLatencyP99Ms float64) bool {
	return h.ErrorRate <= maxErrorRate && h.LatencyP99Ms <= maxLatencyP99Ms
}

// EvaluatePromotion reports whether samples (all observed at trigger's
// implicit current stage) satisfy trigger's RequiredHealthySamples
// count of consecutive healthy observations under the given
// thresholds, returning the stage to promote to when they do.
//
// samples must be supplied in chronological order; only a
// RequiredHealthySamples-length run of healthy samples at the tail of
// samples counts -- a single unhealthy sample partway through resets
// the count, mirroring how a real canary analyzer would not let an
// early bad sample be offset by enough later good ones.
func EvaluatePromotion(trigger *RolloutTrigger, samples []StageHealth, maxErrorRate, maxLatencyP99Ms float64) (RolloutStage, error) {
	if err := trigger.Validate(); err != nil {
		return "", wrapf("EvaluatePromotion", err)
	}

	consecutive := 0
	for _, s := range samples {
		if s.IsHealthy(maxErrorRate, maxLatencyP99Ms) {
			consecutive++
		} else {
			consecutive = 0
		}
	}

	if consecutive < trigger.RequiredHealthySamples {
		return "", wrapf("EvaluatePromotion", fmt.Errorf("%w: only %d consecutive healthy sample(s) observed, want %d",
			ErrPromotionNotReady, consecutive, trigger.RequiredHealthySamples))
	}

	return trigger.TargetStage, nil
}
