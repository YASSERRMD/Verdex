package local

import "context"

// ConcurrencyLimiter is a semaphore that caps the number of simultaneous
// in-flight requests to the local model server. This is important because
// local models are CPU/GPU-bound and queueing too many requests at once
// degrades throughput for all callers rather than helping them.
//
// Wrap every Chat and Embed call with AcquireSlot / ReleaseSlot:
//
//	if err := limiter.AcquireSlot(ctx); err != nil {
//	    return nil, err
//	}
//	defer limiter.ReleaseSlot()
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// NewConcurrencyLimiter returns a limiter that allows at most maxSlots
// concurrent operations. If maxSlots is <= 0 it defaults to 1.
func NewConcurrencyLimiter(maxSlots int) *ConcurrencyLimiter {
	if maxSlots <= 0 {
		maxSlots = 1
	}
	sem := make(chan struct{}, maxSlots)
	// Pre-fill the channel so it acts as a counting semaphore.
	for i := 0; i < maxSlots; i++ {
		sem <- struct{}{}
	}
	return &ConcurrencyLimiter{sem: sem}
}

// AcquireSlot blocks until a concurrency slot is available or ctx is
// cancelled. It returns [ErrConcurrencyLimitExceeded] when ctx expires before
// a slot is freed.
func (l *ConcurrencyLimiter) AcquireSlot(ctx context.Context) error {
	select {
	case <-l.sem:
		return nil
	case <-ctx.Done():
		return ErrConcurrencyLimitExceeded
	}
}

// ReleaseSlot returns a concurrency slot to the pool. It must be called
// exactly once after each successful AcquireSlot call, typically via defer.
func (l *ConcurrencyLimiter) ReleaseSlot() {
	l.sem <- struct{}{}
}
