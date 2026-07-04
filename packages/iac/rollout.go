package iac

import (
	"strings"
	"time"
)

// RolloutStrategy names how a new version is rolled out to a running
// deployment (task 7). A closed enum: the strategy determines which
// orchestrator primitives a real deploy-time tool must drive (two
// parallel environments with a traffic cutover for BlueGreen, an
// incrementally-scaled traffic split for Canary, or a plain rolling
// update for Direct), so it is a structural property of the rollout,
// not a free-form label.
type RolloutStrategy string

const (
	// RolloutStrategyDirect replaces the running version in place
	// (a plain rolling update), with no traffic-splitting phase. The
	// default for TierOnPrem/TierAirgapped's typically single-replica
	// or low-replica-count deployments (see infra/onprem/deployment.yaml
	// and infra/airgapped/deployment.yaml's replica counts), where a
	// parallel blue/green environment or a canary split is often not
	// practical with the available hardware.
	RolloutStrategyDirect RolloutStrategy = "direct"

	// RolloutStrategyBlueGreen runs the new version ("green") fully
	// alongside the running version ("blue") and cuts traffic over in
	// one step once DeploymentVerification passes against green,
	// keeping blue available for immediate rollback.
	RolloutStrategyBlueGreen RolloutStrategy = "blue_green"

	// RolloutStrategyCanary shifts traffic to the new version
	// incrementally across a CanaryPlan's Stages, verifying at each
	// step before proceeding, rather than cutting over all at once.
	RolloutStrategyCanary RolloutStrategy = "canary"
)

// allRolloutStrategies is the exhaustive set of recognized
// RolloutStrategy values, used by IsValid.
var allRolloutStrategies = map[RolloutStrategy]struct{}{
	RolloutStrategyDirect:    {},
	RolloutStrategyBlueGreen: {},
	RolloutStrategyCanary:    {},
}

// IsValid reports whether s is one of the named RolloutStrategy
// constants.
func (s RolloutStrategy) IsValid() bool {
	_, ok := allRolloutStrategies[s]
	return ok
}

// String satisfies fmt.Stringer.
func (s RolloutStrategy) String() string { return string(s) }

// CanaryStage is one step of a CanaryPlan: what fraction of live
// traffic should reach the new version once this step is reached.
type CanaryStage struct {
	// Name is a human-readable label for this step (e.g. "5% canary",
	// "50% canary").
	Name string `json:"name"`

	// TrafficPercent is the percentage (0-100 inclusive) of traffic
	// that should reach the new version once this stage is active.
	TrafficPercent float64 `json:"traffic_percent"`

	// MinBakeTime is how long this stage must run (with a passing
	// DeploymentVerification) before advancing to the next one. Purely
	// informational for TrafficPercentageAt -- it does not itself
	// enforce timing, that is a caller's/orchestrator's responsibility.
	MinBakeTime time.Duration `json:"min_bake_time,omitempty"`
}

// CanaryPlan is an ordered sequence of CanaryStages a canary rollout
// progresses through, from the smallest traffic slice to 100%.
type CanaryPlan struct {
	Stages []CanaryStage `json:"stages"`
}

// Validate checks p for internal consistency: at least one stage,
// every stage's TrafficPercent within [0, 100], and percentages
// non-decreasing across stages (a canary plan that dials traffic back
// down partway through is not a rollout schedule this type
// represents).
func (p *CanaryPlan) Validate() error {
	if p == nil || len(p.Stages) == 0 {
		return wrapf("Validate", ErrEmptyCanaryStages)
	}

	prev := -1.0
	for _, stage := range p.Stages {
		if strings.TrimSpace(stage.Name) == "" {
			return wrapf("Validate", ErrInvalidTrafficPercentage)
		}
		if stage.TrafficPercent < 0 || stage.TrafficPercent > 100 {
			return wrapf("Validate", ErrInvalidTrafficPercentage)
		}
		if stage.TrafficPercent < prev {
			return wrapf("Validate", ErrInvalidTrafficPercentage)
		}
		prev = stage.TrafficPercent
	}
	return nil
}

// TrafficPercentageAt returns the percentage of traffic that should be
// routed to the new version at step index stepIndex (0-based) of p's
// Stages -- the real arithmetic task 7 calls for. stepIndex must be
// within [0, len(p.Stages)); ErrCanaryStageOutOfRange is returned
// otherwise.
//
// This is a pure lookup, not an interpolation: a canary schedule
// advances in the discrete, operator-defined increments p.Stages
// declares (e.g. 5% -> 25% -> 50% -> 100%), not a continuously
// computed ramp, since real orchestrators (a Kubernetes/Istio traffic
// split, an ALB weighted target group) apply a stepped percentage the
// same way.
func (p *CanaryPlan) TrafficPercentageAt(stepIndex int) (float64, error) {
	if p == nil || len(p.Stages) == 0 {
		return 0, wrapf("TrafficPercentageAt", ErrEmptyCanaryStages)
	}
	if stepIndex < 0 || stepIndex >= len(p.Stages) {
		return 0, wrapf("TrafficPercentageAt", ErrCanaryStageOutOfRange)
	}
	return p.Stages[stepIndex].TrafficPercent, nil
}

// RemainingPercentageAt returns 100 minus TrafficPercentageAt(stepIndex)
// -- the fraction still served by the old version at that step. A
// convenience wrapper so a caller computing both sides of a traffic
// split does not have to repeat "100 - x" at every call site.
func (p *CanaryPlan) RemainingPercentageAt(stepIndex int) (float64, error) {
	pct, err := p.TrafficPercentageAt(stepIndex)
	if err != nil {
		return 0, err
	}
	return 100 - pct, nil
}

// DefaultCanaryPlan returns a four-stage canary schedule (5% -> 25% ->
// 50% -> 100%) with a fixed bake time per stage -- a reasonable
// starting schedule a caller may use as-is or override entirely by
// constructing a CanaryPlan directly.
func DefaultCanaryPlan() CanaryPlan {
	return CanaryPlan{Stages: []CanaryStage{
		{Name: "5% canary", TrafficPercent: 5, MinBakeTime: 15 * time.Minute},
		{Name: "25% canary", TrafficPercent: 25, MinBakeTime: 30 * time.Minute},
		{Name: "50% canary", TrafficPercent: 50, MinBakeTime: 30 * time.Minute},
		{Name: "100% (fully rolled out)", TrafficPercent: 100},
	}}
}
