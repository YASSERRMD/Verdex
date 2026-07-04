package scalability

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewConsistentHashShardStrategyInvalidCount(t *testing.T) {
	_, err := NewConsistentHashShardStrategy(0)
	if !errors.Is(err, ErrInvalidShardCount) {
		t.Fatalf("expected ErrInvalidShardCount, got %v", err)
	}
	_, err = NewConsistentHashShardStrategy(-3)
	if !errors.Is(err, ErrInvalidShardCount) {
		t.Fatalf("expected ErrInvalidShardCount, got %v", err)
	}
}

func TestShardForEmptyKey(t *testing.T) {
	s, err := NewConsistentHashShardStrategy(4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = s.ShardFor("")
	if !errors.Is(err, ErrEmptyShardKey) {
		t.Fatalf("expected ErrEmptyShardKey, got %v", err)
	}
	_, err = s.ShardFor("   ")
	if !errors.Is(err, ErrEmptyShardKey) {
		t.Fatalf("expected ErrEmptyShardKey for whitespace-only key, got %v", err)
	}
}

func TestShardForBoundsAndDeterminism(t *testing.T) {
	s, err := NewConsistentHashShardStrategy(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ShardCount() != 5 {
		t.Fatalf("expected ShardCount=5, got %d", s.ShardCount())
	}

	first, err := s.ShardFor("tenant-acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first < 0 || first >= 5 {
		t.Fatalf("ShardFor returned out-of-range shard %d", first)
	}
	for i := 0; i < 20; i++ {
		got, err := s.ShardFor("tenant-acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != first {
			t.Fatalf("ShardFor not deterministic: got %d, want %d", got, first)
		}
	}
}

func TestRebalanceNilStrategy(t *testing.T) {
	s, _ := NewConsistentHashShardStrategy(4)
	_, err := Rebalance(nil, s, []string{"a"})
	if !errors.Is(err, ErrInvalidShardCount) {
		t.Fatalf("expected ErrInvalidShardCount for nil old strategy, got %v", err)
	}
	_, err = Rebalance(s, nil, []string{"a"})
	if !errors.Is(err, ErrInvalidShardCount) {
		t.Fatalf("expected ErrInvalidShardCount for nil new strategy, got %v", err)
	}
}

func TestRebalanceNoChangeWhenShardCountUnchanged(t *testing.T) {
	oldS, _ := NewConsistentHashShardStrategy(4)
	newS, _ := NewConsistentHashShardStrategy(4)

	keys := make([]string, 200)
	for i := range keys {
		keys[i] = fmt.Sprintf("tenant-%d", i)
	}

	plan, err := Rebalance(oldS, newS, keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Moves) != 0 {
		t.Fatalf("expected zero moves for identical shard counts/strategies, got %d", len(plan.Moves))
	}
	if plan.MovedFraction != 0 {
		t.Fatalf("expected MovedFraction=0, got %v", plan.MovedFraction)
	}
}

// TestRebalanceBoundsMovementOnGrowth is the key real-logic assertion
// for task 4: growing shard count from N to M via consistent hashing
// should move dramatically fewer keys than a naive full reshuffle
// (plain modulo sharding remaps close to (M-1)/M of all keys on
// almost any resize). This proves Rebalance's consistent-hashing
// strategies actually bound movement, not just report a number.
func TestRebalanceBoundsMovementOnGrowth(t *testing.T) {
	const totalKeys = 5000
	keys := make([]string, totalKeys)
	for i := range keys {
		keys[i] = fmt.Sprintf("case-%d", i)
	}

	oldS, err := NewConsistentHashShardStrategy(8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	newS, err := NewConsistentHashShardStrategy(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plan, err := Rebalance(oldS, newS, keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.OldShardCount != 8 || plan.NewShardCount != 10 {
		t.Fatalf("expected OldShardCount=8 NewShardCount=10, got %d/%d", plan.OldShardCount, plan.NewShardCount)
	}
	if plan.TotalKeys != totalKeys {
		t.Fatalf("expected TotalKeys=%d, got %d", totalKeys, plan.TotalKeys)
	}

	// Adding 2 shards to 8 (a 25% capacity increase) should move
	// roughly proportional to the added fraction, not anywhere near
	// "almost everything." Naive modulo sharding remapping 8->10
	// would move the vast majority of keys (any key whose
	// hash%8 != hash%10). Allow generous headroom (up to 60%
	// moved) while still proving this is meaningfully bounded, not a
	// full reshuffle.
	if plan.MovedFraction <= 0 {
		t.Fatal("expected at least some keys to move when shard count grows")
	}
	if plan.MovedFraction > 0.60 {
		t.Fatalf("expected bounded movement (<=0.60 moved), got %.3f -- consistent hashing should not reshuffle nearly everything", plan.MovedFraction)
	}

	// Sanity: every reported move must actually change shard, and
	// NewShard must be in range.
	for _, mv := range plan.Moves {
		if mv.OldShard == mv.NewShard {
			t.Fatalf("KeyMove %q reports no-op move %d->%d", mv.Key, mv.OldShard, mv.NewShard)
		}
		if mv.NewShard < 0 || mv.NewShard >= 10 {
			t.Fatalf("KeyMove %q NewShard=%d out of range [0,10)", mv.Key, mv.NewShard)
		}
	}
}

// TestRebalanceNaiveModuloMovesNearlyEverything demonstrates the
// contrast: plain modulo hashing on the same resize moves close to
// 100% of keys, motivating why ShardStrategy uses consistent hashing
// rather than modulo for durable store data.
func TestRebalanceNaiveModuloMovesNearlyEverything(t *testing.T) {
	const totalKeys = 5000
	keys := make([]string, totalKeys)
	for i := range keys {
		keys[i] = fmt.Sprintf("case-%d", i)
	}

	oldMod := &moduloShardAdapter{p: mustModulo(t, 8)}
	newMod := &moduloShardAdapter{p: mustModulo(t, 10)}

	plan, err := Rebalance(oldMod, newMod, keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Modulo resharding from 8->10 should move the overwhelming
	// majority of keys (only keys where hash%8==hash%10 stay put,
	// which is rare). This asserts the contrast is real, not assumed.
	if plan.MovedFraction < 0.70 {
		t.Fatalf("expected modulo resharding to move >=0.70 of keys as a naive-baseline contrast, got %.3f", plan.MovedFraction)
	}
}

// moduloShardAdapter adapts the Partitioner-shaped moduloPartitioner
// to the ShardStrategy interface, purely so
// TestRebalanceNaiveModuloMovesNearlyEverything can demonstrate the
// contrast using Rebalance's real logic rather than a hand-rolled
// comparison.
type moduloShardAdapter struct {
	p Partitioner
}

func (m *moduloShardAdapter) ShardFor(key string) (int, error) {
	if key == "" {
		return 0, ErrEmptyShardKey
	}
	return m.p.Partition(key), nil
}

func (m *moduloShardAdapter) ShardCount() int {
	return m.p.PartitionCount()
}

func mustModulo(t *testing.T, n int) Partitioner {
	t.Helper()
	p, err := NewModuloPartitioner(n)
	if err != nil {
		t.Fatalf("unexpected error constructing modulo partitioner: %v", err)
	}
	return p
}
