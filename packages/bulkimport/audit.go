package bulkimport

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/privacy/audit.go and packages/compliance's equivalent.
const (
	auditActionJobRegister = "bulkimport.job_register"
	auditActionBatchRun    = "bulkimport.batch_run"
	auditActionRollback    = "bulkimport.rollback"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/privacy's and packages/compliance's
// systemActor idiom.
const systemActor = "system:bulkimport"

// AuditSink records every ImportJob registration, RunBatch call, and
// Rollback via packages/auditlog.Store (task 8), reusing the existing
// hash-chained, queryable audit trail rather than a second table. It
// mirrors packages/privacy.AuditSink and packages/compliance.AuditSink
// exactly.
type AuditSink struct {
	store *auditlog.Store
}

// NewAuditSink builds an AuditSink backed by store. Returns
// ErrNilAuditSink if store is nil.
func NewAuditSink(store *auditlog.Store) (*AuditSink, error) {
	if store == nil {
		return nil, ErrNilAuditSink
	}
	return &AuditSink{store: store}, nil
}

// actorFor returns actorUserID.String() if non-nil, else systemActor.
func actorFor(actorUserID uuid.UUID) string {
	if actorUserID == uuid.Nil {
		return systemActor
	}
	return actorUserID.String()
}

// RecordJobRegister appends an Event describing an ImportJob being
// registered.
func (s *AuditSink) RecordJobRegister(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, job ImportJob, registerErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "registered"
	detail := fmt.Sprintf("source=%q total_records=%d", job.SourceDescription, job.TotalRecords)
	if registerErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, registerErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionJobRegister
	ev.Target = job.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordJobRegister", err)
	}
	return appended, nil
}

// RecordBatchRun appends an Event describing a single RunBatch call --
// success or failure -- including the batch's per-outcome counts.
func (s *AuditSink) RecordBatchRun(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, job ImportJob, result BatchResult, runErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "ran"
	detail := fmt.Sprintf("read=%d imported=%d skipped=%d rejected=%d cursor=%d status=%s",
		result.RecordsRead, result.Imported, result.Skipped, result.Rejected, job.Cursor, job.Status)
	if runErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, runErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionBatchRun
	ev.Target = job.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordBatchRun", err)
	}
	return appended, nil
}

// RecordRollback appends an Event describing an Engine.Rollback call,
// including how many previously imported records were reversed.
func (s *AuditSink) RecordRollback(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, job ImportJob, reversedCount int, rollbackErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "rolled_back"
	detail := fmt.Sprintf("reversed_records=%d", reversedCount)
	if rollbackErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, rollbackErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionRollback
	ev.Target = job.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordRollback", err)
	}
	return appended, nil
}

// ImportActivity surfaces every bulk-import-related audit event for
// tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check.
func (s *AuditSink) ImportActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("ImportActivity", err)
	}
	return events, nil
}
