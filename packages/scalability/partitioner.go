package scalability

import (
	"hash/fnv"
	"sort"
	"strconv"
)

// Partitioner maps a work-item key to one of N logical partitions
// (task 2: queue-based workload distribution). A partition typically
// corresponds to a worker, a shard of a downstream store, or a
// consumer-group member; Partitioner itself does not care which --
// it only answers "which of the N buckets does this key belong to."
//
// Partitioner composes with, but does not replace,
// packages/ingestion's JobQueue (Phase 029): JobQueue is the
// single-pipeline handoff between a producer and one worker loop
// (Enqueue/Dequeue/Close over one channel); Partitioner answers a
// different question -- given N independent consumers (or shards)
// pulling from possibly-separate queues/partitions, which one should
// a given key's work land on so that repeated work for the same key
// (e.g. all jobs for one case ID) is handled consistently by the same
// worker. A caller wiring cross-service or cross-worker distribution
// on top of JobQueue would use Partitioner to decide which
// JobQueue instance (of N) to Enqueue onto; Partitioner does not
// itself enqueue, dequeue, or buffer anything. See doc/scalability.md
// for the full composition write-up.
type Partitioner interface {
	// Partition maps key to a partition index in [0, N), where N is
	// the partition count the Partitioner was constructed with.
	Partition(key string) int

	// PartitionCount returns N, the number of logical partitions this
	// Partitioner maps keys across.
	PartitionCount() int
}

// consistentHashPartitioner is a Partitioner implementation using a
// consistent-hashing ring: each of the N partitions is assigned
// several virtual points around a hash ring, and a key is mapped to
// the partition owning the first ring point at or after the key's own
// hash. This bounds key movement when N changes (only keys whose ring
// position falls between the old and new insertion point move),
// unlike plain modulo hashing where changing N remaps nearly every
// key. See ShardStrategy (shard.go) for the store-partitioning use of
// the same technique with an explicit Rebalance plan calculator.
type consistentHashPartitioner struct {
	partitionCount int
	ring           []ringPoint
}

// ringPoint is one virtual node on the consistent-hash ring.
type ringPoint struct {
	hash      uint32
	partition int
}

// virtualNodesPerPartition controls how many ring points each
// partition is assigned. More virtual nodes smooth out distribution
// unevenness at the cost of a larger ring to search. Consistent
// hashing is an approximation, not a perfect partition: even with a
// few hundred virtual nodes, individual partitions can still see a
// meaningfully uneven share of ring space purely from virtual-node
// placement variance (a well-documented property of the algorithm,
// not a bug -- see partitioner_test.go's distribution-evenness test,
// which asserts on mean deviation across partitions rather than a
// single worst-case partition for exactly this reason). 150 balances
// a materially smoother distribution than the more common 100 against
// ring-construction/lookup cost.
const virtualNodesPerPartition = 150

// NewConsistentHashPartitioner constructs a Partitioner distributing
// keys across n logical partitions via consistent hashing. Returns
// ErrInvalidPartitionCount if n <= 0.
func NewConsistentHashPartitioner(n int) (Partitioner, error) {
	if n <= 0 {
		return nil, wrapf("NewConsistentHashPartitioner", ErrInvalidPartitionCount)
	}

	ring := make([]ringPoint, 0, n*virtualNodesPerPartition)
	for p := 0; p < n; p++ {
		for v := 0; v < virtualNodesPerPartition; v++ {
			ring = append(ring, ringPoint{
				hash:      hashString(virtualNodeKey(p, v)),
				partition: p,
			})
		}
	}
	sort.Slice(ring, func(i, j int) bool { return ring[i].hash < ring[j].hash })

	return &consistentHashPartitioner{partitionCount: n, ring: ring}, nil
}

// Partition implements Partitioner.
func (p *consistentHashPartitioner) Partition(key string) int {
	if len(p.ring) == 0 {
		return 0
	}
	h := hashString(key)
	// Binary search for the first ring point with hash >= h; wrap
	// around to the first point if h is greater than every ring hash.
	idx := sort.Search(len(p.ring), func(i int) bool { return p.ring[i].hash >= h })
	if idx == len(p.ring) {
		idx = 0
	}
	return p.ring[idx].partition
}

// PartitionCount implements Partitioner.
func (p *consistentHashPartitioner) PartitionCount() int {
	return p.partitionCount
}

// moduloPartitioner is a Partitioner implementation using plain
// modulo hashing: simpler and perfectly even in expectation, but
// remaps nearly every key when N changes. Offered alongside the
// consistent-hash implementation for callers that never resize their
// partition count and want the simplest possible mapping (e.g. a
// fixed-size worker pool that is never rebalanced).
type moduloPartitioner struct {
	partitionCount int
}

// NewModuloPartitioner constructs a Partitioner distributing keys
// across n logical partitions via key-hash-modulo-n. Returns
// ErrInvalidPartitionCount if n <= 0.
func NewModuloPartitioner(n int) (Partitioner, error) {
	if n <= 0 {
		return nil, wrapf("NewModuloPartitioner", ErrInvalidPartitionCount)
	}
	return &moduloPartitioner{partitionCount: n}, nil
}

// Partition implements Partitioner.
func (p *moduloPartitioner) Partition(key string) int {
	return int(hashString(key)) % p.partitionCount
}

// PartitionCount implements Partitioner.
func (p *moduloPartitioner) PartitionCount() int {
	return p.partitionCount
}

// hashString returns a stable 32-bit FNV-1a hash of s. FNV-1a is used
// throughout for both Partitioner implementations and ShardStrategy
// (shard.go) so that hashing behavior is uniform and dependency-free
// (hash/fnv is stdlib).
func hashString(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s)) // hash.Hash.Write never returns an error
	return h.Sum32()
}

// virtualNodeKey builds the ring-point key for partition p's v-th
// virtual node.
func virtualNodeKey(p, v int) string {
	return strconv.Itoa(p) + "#" + strconv.Itoa(v)
}
