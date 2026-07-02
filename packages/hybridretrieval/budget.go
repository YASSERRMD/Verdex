package hybridretrieval

import (
	"context"
	"time"
)

// budgetTracker turns a HybridQuery's requested Budget (a total time
// allowance for the query's graph-expansion phase) into a concrete
// deadline, and offers helpers to short-circuit or narrow expansion under
// time pressure. A zero Budget means "no tracking" (every helper reports
// no pressure): this is the query's opt-in latency-budget control, not a
// mandatory timeout.
type budgetTracker struct {
	deadline time.Time
	enabled  bool
}

// newBudgetTracker starts a budgetTracker for d, measured from now. A
// non-positive d disables tracking entirely.
func newBudgetTracker(d time.Duration) budgetTracker {
	if d <= 0 {
		return budgetTracker{}
	}
	return budgetTracker{deadline: time.Now().Add(d), enabled: true}
}

// exhausted reports whether the tracked budget has already run out.
// Always false when tracking is disabled.
func (b budgetTracker) exhausted() bool {
	return b.enabled && time.Now().After(b.deadline)
}

// remaining returns how much of the tracked budget is left. Only
// meaningful when enabled() is true.
func (b budgetTracker) remaining() time.Duration {
	if !b.enabled {
		return 0
	}
	return time.Until(b.deadline)
}

// withDeadline returns a derived context bounded by both parent's own
// deadline (if any) and b's tracked deadline (if enabled), plus the
// resulting cancel function. Mirrors context.WithTimeout's contract:
// callers must call the returned cancel function.
func (b budgetTracker) withDeadline(parent context.Context) (context.Context, context.CancelFunc) {
	if !b.enabled {
		return context.WithCancel(parent)
	}
	return context.WithDeadline(parent, b.deadline)
}
