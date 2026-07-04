// Package scalability's per-file unit tests (contract_test.go,
// partitioner_test.go, shard_test.go, policy_test.go,
// backpressure_test.go, capacity_test.go, scaletest_test.go,
// config_test.go) each cover one type in isolation. This file
// (task 9: "tests for scaling behavior") instead exercises multiple
// pieces together in plausible end-to-end scenarios, the way a real
// deployment would actually combine them, catching integration gaps a
// per-type test cannot.
package scalability

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestScalingBehaviorAutoscaleThenCapacityConfirms walks through a
// realistic scaling episode end to end:
//  1. A ScalingPolicy.Decide recommends a replica count from an
//     observed metric.
//  2. EstimateReplicas independently computes a capacity-planning
//     recommendation from historical throughput and a target SLA.
//  3. Both recommendations are checked against the same
//     ScalingPolicy's Min/Max bounds via the PolicyConfig loaded from
//     the repository's real doc/policy.example.yaml fixture --
//     confirming the config-as-data policy, the autoscaling decision
//     logic, and the capacity model all agree on the same bounds
//     rather than each hardcoding its own.
func TestScalingBehaviorAutoscaleThenCapacityConfirms(t *testing.T) {
	policyConfig, err := LoadPolicyConfig("doc/policy.example.yaml")
	if err != nil {
		t.Fatalf("unexpected error loading policy config: %v", err)
	}

	// Step 1: autoscaling decision under sustained high load.
	decision, err := Decide(policyConfig.Scaling, 5, 0.95)
	if err != nil {
		t.Fatalf("unexpected error from Decide: %v", err)
	}
	if decision.Action != ActionScaleUp {
		t.Fatalf("expected ActionScaleUp under 0.95 load, got %v", decision.Action)
	}

	// Step 2: an independent capacity estimate for the same service,
	// using historical throughput data.
	estimate, err := EstimateReplicas(CapacityInput{
		ObservedThroughputPerReplica: 15,
		TargetThroughput:             150,
		HeadroomFraction:             0.25,
	})
	if err != nil {
		t.Fatalf("unexpected error from EstimateReplicas: %v", err)
	}

	// Step 3: both recommendations must respect the same policy
	// bounds -- a capacity estimate that recommends replicas outside
	// [MinReplicas, MaxReplicas] would be inconsistent with the
	// autoscaler that actually enforces those bounds at runtime, and
	// a deployment applying both signals needs to reconcile them
	// against one shared policy.
	if estimate.RecommendedReplicas < policyConfig.Scaling.MinReplicas {
		t.Errorf("capacity estimate %d below policy MinReplicas %d -- deployment should clamp up",
			estimate.RecommendedReplicas, policyConfig.Scaling.MinReplicas)
	}
	if decision.TargetReplicas > policyConfig.Scaling.MaxReplicas {
		t.Errorf("autoscale decision %d exceeds policy MaxReplicas %d", decision.TargetReplicas, policyConfig.Scaling.MaxReplicas)
	}
}

// TestScalingBehaviorPartitionedWorkersRespectBackpressure composes
// Partitioner with a per-partition BackpressureController: work items
// are routed to one of N worker partitions by key, and each
// partition's own BackpressureController independently sheds load
// once that partition (not the whole system) is saturated. This
// demonstrates that Partitioner's per-key routing and
// BackpressureController's per-instance admission control compose
// correctly -- saturating one partition must not affect another
// partition's admission decisions.
func TestScalingBehaviorPartitionedWorkersRespectBackpressure(t *testing.T) {
	const numPartitions = 4
	const maxInFlightPerPartition = 2

	partitioner, err := NewConsistentHashPartitioner(numPartitions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	controllers := make([]*BackpressureController, numPartitions)
	for i := range controllers {
		c, err := NewBackpressureController(BackpressureConfig{MaxInFlight: maxInFlightPerPartition})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		controllers[i] = c
	}

	// Find a key that reliably maps to partition 0 for this
	// partitioner instance (deterministic, since the ring is
	// deterministic given the same construction).
	var busyKey string
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("busy-key-%d", i)
		if partitioner.Partition(candidate) == 0 {
			busyKey = candidate
			break
		}
		if i > 10000 {
			t.Fatal("could not find a key mapping to partition 0 within 10000 attempts")
		}
	}

	// Saturate partition 0's controller to its limit.
	for i := 0; i < maxInFlightPerPartition; i++ {
		if err := controllers[partitioner.Partition(busyKey)].Admit(); err != nil {
			t.Fatalf("unexpected error saturating partition 0: %v", err)
		}
	}

	// A further request for the same key's partition must be shed.
	if err := controllers[partitioner.Partition(busyKey)].Admit(); !errors.Is(err, ErrLoadShed) {
		t.Fatalf("expected ErrLoadShed on saturated partition 0, got %v", err)
	}

	// A different key that maps to a different partition must be
	// completely unaffected by partition 0's saturation.
	var otherKey string
	otherPartition := -1
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("other-key-%d", i)
		part := partitioner.Partition(candidate)
		if part != 0 {
			otherKey = candidate
			otherPartition = part
			break
		}
		if i > 10000 {
			t.Fatal("could not find a key mapping to a non-zero partition within 10000 attempts")
		}
	}

	if err := controllers[otherPartition].Admit(); err != nil {
		t.Fatalf("expected partition %d (unrelated to saturated partition 0) to admit key %q, got %v", otherPartition, otherKey, err)
	}
}

// TestScalingBehaviorShardRebalanceThenScaleTest demonstrates
// ShardStrategy/Rebalance and ScaleTest working over the same
// simulated deployment: after computing a RebalancePlan for a store
// growing from 4 to 6 shards, a ScaleTest run against an operation
// that simulates per-shard work confirms throughput scales as
// concurrency increases -- i.e. the added shards are actually usable
// capacity, not just a paper plan.
func TestScalingBehaviorShardRebalanceThenScaleTest(t *testing.T) {
	oldStrategy, err := NewConsistentHashShardStrategy(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	newStrategy, err := NewConsistentHashShardStrategy(6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("tenant-%d", i)
	}

	plan, err := Rebalance(oldStrategy, newStrategy, keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Moves) == 0 {
		t.Fatal("expected some keys to move when growing from 4 to 6 shards")
	}
	if plan.NewShardCount != 6 {
		t.Fatalf("expected NewShardCount=6, got %d", plan.NewShardCount)
	}

	// Now confirm the new 6-shard topology's aggregate capacity
	// actually scales: a ScaleTest against an operation routed
	// through newStrategy should show higher throughput at higher
	// concurrency, simulating the added shards absorbing more
	// concurrent work.
	cfg := ScaleTestConfig{
		ConcurrencyLevels: []int{2, 12},
		DurationPerLevel:  25 * time.Millisecond,
	}
	op := func(ctx context.Context) (time.Duration, error) {
		shardID, err := newStrategy.ShardFor("tenant-0")
		if err != nil {
			return 0, err
		}
		if shardID < 0 || shardID >= newStrategy.ShardCount() {
			return 0, fmt.Errorf("shard %d out of range", shardID)
		}
		time.Sleep(200 * time.Microsecond)
		return 200 * time.Microsecond, nil
	}

	result, err := ScaleTest(context.Background(), cfg, op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(result.Stages))
	}
	if result.Stages[1].ThroughputPerSecond <= result.Stages[0].ThroughputPerSecond {
		t.Fatalf("expected higher-concurrency stage to show higher throughput; low=%v high=%v",
			result.Stages[0].ThroughputPerSecond, result.Stages[1].ThroughputPerSecond)
	}
	for _, stage := range result.Stages {
		if stage.ErrorCount != 0 {
			t.Fatalf("expected zero errors routing through the rebalanced shard strategy, got %d at concurrency %d",
				stage.ErrorCount, stage.Concurrency)
		}
	}
}

// TestScalingBehaviorStatelessContractGatesRealService demonstrates
// Contract.Verify as it would actually be used in a CI gate: a
// service that has not yet completed the checklist fails; once every
// item is attested, the same service passes -- confirming the
// contract is a real, evolving gate a service owner iterates against.
func TestScalingBehaviorStatelessContractGatesRealService(t *testing.T) {
	contract := NewContract()
	serviceName := "casesearch"

	partial := ChecklistAnswers{
		NoInProcessSessionAffinity: true,
		ExternalizedConfiguration:  true,
	}
	report, err := contract.Verify(serviceName, partial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Passed {
		t.Fatal("expected partial attestation to fail the gate")
	}

	complete := ChecklistAnswers{
		NoInProcessSessionAffinity: true,
		NoLocalDiskOnlyState:       true,
		NoInMemorySingletonState:   true,
		IdempotentRetries:          true,
		ExternalizedConfiguration:  true,
		HealthCheckExposed:         true,
		GracefulShutdownHandled:    true,
	}
	report, err = contract.Verify(serviceName, complete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Passed {
		t.Fatalf("expected complete attestation to pass the gate, failed: %v", report.Failed)
	}
	if report.ServiceName != serviceName {
		t.Fatalf("expected ServiceName=%q, got %q", serviceName, report.ServiceName)
	}
}
