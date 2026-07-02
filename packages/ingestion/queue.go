package ingestion

import (
	"context"
	"sync"
)

// JobQueue is an asynchronous handoff point between whatever submits
// ingestion work (an HTTP handler, a CLI, a test) and the
// IngestionOrchestrator's worker loop. Implementations must be safe for
// concurrent use by multiple producers and consumers.
type JobQueue interface {
	// Enqueue submits job for processing. Returns ErrQueueClosed if the
	// queue has been closed.
	Enqueue(job Job) error

	// Dequeue blocks until a job is available, ctx is cancelled, or the
	// queue is closed and drained. Returns ErrQueueClosed once the queue is
	// closed and no buffered jobs remain.
	Dequeue(ctx context.Context) (*Job, error)

	// Close stops accepting new jobs. Jobs already buffered remain
	// available to Dequeue until drained. Close is idempotent.
	Close()
}

// InMemoryJobQueue is a channel-backed JobQueue with no external broker,
// suitable for a single-process deployment or tests.
type InMemoryJobQueue struct {
	ch chan Job

	closeOnce sync.Once
	closed    chan struct{}
}

// NewInMemoryJobQueue constructs an InMemoryJobQueue with the given buffer
// capacity. A capacity <= 0 yields an unbuffered (synchronous
// handoff) channel.
func NewInMemoryJobQueue(capacity int) *InMemoryJobQueue {
	if capacity < 0 {
		capacity = 0
	}
	return &InMemoryJobQueue{
		ch:     make(chan Job, capacity),
		closed: make(chan struct{}),
	}
}

// Enqueue implements JobQueue.
func (q *InMemoryJobQueue) Enqueue(job Job) error {
	select {
	case <-q.closed:
		return ErrQueueClosed
	default:
	}

	select {
	case q.ch <- job:
		return nil
	case <-q.closed:
		return ErrQueueClosed
	}
}

// Dequeue implements JobQueue. Once closed, buffered jobs are still
// delivered; only after the buffer is drained does Dequeue start returning
// ErrQueueClosed.
func (q *InMemoryJobQueue) Dequeue(ctx context.Context) (*Job, error) {
	select {
	case job := <-q.ch:
		return &job, nil
	default:
	}

	select {
	case job := <-q.ch:
		return &job, nil
	case <-q.closed:
		select {
		case job := <-q.ch:
			return &job, nil
		default:
			return nil, ErrQueueClosed
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close implements JobQueue. It is safe to call multiple times. Close does
// not close the underlying channel (avoiding a send-on-closed-channel
// panic if Enqueue races with Close); buffered jobs remain available to
// Dequeue.
func (q *InMemoryJobQueue) Close() {
	q.closeOnce.Do(func() {
		close(q.closed)
	})
}
