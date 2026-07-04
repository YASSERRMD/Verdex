package scalability

import "math"

// CapacityInput is the historical/observed data a capacity-planning
// estimate is computed from (task 7). This mirrors the shape of
// packages/perf's Budget (a target to meet) and Measurement (an
// observed sample) by name/tag, as the phase brief directs, rather
// than importing packages/perf: this package has no dependency on
// packages/perf, and CapacityInput's fields describe throughput and
// SLA headroom, not packages/perf's latency-percentile-specific
// Budget/Measurement/Verdict shape, so a direct type reuse would be a
// poor fit even if the import were trivial.
type CapacityInput struct {
	// ObservedThroughputPerReplica is the sustained throughput (in
	// operations per second) a single replica has been observed to
	// handle historically -- the per-replica analog of
	// perf.Measurement.Throughput. Must be > 0.
	ObservedThroughputPerReplica float64

	// TargetThroughput is the total sustained throughput (in
	// operations per second) the deployment must support to meet its
	// SLA -- the capacity-planning analog of a perf.Budget's
	// MinThroughput, but stated as a whole-deployment target rather
	// than a single named operation's floor. Must be > 0.
	TargetThroughput float64

	// HeadroomFraction is the fraction of extra capacity to provision
	// above the bare TargetThroughput/ObservedThroughputPerReplica
	// ratio, absorbing normal variance (traffic bursts, one replica
	// briefly unhealthy, GC pauses) without immediately breaching
	// SLA. Must be >= 0. A value of 0.20 provisions 20% extra
	// replicas beyond the bare-minimum ratio.
	HeadroomFraction float64
}

// Validate reports whether in is structurally well-formed.
func (in CapacityInput) Validate() error {
	if in.ObservedThroughputPerReplica <= 0 {
		return wrapf("Validate", ErrInvalidCapacityInput)
	}
	if in.TargetThroughput <= 0 {
		return wrapf("Validate", ErrInvalidCapacityInput)
	}
	if in.HeadroomFraction < 0 {
		return wrapf("Validate", ErrInvalidCapacityInput)
	}
	return nil
}

// CapacityEstimate is the outcome of EstimateReplicas.
type CapacityEstimate struct {
	// BaseReplicas is the bare-minimum replica count needed to sustain
	// TargetThroughput at ObservedThroughputPerReplica per replica,
	// before headroom: ceil(TargetThroughput /
	// ObservedThroughputPerReplica).
	BaseReplicas int

	// RecommendedReplicas is BaseReplicas scaled up by
	// HeadroomFraction and rounded up: the number of replicas this
	// model recommends provisioning.
	RecommendedReplicas int

	// EffectiveThroughputAtRecommended is RecommendedReplicas *
	// ObservedThroughputPerReplica -- the actual sustained throughput
	// the recommended replica count provides, useful for confirming
	// how much headroom above TargetThroughput the recommendation
	// actually delivers (it will typically exceed
	// TargetThroughput*(1+HeadroomFraction) slightly, since replica
	// count is a whole number).
	EffectiveThroughputAtRecommended float64
}

// EstimateReplicas computes a CapacityEstimate from historical
// per-replica throughput and a target SLA throughput, applying
// HeadroomFraction as a safety margin -- a real arithmetic model, not
// a guess. Returns ErrInvalidCapacityInput if in fails validation.
func EstimateReplicas(in CapacityInput) (CapacityEstimate, error) {
	if err := in.Validate(); err != nil {
		return CapacityEstimate{}, wrapf("EstimateReplicas", err)
	}

	baseReplicas := int(math.Ceil(in.TargetThroughput / in.ObservedThroughputPerReplica))
	if baseReplicas < 1 {
		baseReplicas = 1
	}

	withHeadroom := float64(baseReplicas) * (1 + in.HeadroomFraction)
	recommendedReplicas := int(math.Ceil(withHeadroom))
	if recommendedReplicas < baseReplicas {
		recommendedReplicas = baseReplicas
	}

	return CapacityEstimate{
		BaseReplicas:                     baseReplicas,
		RecommendedReplicas:              recommendedReplicas,
		EffectiveThroughputAtRecommended: float64(recommendedReplicas) * in.ObservedThroughputPerReplica,
	}, nil
}
