package bulkimport

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BatchResult is the outcome of a single Engine.RunBatch call: how
// many records were read, and how each one was disposed of.
type BatchResult struct {
	// JobID identifies the job this batch ran against.
	JobID uuid.UUID

	// RecordsRead is how many SourceRecord values RunBatch actually
	// read from the RecordSource this call (may be less than the
	// requested batchSize if the source is near exhaustion).
	RecordsRead int

	// Imported, Skipped, and Rejected count this batch's outcomes.
	Imported int
	Skipped  int
	Rejected int

	// Done reports whether the RecordSource is now exhausted -- the
	// job has moved to StatusCompleted.
	Done bool

	// Job is the ImportJob's state after this batch, including its
	// updated Cursor and aggregate counts.
	Job ImportJob
}

// Engine is the bulk-import orchestrator: it composes the ImportJob
// state machine, ImportRecord processing (validate -> dedup ->
// import), a pluggable RecordSource, and rollback into one set of
// tenant- and permission-scoped operations, recording every job
// registration, batch run, and rollback via AuditSink. Engine mirrors
// packages/compliance.Engine's and packages/privacy.Engine's shape
// closely: authenticate, check tenant match, check permission, mutate,
// audit regardless of outcome.
type Engine struct {
	jobs    JobRepository
	records RecordRepository
	audit   *AuditSink
	clock   func() time.Time

	// validator is the Validator every RunBatch call runs each record
	// through (task 3). Defaults to DefaultValidator{} if never set.
	validator Validator
}

// NewEngine builds an Engine from its dependencies. jobs and records
// must be non-nil (ErrNilStore); audit may be nil (a nil audit sink
// means job/batch/rollback operations simply skip audit recording --
// useful for lightweight unit tests of the decision logic itself,
// though production callers should always supply one).
func NewEngine(jobs JobRepository, records RecordRepository, audit *AuditSink) (*Engine, error) {
	if jobs == nil || records == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		jobs:      jobs,
		records:   records,
		audit:     audit,
		clock:     time.Now,
		validator: DefaultValidator{},
	}, nil
}

// SetValidator replaces the Validator RunBatch uses for every
// subsequent call. Passing nil resets to DefaultValidator{}.
func (e *Engine) SetValidator(v Validator) {
	if v == nil {
		v = DefaultValidator{}
	}
	e.validator = v
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// RegisterJob creates a new ImportJob in StatusPending for tenantID
// (the entry point for tasks 1/2), requiring managePermission and
// tenant match. Every registration is recorded via AuditSink
// regardless of outcome.
func (e *Engine) RegisterJob(ctx context.Context, tenantID uuid.UUID, sourceDescription string, totalRecords int) (ImportJob, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordJobRegister(ctx, tenantID, actorFromCtx(ctx), ImportJob{SourceDescription: sourceDescription}, err)
		}
		return ImportJob{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordJobRegister(ctx, tenantID, user.ID, ImportJob{SourceDescription: sourceDescription}, err)
		}
		return ImportJob{}, err
	}

	job := NewImportJob(tenantID, sourceDescription, user.ID, totalRecords, e.now())
	if err := job.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordJobRegister(ctx, tenantID, user.ID, job, err)
		}
		return ImportJob{}, err
	}
	if err := e.jobs.Create(ctx, tenantID, &job); err != nil {
		wrapped := wrapf("RegisterJob", err)
		if e.audit != nil {
			_, _ = e.audit.RecordJobRegister(ctx, tenantID, user.ID, job, wrapped)
		}
		return ImportJob{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordJobRegister(ctx, tenantID, user.ID, job, nil)
	}
	return job, nil
}

// GetJob returns the ImportJob identified by jobID, requiring
// viewPermission and tenant match.
func (e *Engine) GetJob(ctx context.Context, tenantID, jobID uuid.UUID) (ImportJob, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return ImportJob{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ImportJob{}, err
	}
	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return ImportJob{}, wrapf("GetJob", err)
	}
	return *job, nil
}

// ListJobs returns every ImportJob registered for tenantID, requiring
// viewPermission and tenant match.
func (e *Engine) ListJobs(ctx context.Context, tenantID uuid.UUID) ([]ImportJob, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.jobs.List(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListJobs", err)
	}
	return list, nil
}

// ListRecords returns every ImportRecord belonging to jobID, requiring
// viewPermission and tenant match.
func (e *Engine) ListRecords(ctx context.Context, tenantID, jobID uuid.UUID) ([]ImportRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.records.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, wrapf("ListRecords", err)
	}
	return list, nil
}

// GetProgress returns a Progress snapshot of jobID's current state
// (task 6), requiring viewPermission and tenant match.
func (e *Engine) GetProgress(ctx context.Context, tenantID, jobID uuid.UUID) (Progress, error) {
	job, err := e.GetJob(ctx, tenantID, jobID)
	if err != nil {
		return Progress{}, err
	}
	return progressFromJob(job, e.now()), nil
}

// RunBatch processes up to batchSize records from source, starting at
// job's current Cursor (task 2's real chunking logic, and task 4's
// resumability: a crash between batches leaves Cursor at the last
// successfully processed record, so the next RunBatch call picks up
// exactly where it left off). Requires managePermission and tenant
// match. Returns ErrJobNotRunnable if job.Status is not Pending or
// Running.
//
// Each record read is: validated via the configured Validator (task
// 3); if invalid, recorded as OutcomeRejected with its
// ValidationErrors attached and counted toward FailedRecords. If
// valid, its DedupKey is computed and looked up against already-
// imported records in this job/tenant (task 5); a match is recorded as
// OutcomeSkippedDuplicate and counted toward SkippedRecords. Otherwise
// the record is recorded as OutcomeImported and counted toward
// ImportedRecords.
//
// The job's Cursor, per-outcome counters, and Status are persisted via
// JobRepository.Update once per RunBatch call (after the whole batch),
// and every batch run is recorded via AuditSink regardless of outcome.
func (e *Engine) RunBatch(ctx context.Context, tenantID, jobID uuid.UUID, source RecordSource, batchSize int) (BatchResult, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return BatchResult{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return BatchResult{}, err
	}
	if source == nil {
		return BatchResult{}, wrapf("RunBatch", ErrNilSource)
	}
	if batchSize <= 0 {
		return BatchResult{}, wrapf("RunBatch", ErrInvalidBatchSize)
	}

	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return BatchResult{}, wrapf("RunBatch", err)
	}
	if job.Status != StatusPending && job.Status != StatusRunning {
		err := fmt.Errorf("%w: job %s is %s", ErrJobNotRunnable, jobID, job.Status)
		if e.audit != nil {
			_, _ = e.audit.RecordBatchRun(ctx, tenantID, user.ID, *job, BatchResult{JobID: jobID, Job: *job}, err)
		}
		return BatchResult{}, err
	}

	now := e.now()
	if job.Status == StatusPending {
		if err := transitionJob(job, StatusRunning, now); err != nil {
			return BatchResult{}, wrapf("RunBatch", err)
		}
	}

	sourceRecords, done, err := source.ReadAt(ctx, job.Cursor, batchSize)
	if err != nil {
		return BatchResult{}, wrapf("RunBatch", err)
	}

	result := BatchResult{JobID: jobID, RecordsRead: len(sourceRecords), Done: done}

	for i, sr := range sourceRecords {
		rec := ImportRecord{
			ID:               uuid.New(),
			TenantID:         tenantID,
			JobID:            jobID,
			SourceIndex:      job.Cursor + i,
			PayloadRef:       sr.PayloadRef,
			CaseNumber:       sr.CaseNumber,
			Jurisdiction:     sr.Jurisdiction,
			PartyNames:       sr.PartyNames,
			ValidationStatus: ValidationPending,
			Outcome:          OutcomePending,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		validationErrs := e.validator.Validate(rec)
		if len(validationErrs) > 0 {
			rec.ValidationStatus = ValidationFailed
			rec.ValidationErrors = validationErrs
			rec.Outcome = OutcomeRejected
			rec.OutcomeReason = "failed validation"
			job.FailedRecords++
			result.Rejected++
		} else {
			rec.ValidationStatus = ValidationPassed
			rec.DedupKey = ComputeDedupKey(rec.CaseNumber, rec.Jurisdiction, rec.PartyNames)

			var dup *ImportRecord
			if rec.DedupKey != "" {
				dup, _ = e.records.FindByDedupKey(ctx, tenantID, jobID, rec.DedupKey)
			}
			if dup != nil {
				rec.Outcome = OutcomeSkippedDuplicate
				rec.OutcomeReason = fmt.Sprintf("duplicate of record %s", dup.ID)
				job.SkippedRecords++
				result.Skipped++
			} else {
				rec.Outcome = OutcomeImported
				job.ImportedRecords++
				result.Imported++
			}
		}

		if err := e.records.Create(ctx, tenantID, &rec); err != nil {
			return BatchResult{}, wrapf("RunBatch", err)
		}
		job.ProcessedRecords++
	}

	job.Cursor += len(sourceRecords)
	if done {
		if err := transitionJob(job, StatusCompleted, e.now()); err != nil {
			return BatchResult{}, wrapf("RunBatch", err)
		}
	} else {
		job.UpdatedAt = e.now()
	}

	if err := e.jobs.Update(ctx, tenantID, job); err != nil {
		wrapped := wrapf("RunBatch", err)
		if e.audit != nil {
			_, _ = e.audit.RecordBatchRun(ctx, tenantID, user.ID, *job, result, wrapped)
		}
		return BatchResult{}, wrapped
	}

	result.Job = *job
	if e.audit != nil {
		_, _ = e.audit.RecordBatchRun(ctx, tenantID, user.ID, *job, result, nil)
	}
	return result, nil
}
