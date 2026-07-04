package scalability

import (
	"errors"
	"testing"
)

func TestCapacityInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CapacityInput
		wantErr bool
	}{
		{"valid", CapacityInput{ObservedThroughputPerReplica: 10, TargetThroughput: 100, HeadroomFraction: 0.2}, false},
		{"zero observed throughput", CapacityInput{ObservedThroughputPerReplica: 0, TargetThroughput: 100}, true},
		{"negative observed throughput", CapacityInput{ObservedThroughputPerReplica: -1, TargetThroughput: 100}, true},
		{"zero target throughput", CapacityInput{ObservedThroughputPerReplica: 10, TargetThroughput: 0}, true},
		{"negative headroom", CapacityInput{ObservedThroughputPerReplica: 10, TargetThroughput: 100, HeadroomFraction: -0.1}, true},
		{"zero headroom is valid", CapacityInput{ObservedThroughputPerReplica: 10, TargetThroughput: 100, HeadroomFraction: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEstimateReplicasInvalidInput(t *testing.T) {
	_, err := EstimateReplicas(CapacityInput{})
	if !errors.Is(err, ErrInvalidCapacityInput) {
		t.Fatalf("expected ErrInvalidCapacityInput, got %v", err)
	}
}

// TestEstimateReplicasExactDivision covers the case where target
// throughput divides evenly by per-replica throughput.
func TestEstimateReplicasExactDivision(t *testing.T) {
	est, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 20,
		TargetThroughput:             100,
		HeadroomFraction:             0, // no headroom, so this isolates the base ratio
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.BaseReplicas != 5 {
		t.Fatalf("expected BaseReplicas=5, got %d", est.BaseReplicas)
	}
	if est.RecommendedReplicas != 5 {
		t.Fatalf("expected RecommendedReplicas=5 with zero headroom, got %d", est.RecommendedReplicas)
	}
	if est.EffectiveThroughputAtRecommended != 100 {
		t.Fatalf("expected EffectiveThroughputAtRecommended=100, got %v", est.EffectiveThroughputAtRecommended)
	}
}

// TestEstimateReplicasRoundsUpFractionalReplicas covers the case
// where target/per-replica does not divide evenly -- a fractional
// replica count must round up (you cannot provision half a replica
// and still meet the SLA).
func TestEstimateReplicasRoundsUpFractionalReplicas(t *testing.T) {
	est, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 30,
		TargetThroughput:             100, // 100/30 = 3.33...
		HeadroomFraction:             0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.BaseReplicas != 4 {
		t.Fatalf("expected BaseReplicas=4 (ceil(3.33)), got %d", est.BaseReplicas)
	}
}

// TestEstimateReplicasAppliesHeadroom confirms HeadroomFraction
// actually inflates the recommendation above the bare ratio.
func TestEstimateReplicasAppliesHeadroom(t *testing.T) {
	est, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 10,
		TargetThroughput:             100, // base = 10 replicas
		HeadroomFraction:             0.20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.BaseReplicas != 10 {
		t.Fatalf("expected BaseReplicas=10, got %d", est.BaseReplicas)
	}
	// 10 * 1.20 = 12 exactly.
	if est.RecommendedReplicas != 12 {
		t.Fatalf("expected RecommendedReplicas=12 (10 base * 1.20 headroom), got %d", est.RecommendedReplicas)
	}
	if est.RecommendedReplicas <= est.BaseReplicas {
		t.Fatalf("expected RecommendedReplicas (%d) > BaseReplicas (%d) with positive headroom", est.RecommendedReplicas, est.BaseReplicas)
	}
	wantEffective := 12.0 * 10.0
	if est.EffectiveThroughputAtRecommended != wantEffective {
		t.Fatalf("expected EffectiveThroughputAtRecommended=%v, got %v", wantEffective, est.EffectiveThroughputAtRecommended)
	}
	// Effective throughput at the recommendation must meet or exceed
	// the target with headroom applied.
	if est.EffectiveThroughputAtRecommended < 100*1.20 {
		t.Fatalf("expected effective throughput >= target*headroom (120), got %v", est.EffectiveThroughputAtRecommended)
	}
}

// TestEstimateReplicasHeadroomRoundsUp confirms fractional headroom
// results still round up to a whole replica, never truncating below
// what's needed to meet the SLA.
func TestEstimateReplicasHeadroomRoundsUp(t *testing.T) {
	est, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 100,
		TargetThroughput:             1000, // base = 10 replicas
		HeadroomFraction:             0.05, // 10 * 1.05 = 10.5 -> ceil to 11
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.BaseReplicas != 10 {
		t.Fatalf("expected BaseReplicas=10, got %d", est.BaseReplicas)
	}
	if est.RecommendedReplicas != 11 {
		t.Fatalf("expected RecommendedReplicas=11 (ceil(10.5)), got %d", est.RecommendedReplicas)
	}
}

// TestEstimateReplicasMinimumOneReplica confirms a tiny target
// throughput still recommends at least one replica, never zero.
func TestEstimateReplicasMinimumOneReplica(t *testing.T) {
	est, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 1000,
		TargetThroughput:             1, // far below one replica's capacity
		HeadroomFraction:             0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.BaseReplicas != 1 {
		t.Fatalf("expected BaseReplicas=1 (minimum), got %d", est.BaseReplicas)
	}
	if est.RecommendedReplicas != 1 {
		t.Fatalf("expected RecommendedReplicas=1, got %d", est.RecommendedReplicas)
	}
}

// TestEstimateReplicasRealWorldScenario models a plausible ingestion-
// pipeline capacity plan: packages/perf's DefaultBudgets documents
// OpIngestionPipeline's MinThroughput as 5 ops/sec; if a single
// replica has been observed sustaining 2 ops/sec historically and the
// deployment needs to meet that 5 ops/sec SLA floor with 25% headroom
// for burst traffic, this asserts the model recommends a sane,
// specific replica count.
func TestEstimateReplicasRealWorldScenario(t *testing.T) {
	est, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 2,
		TargetThroughput:             5,
		HeadroomFraction:             0.25,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// base = ceil(5/2) = 3; with 25% headroom = ceil(3.75) = 4
	if est.BaseReplicas != 3 {
		t.Fatalf("expected BaseReplicas=3, got %d", est.BaseReplicas)
	}
	if est.RecommendedReplicas != 4 {
		t.Fatalf("expected RecommendedReplicas=4, got %d", est.RecommendedReplicas)
	}
	if est.EffectiveThroughputAtRecommended < 5 {
		t.Fatalf("expected recommended replicas to sustain at least the 5 ops/sec target, got %v", est.EffectiveThroughputAtRecommended)
	}
}
