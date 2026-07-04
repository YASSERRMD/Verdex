package privacy

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/accessgovernance/audit.go and
// packages/keymanagement's audit.go.
const (
	auditActionSARTransition     = "privacy.sar_transition"
	auditActionErasureExecute    = "privacy.erasure_execute"
	auditActionConsentRecord     = "privacy.consent_record"
	auditActionConsentWithdraw   = "privacy.consent_withdraw"
	auditActionInventoryRegister = "privacy.inventory_register"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/accessgovernance's systemActor idiom.
const systemActor = "system:privacy"

// AuditSink records every subject-access-request transition, erasure
// execution, and consent change via packages/auditlog.Store (task 7),
// reusing the existing hash-chained, queryable audit trail rather than
// a second table. It mirrors packages/accessgovernance.AuditSink and
// packages/keymanagement's recordAudit helper exactly.
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

// RecordSARTransition appends an Event describing a
// SubjectAccessRequest status transition.
func (s *AuditSink) RecordSARTransition(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, sarID uuid.UUID, from, to SARStatus, transitionErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "transitioned"
	detail := fmt.Sprintf("sar=%s from=%s to=%s", sarID, from, to)
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
	ev.Action = auditActionSARTransition
	ev.Target = sarID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordSARTransition", err)
	}
	return appended, nil
}

// RecordErasureExecute appends an Event describing an ExecuteErasure
// call -- success or failure -- including whether the provenance
// chain-of-custody record was preserved, satisfying the "audited
// regardless of outcome" discipline this package inherits from
// packages/accessgovernance's Elevate/Evaluate recording.
func (s *AuditSink) RecordErasureExecute(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, req ErasureRequest, result ErasureResult, execErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "completed"
	detail := fmt.Sprintf("subject=%s category=%s source=%s action=%s provenance_preserved=%t",
		req.SubjectID, req.Category, req.SourceTag, result.ActionTaken, result.ProvenancePreserved)
	if req.HasProvenance() {
		detail = fmt.Sprintf("%s provenance_record=%s", detail, req.ProvenanceRecordID)
	}
	if execErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, execErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionErasureExecute
	ev.Target = req.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordErasureExecute", err)
	}
	return appended, nil
}

// RecordConsentChange appends an Event describing a consent grant or
// withdrawal.
func (s *AuditSink) RecordConsentChange(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, c ConsentRecord, withdrawn bool, changeErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	action := auditActionConsentRecord
	outcome := "granted"
	if withdrawn {
		action = auditActionConsentWithdraw
		outcome = "withdrawn"
	}
	detail := fmt.Sprintf("subject=%s purpose=%s basis=%s", c.SubjectID, c.Purpose, c.LegalBasis)
	if changeErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, changeErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = action
	ev.Target = c.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordConsentChange", err)
	}
	return appended, nil
}

// RecordInventoryRegister appends an Event describing a
// DataInventoryEntry being registered or updated.
func (s *AuditSink) RecordInventoryRegister(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, e DataInventoryEntry, registerErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "registered"
	detail := fmt.Sprintf("category=%s source=%s sensitivity=%s basis=%s", e.Category, e.SourceTag, e.Sensitivity, e.LegalBasis)
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
	ev.Action = auditActionInventoryRegister
	ev.Target = e.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordInventoryRegister", err)
	}
	return appended, nil
}

// PrivacyActivity surfaces every privacy-related audit event for
// tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check. Distinct from any
// single action's recorder, this is the read-side counterpart used by
// a privacy compliance dashboard or report.
func (s *AuditSink) PrivacyActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("PrivacyActivity", err)
	}
	return events, nil
}
