package bulkimport

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Rollback reverses jobID's imported records (task 7): every
// ImportRecord with Outcome == OutcomeImported is moved to
// OutcomeRolledBack (and, if CreatedCaseID references a downstream
// case, that reference is cleared -- this package does not itself
// delete the referenced case, matching the reference-only,
// no-hard-dependency convention documented on
// ImportRecord.CreatedCaseID; a caller that models real case creation
// is responsible for reacting to the RolledBack outcome and
// retracting/flagging the case it created). The job itself moves to
// StatusRolledBack. Requires managePermission and tenant match, and
// returns ErrJobNotRollbackable unless job.Status is Completed or
// Failed. Every rollback is recorded via AuditSink regardless of
// outcome.
func (e *Engine) Rollback(ctx context.Context, tenantID, jobID uuid.UUID) (ImportJob, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return ImportJob{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ImportJob{}, err
	}

	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return ImportJob{}, wrapf("Rollback", err)
	}
	if job.Status != StatusCompleted && job.Status != StatusFailed {
		err := fmt.Errorf("%w: job %s is %s", ErrJobNotRollbackable, jobID, job.Status)
		if e.audit != nil {
			_, _ = e.audit.RecordRollback(ctx, tenantID, user.ID, *job, 0, err)
		}
		return ImportJob{}, err
	}

	records, err := e.records.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		wrapped := wrapf("Rollback", err)
		if e.audit != nil {
			_, _ = e.audit.RecordRollback(ctx, tenantID, user.ID, *job, 0, wrapped)
		}
		return ImportJob{}, wrapped
	}

	now := e.now()
	reversedCount := 0
	for i := range records {
		rec := records[i]
		if rec.Outcome != OutcomeImported {
			continue
		}
		rec.Outcome = OutcomeRolledBack
		rec.OutcomeReason = fmt.Sprintf("rolled back at %s", now.Format(time.RFC3339))
		rec.CreatedCaseID = uuid.Nil
		rec.UpdatedAt = now
		if err := e.records.Update(ctx, tenantID, &rec); err != nil {
			wrapped := wrapf("Rollback", err)
			if e.audit != nil {
				_, _ = e.audit.RecordRollback(ctx, tenantID, user.ID, *job, reversedCount, wrapped)
			}
			return ImportJob{}, wrapped
		}
		reversedCount++
	}

	if err := transitionJob(job, StatusRolledBack, now); err != nil {
		wrapped := wrapf("Rollback", err)
		if e.audit != nil {
			_, _ = e.audit.RecordRollback(ctx, tenantID, user.ID, *job, reversedCount, wrapped)
		}
		return ImportJob{}, wrapped
	}
	job.ImportedRecords -= reversedCount
	if err := e.jobs.Update(ctx, tenantID, job); err != nil {
		wrapped := wrapf("Rollback", err)
		if e.audit != nil {
			_, _ = e.audit.RecordRollback(ctx, tenantID, user.ID, *job, reversedCount, wrapped)
		}
		return ImportJob{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordRollback(ctx, tenantID, user.ID, *job, reversedCount, nil)
	}
	return *job, nil
}
