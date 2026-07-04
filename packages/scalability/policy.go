package scalability

// ScalingAction names the direction of an autoscaling Decide outcome.
type ScalingAction string

const (
	// ActionScaleUp means Decide recommends increasing replica count.
	ActionScaleUp ScalingAction = "scale_up"

	// ActionScaleDown means Decide recommends decreasing replica
	// count.
	ActionScaleDown ScalingAction = "scale_down"

	// ActionHold means Decide recommends leaving replica count
	// unchanged (either the metric is within the acceptable band, or a
	// hysteresis/bound guard suppressed a change that would otherwise
	// have been indicated).
	ActionHold ScalingAction = "hold"
)

// ScalingPolicy names the bounds and target an autoscaler evaluates
// observed load against (task 5: autoscaling policies). It carries no
// history itself; Decide is a pure function of (policy, currentReplicas,
// observedMetric), so it can be called on a schedule (e.g. every
// metrics-scrape interval) without the policy object needing to track
// state between calls.
type ScalingPolicy struct {
	// MinReplicas is the minimum replica count Decide will ever
	// recommend, regardless of how low observedMetric is. Must be >=
	// 1 (a service cannot scale to zero replicas under this policy;
	// scale-to-zero is a different concern this phase does not
	// address).
	MinReplicas int

	// MaxReplicas is the maximum replica count Decide will ever
	// recommend, regardless of how high observedMetric is. Must be >=
	// MinReplicas.
	MaxReplicas int

	// TargetMetric is the desired steady-state value of the observed
	// metric (e.g. target CPU utilization as a fraction, or target
	// queue depth as an absolute count) that replica count should be
	// sized to sustain. Must be > 0.
	TargetMetric float64

	// UpperTolerance and LowerTolerance bound a "dead zone" around
	// TargetMetric within which Decide holds rather than adjusting,
	// preventing flapping on small, noisy fluctuations right at the
	// target. observedMetric must exceed
	// TargetMetric*(1+UpperTolerance) to trigger scale-up, or fall
	// below TargetMetric*(1-LowerTolerance) to trigger scale-down.
	// Both must be in [0, 1); 0 means no tolerance band (any deviation
	// from TargetMetric triggers an adjustment).
	UpperTolerance float64
	LowerTolerance float64

	// ScaleUpStep and ScaleDownStep bound how many replicas Decide
	// adds or removes in a single decision, preventing a single noisy
	// metric spike from jumping straight from MinReplicas to
	// MaxReplicas. Must be >= 1.
	ScaleUpStep   int
	ScaleDownStep int
}

// Validate reports whether p is structurally well-formed.
func (p ScalingPolicy) Validate() error {
	if p.MinReplicas < 1 {
		return wrapf("Validate", ErrInvalidScalingPolicy)
	}
	if p.MaxReplicas < p.MinReplicas {
		return wrapf("Validate", ErrInvalidScalingPolicy)
	}
	if p.TargetMetric <= 0 {
		return wrapf("Validate", ErrInvalidScalingPolicy)
	}
	if p.UpperTolerance < 0 || p.UpperTolerance >= 1 {
		return wrapf("Validate", ErrInvalidScalingPolicy)
	}
	if p.LowerTolerance < 0 || p.LowerTolerance >= 1 {
		return wrapf("Validate", ErrInvalidScalingPolicy)
	}
	if p.ScaleUpStep < 1 || p.ScaleDownStep < 1 {
		return wrapf("Validate", ErrInvalidScalingPolicy)
	}
	return nil
}

// DefaultScalingPolicy returns a conservative starter policy: 2-20
// replicas, targeting 70% of whatever metric the caller measures
// (typically CPU utilization or normalized queue depth), with a 15%
// tolerance band each side of target and single-replica steps -- slow,
// steady adjustment favored over aggressive jumps.
func DefaultScalingPolicy() ScalingPolicy {
	return ScalingPolicy{
		MinReplicas:    2,
		MaxReplicas:    20,
		TargetMetric:   0.70,
		UpperTolerance: 0.15,
		LowerTolerance: 0.15,
		ScaleUpStep:    1,
		ScaleDownStep:  1,
	}
}

// Decision is the outcome of evaluating a ScalingPolicy against an
// observed metric.
type Decision struct {
	// Action is the recommended direction.
	Action ScalingAction

	// TargetReplicas is the replica count Decide recommends moving to.
	// Equals currentReplicas when Action is ActionHold.
	TargetReplicas int

	// Reason is a short, human-readable explanation of why this
	// Action/TargetReplicas was chosen (e.g. "observed metric 0.92
	// exceeds upper bound 0.805", "at MaxReplicas, cannot scale up
	// further").
	Reason string
}

// Decide evaluates observedMetric against policy's target/tolerance
// band and returns a real scale-up/scale-down/hold Decision --
// including hysteresis (the tolerance band) so a metric oscillating
// right around TargetMetric does not cause the replica count to
// flap up and down on every evaluation.
//
// Returns ErrInvalidScalingPolicy if policy fails validation, or
// ErrInvalidCapacityInput if currentReplicas is not itself within
// [policy.MinReplicas, policy.MaxReplicas] or observedMetric is
// negative.
func Decide(policy ScalingPolicy, currentReplicas int, observedMetric float64) (Decision, error) {
	if err := policy.Validate(); err != nil {
		return Decision{}, wrapf("Decide", err)
	}
	if currentReplicas < policy.MinReplicas || currentReplicas > policy.MaxReplicas {
		return Decision{}, wrapf("Decide", ErrInvalidCapacityInput)
	}
	if observedMetric < 0 {
		return Decision{}, wrapf("Decide", ErrInvalidCapacityInput)
	}

	upperBound := policy.TargetMetric * (1 + policy.UpperTolerance)
	lowerBound := policy.TargetMetric * (1 - policy.LowerTolerance)

	switch {
	case observedMetric > upperBound:
		if currentReplicas >= policy.MaxReplicas {
			return Decision{
				Action:         ActionHold,
				TargetReplicas: currentReplicas,
				Reason:         "observed metric exceeds upper bound but already at MaxReplicas",
			}, nil
		}
		target := currentReplicas + policy.ScaleUpStep
		if target > policy.MaxReplicas {
			target = policy.MaxReplicas
		}
		return Decision{
			Action:         ActionScaleUp,
			TargetReplicas: target,
			Reason:         "observed metric exceeds upper tolerance bound",
		}, nil

	case observedMetric < lowerBound:
		if currentReplicas <= policy.MinReplicas {
			return Decision{
				Action:         ActionHold,
				TargetReplicas: currentReplicas,
				Reason:         "observed metric below lower bound but already at MinReplicas",
			}, nil
		}
		target := currentReplicas - policy.ScaleDownStep
		if target < policy.MinReplicas {
			target = policy.MinReplicas
		}
		return Decision{
			Action:         ActionScaleDown,
			TargetReplicas: target,
			Reason:         "observed metric below lower tolerance bound",
		}, nil

	default:
		return Decision{
			Action:         ActionHold,
			TargetReplicas: currentReplicas,
			Reason:         "observed metric within tolerance band around target",
		}, nil
	}
}
