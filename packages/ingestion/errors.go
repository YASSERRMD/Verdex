package ingestion

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrJobNotFound is returned when a job ID does not resolve to a known
	// job in a JobQueue, IdempotencyStore, RecoveryStore, ProgressReporter,
	// or IngestionStatusAPI.
	ErrJobNotFound = errors.New("ingestion: job not found")

	// ErrStageFailed is returned (wrapped, with additional context) when a
	// pipeline stage function returns an error.
	ErrStageFailed = errors.New("ingestion: stage failed")

	// ErrRetriesExhausted is returned when a stage has failed
	// MaxAttempts times and the job has been moved to the dead-letter
	// queue.
	ErrRetriesExhausted = errors.New("ingestion: retries exhausted")

	// ErrInvalidJob is returned when a Job fails basic structural
	// validation (e.g. missing JobID, CaseID, or InputKind).
	ErrInvalidJob = errors.New("ingestion: invalid job")

	// ErrDiscardVerificationFailed is returned when the discard-guarantee
	// check after an extraction stage finds that the source binary was not
	// actually discarded by the underlying stt/ocr service.
	ErrDiscardVerificationFailed = errors.New("ingestion: discard verification failed")

	// ErrQueueClosed is returned by JobQueue.Enqueue/Dequeue once the queue
	// has been closed.
	ErrQueueClosed = errors.New("ingestion: queue closed")

	// ErrNotResumable is returned by Resume when a job has no persisted
	// recovery checkpoint to resume from.
	ErrNotResumable = errors.New("ingestion: job has no recovery checkpoint")
)
