package bulkimport

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// transitionJob validates and applies a Status transition on job,
// returning ErrInvalidTransition if the move is not permitted from
// job's current Status.
func transitionJob(job *ImportJob, next Status, now time.Time) error {
	if !job.Status.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, job.Status, next)
	}
	job.Status = next
	job.UpdatedAt = now
	switch next {
	case StatusRunning:
		if job.StartedAt == nil {
			job.StartedAt = &now
		}
	case StatusCompleted, StatusFailed:
		job.FinishedAt = &now
	default:
		// StatusPending, StatusPaused, StatusRolledBack need no
		// additional bookkeeping beyond the Status/UpdatedAt already
		// set above.
	}
	return nil
}

// Pause moves jobID from StatusRunning to StatusPaused, preserving its
// Cursor so a later RunBatch call resumes exactly where it left off
// (task 4). Requires managePermission and tenant match.
func (e *Engine) Pause(ctx context.Context, tenantID, jobID uuid.UUID) (ImportJob, error) {
	return e.applyTransition(ctx, tenantID, jobID, StatusPaused)
}

// ResumeJob moves jobID from StatusPaused (or StatusFailed, to retry)
// back to StatusRunning, so the next RunBatch call continues from
// Cursor. Requires managePermission and tenant match.
func (e *Engine) ResumeJob(ctx context.Context, tenantID, jobID uuid.UUID) (ImportJob, error) {
	return e.applyTransition(ctx, tenantID, jobID, StatusRunning)
}

// FailJob explicitly marks jobID as StatusFailed with reason, e.g.
// when a caller detects an unrecoverable condition RunBatch itself
// cannot see (a corrupt source file, an operator abort). Requires
// managePermission and tenant match.
func (e *Engine) FailJob(ctx context.Context, tenantID, jobID uuid.UUID, reason string) (ImportJob, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return ImportJob{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ImportJob{}, err
	}
	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return ImportJob{}, wrapf("FailJob", err)
	}
	if err := transitionJob(job, StatusFailed, e.now()); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordJobRegister(ctx, tenantID, user.ID, *job, err)
		}
		return ImportJob{}, wrapf("FailJob", err)
	}
	job.FailureReason = reason
	if err := e.jobs.Update(ctx, tenantID, job); err != nil {
		return ImportJob{}, wrapf("FailJob", err)
	}
	return *job, nil
}

func (e *Engine) applyTransition(ctx context.Context, tenantID, jobID uuid.UUID, next Status) (ImportJob, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return ImportJob{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ImportJob{}, err
	}
	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return ImportJob{}, wrapf("applyTransition", err)
	}
	if err := transitionJob(job, next, e.now()); err != nil {
		return ImportJob{}, wrapf("applyTransition", err)
	}
	if err := e.jobs.Update(ctx, tenantID, job); err != nil {
		return ImportJob{}, wrapf("applyTransition", err)
	}
	return *job, nil
}
