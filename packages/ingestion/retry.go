package ingestion

import (
	"context"
	"fmt"
	"sync"
)

// DefaultMaxAttempts is the default number of attempts (including the
// first) RunWithRetry allows for a single stage before giving up.
const DefaultMaxAttempts = 3

// IdempotencyRecord captures the outcome of a previously executed
// (idempotencyKey, stage) pair.
type IdempotencyRecord struct {
	// Completed reports whether the stage ran to completion successfully.
	Completed bool

	// Attempts is the number of attempts made so far for this key+stage.
	Attempts int
}

// IdempotencyStore records which (key, stage) pairs have already completed,
// so re-running a completed stage is a no-op, and tracks attempt counts for
// stages still in progress.
//
// Implementations must be safe for concurrent use.
type IdempotencyStore interface {
	// Get returns the recorded outcome for key+stage, and ok=false if no
	// record exists yet.
	Get(key string, stage Stage) (rec IdempotencyRecord, ok bool)

	// MarkAttempt records that another attempt was made for key+stage,
	// returning the updated attempt count.
	MarkAttempt(key string, stage Stage) int

	// MarkCompleted records that key+stage finished successfully.
	MarkCompleted(key string, stage Stage)

	// Reset clears any recorded state for key+stage, allowing it to be
	// retried from attempt zero (used when a job is resumed into a stage it
	// previously failed).
	Reset(key string, stage Stage)
}

// InMemoryIdempotencyStore is a map-backed IdempotencyStore.
type InMemoryIdempotencyStore struct {
	mu      sync.Mutex
	records map[idempotencyKey]IdempotencyRecord
}

type idempotencyKey struct {
	key   string
	stage Stage
}

// NewInMemoryIdempotencyStore constructs an empty InMemoryIdempotencyStore.
func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{records: make(map[idempotencyKey]IdempotencyRecord)}
}

// Get implements IdempotencyStore.
func (s *InMemoryIdempotencyStore) Get(key string, stage Stage) (IdempotencyRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[idempotencyKey{key, stage}]
	return rec, ok
}

// MarkAttempt implements IdempotencyStore.
func (s *InMemoryIdempotencyStore) MarkAttempt(key string, stage Stage) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := idempotencyKey{key, stage}
	rec := s.records[k]
	rec.Attempts++
	s.records[k] = rec
	return rec.Attempts
}

// MarkCompleted implements IdempotencyStore.
func (s *InMemoryIdempotencyStore) MarkCompleted(key string, stage Stage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := idempotencyKey{key, stage}
	rec := s.records[k]
	rec.Completed = true
	s.records[k] = rec
}

// Reset implements IdempotencyStore.
func (s *InMemoryIdempotencyStore) Reset(key string, stage Stage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, idempotencyKey{key, stage})
}

// StageFunc performs the work of a single pipeline stage for a job,
// returning an updated WorkflowState (still at the same Stage; the caller
// advances Stage on success) or an error.
type StageFunc func(ctx context.Context, job Job, state WorkflowState) (WorkflowState, error)

// RetryPolicy bounds how many times RunWithRetry will attempt a StageFunc
// before giving up.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	// Values <= 0 fall back to DefaultMaxAttempts.
	MaxAttempts int
}

func (p RetryPolicy) maxAttempts() int {
	if p.MaxAttempts <= 0 {
		return DefaultMaxAttempts
	}
	return p.MaxAttempts
}

// RunWithRetry executes fn for job at stage, honoring idempotency and
// bounded retries:
//
//   - If store already has a Completed record for (job.JobID, stage), fn is
//     not invoked at all and RunWithRetry returns state unchanged with a
//     nil error (the no-op idempotency path).
//   - Otherwise fn is invoked up to policy.maxAttempts() times. Each
//     attempt increments the recorded attempt count. The first successful
//     attempt marks the record Completed and returns immediately.
//   - If every attempt fails, RunWithRetry returns the last error wrapped
//     in ErrRetriesExhausted.
func RunWithRetry(ctx context.Context, store IdempotencyStore, policy RetryPolicy, job Job, stage Stage, state WorkflowState, fn StageFunc) (WorkflowState, error) {
	if store == nil {
		store = NewInMemoryIdempotencyStore()
	}

	if rec, ok := store.Get(job.JobID, stage); ok && rec.Completed {
		return state, nil
	}

	max := policy.maxAttempts()
	for {
		attempt := store.MarkAttempt(job.JobID, stage)

		next, err := fn(ctx, job, state)
		if err == nil {
			store.MarkCompleted(job.JobID, stage)
			return next, nil
		}

		if attempt >= max {
			return state, fmt.Errorf("%w: stage=%s job=%s after %d attempts: %v", ErrRetriesExhausted, stage, job.JobID, attempt, err)
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return state, fmt.Errorf("%w: stage=%s job=%s context: %v", ErrStageFailed, stage, job.JobID, ctxErr)
		}
	}
}
