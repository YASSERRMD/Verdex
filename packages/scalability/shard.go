package scalability

import (
	"sort"
	"strings"
)

// ShardStrategy maps a store key (typically a tenant ID or case ID)
// to a durable shard (task 4: store partitioning/sharding strategy).
// Unlike Partitioner (partitioner.go), which distributes transient
// work items across workers, ShardStrategy answers "which physical
// store instance holds this tenant/case's durable records" -- a
// decision that must remain stable across process restarts and,
// critically, must minimize data movement when the shard count
// changes (see Rebalance).
type ShardStrategy interface {
	// ShardFor maps key (a tenant ID or case ID) to a shard index in
	// [0, N), where N is ShardCount().
	ShardFor(key string) (shardID int, err error)

	// ShardCount returns N, the number of shards this ShardStrategy
	// currently maps keys across.
	ShardCount() int
}

// consistentHashShardStrategy is a ShardStrategy using the same
// consistent-hashing ring technique as consistentHashPartitioner
// (partitioner.go), applied to durable shard assignment instead of
// transient work distribution. Consistent hashing is used here
// specifically because store data must move when shards are added or
// removed, and bounding that movement (see Rebalance) is the whole
// point -- plain modulo sharding would require re-copying nearly every
// tenant's data on every resize.
type consistentHashShardStrategy struct {
	shardCount int
	ring       []ringPoint
}

// NewConsistentHashShardStrategy constructs a ShardStrategy mapping
// keys across n shards via consistent hashing. Returns
// ErrInvalidShardCount if n <= 0.
func NewConsistentHashShardStrategy(n int) (ShardStrategy, error) {
	if n <= 0 {
		return nil, wrapf("NewConsistentHashShardStrategy", ErrInvalidShardCount)
	}

	ring := make([]ringPoint, 0, n*virtualNodesPerPartition)
	for s := 0; s < n; s++ {
		for v := 0; v < virtualNodesPerPartition; v++ {
			ring = append(ring, ringPoint{
				hash:      hashString(virtualNodeKey(s, v)),
				partition: s,
			})
		}
	}
	sort.Slice(ring, func(i, j int) bool { return ring[i].hash < ring[j].hash })

	return &consistentHashShardStrategy{shardCount: n, ring: ring}, nil
}

// ShardFor implements ShardStrategy. Returns ErrEmptyShardKey if key
// is blank.
func (s *consistentHashShardStrategy) ShardFor(key string) (int, error) {
	if strings.TrimSpace(key) == "" {
		return 0, wrapf("ShardFor", ErrEmptyShardKey)
	}
	if len(s.ring) == 0 {
		return 0, nil
	}
	h := hashString(key)
	idx := sort.Search(len(s.ring), func(i int) bool { return s.ring[i].hash >= h })
	if idx == len(s.ring) {
		idx = 0
	}
	return s.ring[idx].partition, nil
}

// ShardCount implements ShardStrategy.
func (s *consistentHashShardStrategy) ShardCount() int {
	return s.shardCount
}

// RebalancePlan is the outcome of computing which keys must move when
// a store's shard count changes from an old topology to a new one.
type RebalancePlan struct {
	// OldShardCount and NewShardCount are the shard counts compared.
	OldShardCount int
	NewShardCount int

	// Moves lists, for each key that changes shard assignment, its
	// old and new shard. Keys not present here keep their existing
	// shard assignment unchanged.
	Moves []KeyMove

	// TotalKeys is the size of the key population Rebalance was given.
	TotalKeys int

	// MovedFraction is len(Moves) / TotalKeys, or 0 if TotalKeys is 0.
	// For consistent hashing, this is expected to be close to
	// |NewShardCount-OldShardCount| / max(OldShardCount,NewShardCount)
	// rather than the near-1.0 fraction plain modulo sharding would
	// produce on almost any resize.
	MovedFraction float64
}

// KeyMove records a single key's shard reassignment.
type KeyMove struct {
	Key      string
	OldShard int
	NewShard int
}

// Rebalance computes a RebalancePlan for moving from oldShards to
// newShards shards, given the concrete population of keys currently
// stored (typically every tenant ID or case ID with data in the
// store). Both strategies must use the same hashing scheme (both
// constructed via NewConsistentHashShardStrategy, or both via the
// same alternative) for the comparison to be meaningful; Rebalance
// itself only calls ShardFor on each, so it works with any
// ShardStrategy pairing.
//
// Returns ErrInvalidShardCount if either strategy is nil.
func Rebalance(oldStrategy, newStrategy ShardStrategy, keys []string) (RebalancePlan, error) {
	if oldStrategy == nil || newStrategy == nil {
		return RebalancePlan{}, wrapf("Rebalance", ErrInvalidShardCount)
	}

	plan := RebalancePlan{
		OldShardCount: oldStrategy.ShardCount(),
		NewShardCount: newStrategy.ShardCount(),
		TotalKeys:     len(keys),
	}

	for _, key := range keys {
		oldShard, err := oldStrategy.ShardFor(key)
		if err != nil {
			continue // skip structurally invalid keys (e.g. blank)
		}
		newShard, err := newStrategy.ShardFor(key)
		if err != nil {
			continue
		}
		if oldShard != newShard {
			plan.Moves = append(plan.Moves, KeyMove{
				Key:      key,
				OldShard: oldShard,
				NewShard: newShard,
			})
		}
	}

	if plan.TotalKeys > 0 {
		plan.MovedFraction = float64(len(plan.Moves)) / float64(plan.TotalKeys)
	}

	return plan, nil
}
