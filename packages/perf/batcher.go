package perf

import (
	"sync"
	"time"
)

// FlushFunc is a caller-supplied function invoked with a completed batch of
// items. Batcher does not retry a FlushFunc that panics; callers wanting
// retry-on-failure semantics should implement that inside FlushFunc itself.
type FlushFunc[T any] func(items []T)

// BatcherConfig configures a Batcher.
type BatcherConfig struct {
	// MaxSize is the number of buffered items that triggers an immediate
	// flush. Must be > 0.
	MaxSize int

	// MaxWait is the maximum time an item may sit buffered before a flush
	// is triggered, even if MaxSize has not been reached. Must be > 0.
	MaxWait time.Duration
}

// Batcher collects items of type T added via Add, flushing them as a slice
// to a caller-supplied FlushFunc whenever either MaxSize items have
// accumulated or MaxWait has elapsed since the oldest buffered item was
// added, whichever comes first. Batcher is safe for concurrent use: many
// goroutines may call Add concurrently.
type Batcher[T any] struct {
	cfg   BatcherConfig
	flush FlushFunc[T]

	mu     sync.Mutex
	buf    []T
	timer  *time.Timer
	closed bool

	wg sync.WaitGroup
}

// NewBatcher constructs a Batcher with the given config, invoking flush for
// every completed batch. Panics if cfg.MaxSize <= 0 or cfg.MaxWait <= 0,
// mirroring how this repository's other constructors reject impossible
// configuration rather than silently defaulting it (see e.g.
// packages/reasoningeval.NewRegressionDetector's clamp, contrasted with a
// hard programmer error here: a Batcher with no size or time bound can
// never usefully flush).
func NewBatcher[T any](cfg BatcherConfig, flush FlushFunc[T]) *Batcher[T] {
	if cfg.MaxSize <= 0 {
		panic("perf: NewBatcher: MaxSize must be > 0")
	}
	if cfg.MaxWait <= 0 {
		panic("perf: NewBatcher: MaxWait must be > 0")
	}
	if flush == nil {
		panic("perf: NewBatcher: flush must not be nil")
	}
	return &Batcher[T]{cfg: cfg, flush: flush}
}

// Add appends item to the current batch, triggering an immediate flush if
// the batch has now reached MaxSize. Returns ErrBatcherClosed if Stop has
// already been called.
func (b *Batcher[T]) Add(item T) error {
	b.mu.Lock()

	if b.closed {
		b.mu.Unlock()
		return ErrBatcherClosed
	}

	b.buf = append(b.buf, item)
	if len(b.buf) == 1 {
		// First item in a fresh batch: arm the MaxWait timer.
		b.timer = time.AfterFunc(b.cfg.MaxWait, b.flushOnTimer)
	}

	sizeTriggered := len(b.buf) >= b.cfg.MaxSize
	var batch []T
	if sizeTriggered {
		batch = b.takeLocked()
	}
	b.mu.Unlock()

	if sizeTriggered {
		b.runFlush(batch)
	}
	return nil
}

// flushOnTimer is invoked by the MaxWait timer. It flushes whatever is
// currently buffered, if anything (a concurrent size-triggered flush may
// have already emptied the buffer and disarmed nothing, since time.Timer
// does not support reliable cancellation races -- this handles that by
// simply flushing an empty batch as a no-op).
func (b *Batcher[T]) flushOnTimer() {
	b.mu.Lock()
	batch := b.takeLocked()
	b.mu.Unlock()

	if len(batch) > 0 {
		b.runFlush(batch)
	}
}

// takeLocked returns the current buffer and resets it to empty, stopping
// the MaxWait timer if one is armed. Caller must hold b.mu.
func (b *Batcher[T]) takeLocked() []T {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	if len(b.buf) == 0 {
		return nil
	}
	batch := b.buf
	b.buf = nil
	return batch
}

// runFlush invokes b.flush(batch), tracked by b.wg so Stop can wait for any
// in-flight flush to complete before returning.
func (b *Batcher[T]) runFlush(batch []T) {
	b.wg.Add(1)
	defer b.wg.Done()
	b.flush(batch)
}

// Stop flushes any remaining buffered items and marks the Batcher closed:
// further Add calls return ErrBatcherClosed. Stop blocks until every
// in-flight flush (including the final one it triggers) has completed.
func (b *Batcher[T]) Stop() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	batch := b.takeLocked()
	b.mu.Unlock()

	if len(batch) > 0 {
		b.runFlush(batch)
	}
	b.wg.Wait()
}
