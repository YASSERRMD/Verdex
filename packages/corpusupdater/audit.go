package corpusupdater

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/compliance/audit.go and packages/privacy/audit.go.
const (
	auditActionJobCreate      = "corpusupdater.job_create"
	auditActionJobTransition  = "corpusupdater.job_transition"
	auditActionAmendmentStage = "corpusupdater.amendment_stage"
	auditActionAmendmentApply = "corpusupdater.amendment_apply"
	auditActionJobRollback    = "corpusupdater.job_rollback"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/compliance and packages/privacy's
// systemActor idiom.
const systemActor = "system:corpusupdater"

// AuditSink records every job creation/transition, amendment stage/
// apply, and rollback via packages/auditlog.Store, reusing the
// existing hash-chained, queryable audit trail rather than a second
// table (task 8) -- exactly the composition pattern
// packages/compliance's and packages/privacy's own AuditSink
// established.
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

// actorFromCtx resolves the actor's user ID from ctx if present,
// falling back to uuid.Nil (which actorFor renders as systemActor) when
// ctx carries no authenticated user -- used by the audit-on-failure
// paths, which must still record an event even when authorizeManage
// itself failed (e.g. ErrUnauthenticated).
func actorFromCtx(ctx context.Context) uuid.UUID {
	user, err := authorizeActor(ctx)
	if err != nil {
		return uuid.Nil
	}
	return user.ID
}

// RecordJobCreate appends an Event describing a CorpusUpdateJob being
// created.
func (s *AuditSink) RecordJobCreate(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, j CorpusUpdateJob, createErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "created"
	detail := fmt.Sprintf("jurisdiction=%s corpus=%s source=%s", j.JurisdictionCode, j.TargetCorpus, j.SourceDescription)
	if createErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, createErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionJobCreate
	ev.Target = j.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordJobCreate", err)
	}
	return appended, nil
}

// RecordJobTransition appends an Event describing a CorpusUpdateJob
// moving from one JobStatus to another (StageAmendment's owning job
// entering StatusValidating/StatusApplying, or resolving to
// StatusApplied/StatusFailed).
func (s *AuditSink) RecordJobTransition(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, jobID uuid.UUID, from, to JobStatus, transitionErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "transitioned"
	detail := fmt.Sprintf("from=%s to=%s", from, to)
	if transitionErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, transitionErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionJobTransition
	ev.Target = jobID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordJobTransition", err)
	}
	return appended, nil
}

// RecordAmendmentStage appends an Event describing an Amendment being
// staged onto a job.
func (s *AuditSink) RecordAmendmentStage(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, a Amendment, stageErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "staged"
	detail := fmt.Sprintf("job=%s corpus=%s target=%s change=%s citation=%s", a.JobID, a.TargetCorpus, a.TargetID, a.ChangeType, a.Citation)
	if stageErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, stageErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionAmendmentStage
	ev.Target = a.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordAmendmentStage", err)
	}
	return appended, nil
}

// RecordAmendmentApply appends an Event describing an Amendment being
// applied to its target corpus.
func (s *AuditSink) RecordAmendmentApply(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, a Amendment, applyErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "applied"
	detail := fmt.Sprintf("job=%s corpus=%s target=%s change=%s", a.JobID, a.TargetCorpus, a.TargetID, a.ChangeType)
	if applyErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, applyErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionAmendmentApply
	ev.Target = a.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordAmendmentApply", err)
	}
	return appended, nil
}

// RecordJobRollback appends an Event describing a job's applied
// Amendments being reverted.
func (s *AuditSink) RecordJobRollback(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, jobID uuid.UUID, amendmentCount int, rollbackErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "rolled_back"
	detail := fmt.Sprintf("amendments_reverted=%d", amendmentCount)
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
	ev.Action = auditActionJobRollback
	ev.Target = jobID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordJobRollback", err)
	}
	return appended, nil
}

// CorpusUpdateActivity surfaces every corpus-update-related audit
// event for tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check.
func (s *AuditSink) CorpusUpdateActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("CorpusUpdateActivity", err)
	}
	return events, nil
}
