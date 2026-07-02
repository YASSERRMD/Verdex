package adaptiveretrieval

import (
	"context"
	"time"
)

// DefaultMaxNodes is the MaxNodes a zero-valued BuildBudget falls back to.
const DefaultMaxNodes = 200

// DefaultMaxHops is the MaxHops a zero-valued BuildBudget falls back to.
const DefaultMaxHops = 4

// DefaultMaxWallClock is the MaxWallClock a zero-valued BuildBudget falls
// back to.
const DefaultMaxWallClock = 250 * time.Millisecond

// BuildBudget bounds how much on-demand subgraph-construction work a
// single adaptive build may perform: how many distinct nodes it may visit,
// how many hops deep it may walk, and how much wall-clock time it may
// spend. A Builder enforces all three independently — whichever limit is
// reached first ends the build — so a query that would otherwise fan out
// into an expensive walk degrades to a partial subgraph (or a
// treeindex fallback) rather than blocking indefinitely or scanning an
// entire case's tree.
//
// The zero value is not directly usable; use NewBuildBudget or
// DefaultBuildBudget to get a budget with sensible defaults, or construct
// a BuildBudget literal and call withDefaults-backed validation via
// Builder, which treats zero fields as "use the default" (mirroring
// treeindex.IndexerOptions and traversal.Query's own zero-means-default
// conventions).
type BuildBudget struct {
	// MaxNodes caps how many distinct nodes an adaptive build may visit
	// while walking outward from a query's anchor. Zero or negative falls
	// back to DefaultMaxNodes.
	MaxNodes int

	// MaxHops caps how many hops deep an adaptive build may walk from the
	// anchor. Zero or negative falls back to DefaultMaxHops.
	MaxHops int

	// MaxWallClock caps how long an adaptive build may run before it must
	// stop and return whatever partial subgraph it has built so far. Zero
	// or negative falls back to DefaultMaxWallClock.
	MaxWallClock time.Duration
}

// DefaultBuildBudget returns a BuildBudget using DefaultMaxNodes,
// DefaultMaxHops, and DefaultMaxWallClock.
func DefaultBuildBudget() BuildBudget {
	return BuildBudget{
		MaxNodes:     DefaultMaxNodes,
		MaxHops:      DefaultMaxHops,
		MaxWallClock: DefaultMaxWallClock,
	}
}

// withDefaults returns a copy of b with zero-or-negative fields replaced
// by their documented defaults.
func (b BuildBudget) withDefaults() BuildBudget {
	out := b
	if out.MaxNodes <= 0 {
		out.MaxNodes = DefaultMaxNodes
	}
	if out.MaxHops <= 0 {
		out.MaxHops = DefaultMaxHops
	}
	if out.MaxWallClock <= 0 {
		out.MaxWallClock = DefaultMaxWallClock
	}
	return out
}

// buildTracker turns a BuildBudget into a concrete deadline and running
// node counter for a single build, offering helpers to test whether the
// budget has been exceeded so far. Unlike hybridretrieval's budgetTracker
// (which only tracks wall-clock time), buildTracker also tracks a node
// visit count, since MaxNodes is enforced independently of MaxWallClock.
type buildTracker struct {
	budget    BuildBudget
	deadline  time.Time
	nodeCount int
}

// newBuildTracker starts a buildTracker for budget, measured from now.
func newBuildTracker(budget BuildBudget) *buildTracker {
	budget = budget.withDefaults()
	return &buildTracker{
		budget:   budget,
		deadline: time.Now().Add(budget.MaxWallClock),
	}
}

// exceeded reports whether the tracked wall-clock deadline has passed.
func (t *buildTracker) exceeded() bool {
	return time.Now().After(t.deadline)
}

// nodesExceeded reports whether visiting one more node would exceed
// MaxNodes.
func (t *buildTracker) nodesExceeded() bool {
	return t.nodeCount >= t.budget.MaxNodes
}

// recordNodes adds n to the running node count.
func (t *buildTracker) recordNodes(n int) {
	t.nodeCount += n
}

// withDeadline returns a derived context bounded by both parent's own
// deadline (if any) and t's tracked deadline, plus the resulting cancel
// function. Callers must call the returned cancel function, mirroring
// context.WithDeadline's contract.
func (t *buildTracker) withDeadline(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, t.deadline)
}
