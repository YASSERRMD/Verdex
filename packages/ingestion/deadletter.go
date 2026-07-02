package ingestion

import (
	"sync"
	"time"
)

// DeadLetter records a job that exhausted retries at some stage and was
// pulled out of the pipeline for later inspection/manual intervention.
type DeadLetter struct {
	// JobID identifies the job that failed.
	JobID string

	// CaseID identifies the case the job belonged to.
	CaseID string

	// Stage is the stage the job was attempting when retries were
	// exhausted.
	Stage Stage

	// Reason is the wrapped failure error's message (typically wrapping
	// ErrRetriesExhausted or ErrDiscardVerificationFailed).
	Reason string

	// Attempts is the number of attempts made at Stage before the job was
	// dead-lettered.
	Attempts int

	// FailedAt is the wall-clock time the job was dead-lettered.
	FailedAt time.Time
}

// DeadLetterQueue holds jobs that exhausted retries, keyed by JobID, so
// they can be listed and inspected (and, in principle, resubmitted) without
// blocking the live pipeline.
//
// Implementations must be safe for concurrent use.
type DeadLetterQueue interface {
	// Add records dl. A second Add for the same JobID overwrites the
	// previous entry (the most recent failure is what matters).
	Add(dl DeadLetter)

	// Get returns the DeadLetter recorded for jobID, and ok=false if the
	// job has never been dead-lettered.
	Get(jobID string) (dl DeadLetter, ok bool)

	// List returns every recorded DeadLetter, in no particular order.
	List() []DeadLetter

	// Remove deletes any DeadLetter recorded for jobID (used once a job has
	// been manually resubmitted and no longer needs to appear dead-lettered).
	Remove(jobID string)
}

// InMemoryDeadLetterQueue is a map-backed DeadLetterQueue.
type InMemoryDeadLetterQueue struct {
	mu      sync.Mutex
	entries map[string]DeadLetter
}

// NewInMemoryDeadLetterQueue constructs an empty InMemoryDeadLetterQueue.
func NewInMemoryDeadLetterQueue() *InMemoryDeadLetterQueue {
	return &InMemoryDeadLetterQueue{entries: make(map[string]DeadLetter)}
}

// Add implements DeadLetterQueue.
func (q *InMemoryDeadLetterQueue) Add(dl DeadLetter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries[dl.JobID] = dl
}

// Get implements DeadLetterQueue.
func (q *InMemoryDeadLetterQueue) Get(jobID string) (DeadLetter, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	dl, ok := q.entries[jobID]
	return dl, ok
}

// List implements DeadLetterQueue.
func (q *InMemoryDeadLetterQueue) List() []DeadLetter {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]DeadLetter, 0, len(q.entries))
	for _, dl := range q.entries {
		out = append(out, dl)
	}
	return out
}

// Remove implements DeadLetterQueue.
func (q *InMemoryDeadLetterQueue) Remove(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.entries, jobID)
}
