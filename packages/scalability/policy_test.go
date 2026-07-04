package scalability

import (
	"errors"
	"testing"
)

func TestScalingPolicyValidate(t *testing.T) {
	tests := []struct {
		name    string
		policy  ScalingPolicy
		wantErr bool
	}{
		{"valid default", DefaultScalingPolicy(), false},
		{"min replicas zero", ScalingPolicy{MinReplicas: 0, MaxReplicas: 5, TargetMetric: 0.5, ScaleUpStep: 1, ScaleDownStep: 1}, true},
		{"max less than min", ScalingPolicy{MinReplicas: 5, MaxReplicas: 2, TargetMetric: 0.5, ScaleUpStep: 1, ScaleDownStep: 1}, true},
		{"target metric zero", ScalingPolicy{MinReplicas: 1, MaxReplicas: 5, TargetMetric: 0, ScaleUpStep: 1, ScaleDownStep: 1}, true},
		{"upper tolerance negative", ScalingPolicy{MinReplicas: 1, MaxReplicas: 5, TargetMetric: 0.5, UpperTolerance: -0.1, ScaleUpStep: 1, ScaleDownStep: 1}, true},
		{"upper tolerance too large", ScalingPolicy{MinReplicas: 1, MaxReplicas: 5, TargetMetric: 0.5, UpperTolerance: 1.0, ScaleUpStep: 1, ScaleDownStep: 1}, true},
		{"scale up step zero", ScalingPolicy{MinReplicas: 1, MaxReplicas: 5, TargetMetric: 0.5, ScaleUpStep: 0, ScaleDownStep: 1}, true},
		{"scale down step zero", ScalingPolicy{MinReplicas: 1, MaxReplicas: 5, TargetMetric: 0.5, ScaleUpStep: 1, ScaleDownStep: 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDecideInvalidPolicy(t *testing.T) {
	_, err := Decide(ScalingPolicy{}, 1, 0.5)
	if !errors.Is(err, ErrInvalidScalingPolicy) {
		t.Fatalf("expected ErrInvalidScalingPolicy, got %v", err)
	}
}

func TestDecideCurrentReplicasOutOfBounds(t *testing.T) {
	policy := DefaultScalingPolicy() // Min=2, Max=20
	_, err := Decide(policy, 1, 0.5)
	if !errors.Is(err, ErrInvalidCapacityInput) {
		t.Fatalf("expected ErrInvalidCapacityInput for below-min replicas, got %v", err)
	}
	_, err = Decide(policy, 21, 0.5)
	if !errors.Is(err, ErrInvalidCapacityInput) {
		t.Fatalf("expected ErrInvalidCapacityInput for above-max replicas, got %v", err)
	}
}

func TestDecideNegativeMetric(t *testing.T) {
	policy := DefaultScalingPolicy()
	_, err := Decide(policy, 5, -0.1)
	if !errors.Is(err, ErrInvalidCapacityInput) {
		t.Fatalf("expected ErrInvalidCapacityInput for negative metric, got %v", err)
	}
}

func TestDecideScaleUpAboveUpperBound(t *testing.T) {
	policy := DefaultScalingPolicy() // target 0.70, +15% tolerance => upper bound 0.805
	decision, err := Decide(policy, 5, 0.90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionScaleUp {
		t.Fatalf("expected ActionScaleUp, got %v (%s)", decision.Action, decision.Reason)
	}
	if decision.TargetReplicas != 6 {
		t.Fatalf("expected TargetReplicas=6, got %d", decision.TargetReplicas)
	}
}

func TestDecideScaleDownBelowLowerBound(t *testing.T) {
	policy := DefaultScalingPolicy() // target 0.70, -15% tolerance => lower bound 0.595
	decision, err := Decide(policy, 8, 0.30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionScaleDown {
		t.Fatalf("expected ActionScaleDown, got %v (%s)", decision.Action, decision.Reason)
	}
	if decision.TargetReplicas != 7 {
		t.Fatalf("expected TargetReplicas=7, got %d", decision.TargetReplicas)
	}
}

func TestDecideHoldWithinToleranceBand(t *testing.T) {
	policy := DefaultScalingPolicy() // band is [0.595, 0.805]
	for _, metric := range []float64{0.60, 0.70, 0.80} {
		decision, err := Decide(policy, 5, metric)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if decision.Action != ActionHold {
			t.Fatalf("metric=%v: expected ActionHold, got %v (%s)", metric, decision.Action, decision.Reason)
		}
		if decision.TargetReplicas != 5 {
			t.Fatalf("metric=%v: expected TargetReplicas=5 (unchanged), got %d", metric, decision.TargetReplicas)
		}
	}
}

func TestDecideClampsAtMaxReplicas(t *testing.T) {
	policy := DefaultScalingPolicy() // Max=20
	decision, err := Decide(policy, 20, 0.99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionHold {
		t.Fatalf("expected ActionHold at MaxReplicas ceiling, got %v", decision.Action)
	}
	if decision.TargetReplicas != 20 {
		t.Fatalf("expected TargetReplicas=20, got %d", decision.TargetReplicas)
	}
}

func TestDecideClampsAtMinReplicas(t *testing.T) {
	policy := DefaultScalingPolicy() // Min=2
	decision, err := Decide(policy, 2, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionHold {
		t.Fatalf("expected ActionHold at MinReplicas floor, got %v", decision.Action)
	}
	if decision.TargetReplicas != 2 {
		t.Fatalf("expected TargetReplicas=2, got %d", decision.TargetReplicas)
	}
}

func TestDecideStepClampedToMaxReplicas(t *testing.T) {
	policy := DefaultScalingPolicy()
	policy.ScaleUpStep = 5
	// currentReplicas=18, step=5 would overshoot Max=20; must clamp.
	decision, err := Decide(policy, 18, 0.95)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionScaleUp {
		t.Fatalf("expected ActionScaleUp, got %v", decision.Action)
	}
	if decision.TargetReplicas != 20 {
		t.Fatalf("expected TargetReplicas clamped to 20, got %d", decision.TargetReplicas)
	}
}

func TestDecideStepClampedToMinReplicas(t *testing.T) {
	policy := DefaultScalingPolicy()
	policy.ScaleDownStep = 5
	// currentReplicas=4, step=5 would undershoot Min=2; must clamp.
	decision, err := Decide(policy, 4, 0.05)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionScaleDown {
		t.Fatalf("expected ActionScaleDown, got %v", decision.Action)
	}
	if decision.TargetReplicas != 2 {
		t.Fatalf("expected TargetReplicas clamped to 2, got %d", decision.TargetReplicas)
	}
}

// TestDecideFlappingPrevention is the brief's explicitly required
// flapping-prevention scenario: a metric oscillating just above and
// just below TargetMetric (but within the tolerance band) must never
// trigger a scale action across a whole sequence of evaluations, only
// ActionHold. Without the tolerance band this exact oscillation
// pattern (0.70 +/- small jitter) would otherwise flap the replica
// count up and down on every single evaluation.
func TestDecideFlappingPrevention(t *testing.T) {
	policy := DefaultScalingPolicy() // target=0.70, band=[0.595,0.805]
	currentReplicas := 6

	// Jitter that stays inside the tolerance band: 0.65, 0.75, 0.68,
	// 0.72, 0.66, 0.74 -- all within [0.595, 0.805].
	jitterSequence := []float64{0.65, 0.75, 0.68, 0.72, 0.66, 0.74, 0.70, 0.80, 0.60}

	for i, metric := range jitterSequence {
		decision, err := Decide(policy, currentReplicas, metric)
		if err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
		if decision.Action != ActionHold {
			t.Fatalf("step %d: metric=%v triggered %v (expected ActionHold to prevent flapping); reason=%q",
				i, metric, decision.Action, decision.Reason)
		}
		if decision.TargetReplicas != currentReplicas {
			t.Fatalf("step %d: expected replica count to stay at %d, got %d", i, currentReplicas, decision.TargetReplicas)
		}
		// currentReplicas intentionally never updated across
		// iterations here (ActionHold means it wouldn't change
		// anyway) -- this asserts stability, not just one lucky call.
	}
}

// TestDecideRealScalingScenario walks through a plausible real
// autoscaling episode: sustained high load triggers repeated scale-ups
// up to a point, then load drops and repeated scale-downs bring it
// back down, exercising Decide across multiple sequential scenarios
// as the brief requires ("tested across multiple scenarios").
func TestDecideRealScalingScenario(t *testing.T) {
	policy := DefaultScalingPolicy() // Min=2 Max=20 step=1
	replicas := 2

	// Sustained high load: scale up repeatedly.
	for i := 0; i < 4; i++ {
		decision, err := Decide(policy, replicas, 0.95)
		if err != nil {
			t.Fatalf("scale-up iteration %d: unexpected error: %v", i, err)
		}
		if decision.Action != ActionScaleUp {
			t.Fatalf("scale-up iteration %d: expected ActionScaleUp, got %v", i, decision.Action)
		}
		if decision.TargetReplicas != replicas+1 {
			t.Fatalf("scale-up iteration %d: expected TargetReplicas=%d, got %d", i, replicas+1, decision.TargetReplicas)
		}
		replicas = decision.TargetReplicas
	}
	if replicas != 6 {
		t.Fatalf("expected replicas=6 after 4 scale-ups from 2, got %d", replicas)
	}

	// Load subsides: scale down repeatedly.
	for i := 0; i < 3; i++ {
		decision, err := Decide(policy, replicas, 0.10)
		if err != nil {
			t.Fatalf("scale-down iteration %d: unexpected error: %v", i, err)
		}
		if decision.Action != ActionScaleDown {
			t.Fatalf("scale-down iteration %d: expected ActionScaleDown, got %v", i, decision.Action)
		}
		replicas = decision.TargetReplicas
	}
	if replicas != 3 {
		t.Fatalf("expected replicas=3 after 3 scale-downs from 6, got %d", replicas)
	}

	// Load stabilizes near target: hold.
	decision, err := Decide(policy, replicas, 0.70)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionHold {
		t.Fatalf("expected ActionHold once load stabilizes at target, got %v", decision.Action)
	}
}
