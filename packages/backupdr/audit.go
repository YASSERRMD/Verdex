package backupdr

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
	auditActionPolicySet = "backupdr.policy_set"
	auditActionRecordAdd = "backupdr.record_add"
	auditActionDrillRun  = "backupdr.drill_run"
	auditActionTargetSet = "backupdr.target_set"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor (e.g. an automated backup scheduler), mirroring
// packages/compliance and packages/privacy's systemActor idiom.
const systemActor = "system:backupdr"

// AuditSink records every backup-policy change, backup record
// addition, restore-drill execution, and RPO/RTO target change via
// packages/auditlog.Store, reusing the existing hash-chained,
// queryable audit trail rather than a second table -- exactly the
// composition pattern packages/compliance's and packages/privacy's own
// AuditSink established.
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

// RecordPolicySet appends an Event describing a tenant's BackupPolicy
// for a DataClass being set or replaced.
func (s *AuditSink) RecordPolicySet(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, p BackupPolicy, setErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "set"
	detail := fmt.Sprintf("class=%s frequency=%s retention=%s encryption_required=%t cross_region_required=%t",
		p.Class, p.Frequency, p.RetentionWindow, p.EncryptionRequired, p.CrossRegionRequired)
	if setErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, setErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionPolicySet
	ev.Target = string(p.Class)
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordPolicySet", err)
	}
	return appended, nil
}

// RecordBackup appends an Event describing a BackupRecord being added
// for tenantID.
func (s *AuditSink) RecordBackup(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, rec BackupRecord, addErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := string(rec.Status)
	detail := fmt.Sprintf("class=%s location=%s size_bytes=%d encrypted=%t", rec.Class, rec.Location, rec.SizeBytes, rec.Encrypted)
	if addErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, addErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionRecordAdd
	ev.Target = rec.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordBackup", err)
	}
	return appended, nil
}

// RecordDrill appends an Event describing a RestoreDrill being
// executed for tenantID.
func (s *AuditSink) RecordDrill(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, d RestoreDrill, runErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := string(d.Outcome)
	detail := fmt.Sprintf("class=%s record_id=%s duration=%s", d.Class, d.RecordID, d.Duration)
	if runErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, runErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindSystem,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionDrillRun
	ev.Target = d.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordDrill", err)
	}
	return appended, nil
}

// RecordTargetSet appends an Event describing a tenant's RPO/RTO
// Target for a DataClass being set or replaced.
func (s *AuditSink) RecordTargetSet(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, t Target, setErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "set"
	detail := fmt.Sprintf("class=%s rpo=%s rto=%s", t.Class, t.RPO, t.RTO)
	if setErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, setErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionTargetSet
	ev.Target = string(t.Class)
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordTargetSet", err)
	}
	return appended, nil
}

// BackupDRActivity surfaces every backup/DR-related audit event for
// tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check.
func (s *AuditSink) BackupDRActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("BackupDRActivity", err)
	}
	return events, nil
}
