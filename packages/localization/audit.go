package localization

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/privacy/audit.go and packages/compliance/audit.go.
const (
	auditActionPreferenceSet    = "localization.preference_set"
	auditActionPreferenceDelete = "localization.preference_delete"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/privacy's systemActor idiom.
const systemActor = "system:localization"

// AuditSink records every locale-preference change via
// packages/auditlog.Store (task 7/8's audit trail requirement,
// mirroring every other packages/* AuditSink in this codebase), reusing
// the existing hash-chained, queryable audit trail rather than a
// second table.
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

// RecordPreferenceSet appends an Event describing a Preference being
// set (created or updated).
func (s *AuditSink) RecordPreferenceSet(ctx context.Context, tenantID, actorUserID, targetUserID uuid.UUID, locale Locale, setErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "set"
	detail := fmt.Sprintf("user=%s locale=%s", targetUserID, locale)
	if setErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, setErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionPreferenceSet
	ev.Target = targetUserID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordPreferenceSet", err)
	}
	return appended, nil
}

// RecordPreferenceDelete appends an Event describing a Preference
// being removed (reverting the user to the platform default locale).
func (s *AuditSink) RecordPreferenceDelete(ctx context.Context, tenantID, actorUserID, targetUserID uuid.UUID, deleteErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "deleted"
	detail := fmt.Sprintf("user=%s", targetUserID)
	if deleteErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, deleteErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionPreferenceDelete
	ev.Target = targetUserID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordPreferenceDelete", err)
	}
	return appended, nil
}
