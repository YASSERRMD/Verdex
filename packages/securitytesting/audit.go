package securitytesting

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/compliance/audit.go and packages/threatmodel/audit.go.
const (
	auditActionSuiteRun          = "securitytesting.suite_run"
	auditActionFindingOpen       = "securitytesting.finding_open"
	auditActionFindingTransition = "securitytesting.finding_transition"
	auditActionRemediationVerify = "securitytesting.remediation_verify"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/compliance and packages/threatmodel's
// systemActor idiom.
const systemActor = "system:securitytesting"

// AuditSink records every suite run, finding open/transition, and
// remediation-verification into packages/auditlog.Store, reusing the
// existing hash-chained, queryable audit trail rather than a second
// table -- exactly the composition pattern packages/compliance's and
// packages/threatmodel's own AuditSink established. This package never
// creates a parallel audit mechanism: packages/auditlog remains the
// single durable audit sink for the entire platform.
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

// RecordSuiteRun appends an Event describing a Harness suite/category
// run: how many scenarios ran, how many failed, how many errored.
func (s *AuditSink) RecordSuiteRun(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, records []RunRecord, runErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	failed := len(FailedRecords(records))
	errored := len(ErroredRecords(records))
	outcome := "completed"
	detail := fmt.Sprintf("ran=%d failed=%d errored=%d", len(records), failed, errored)
	if runErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, runErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionSuiteRun
	ev.Target = tenantID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordSuiteRun", err)
	}
	return appended, nil
}

// RecordFindingOpen appends an Event describing a Finding being
// opened from a failed RunRecord.
func (s *AuditSink) RecordFindingOpen(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, f Finding, openErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "opened"
	detail := fmt.Sprintf("category=%s severity=%s scenario=%s", f.Category, f.Severity, f.SourceScenario)
	if openErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, openErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionFindingOpen
	ev.Target = f.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordFindingOpen", err)
	}
	return appended, nil
}

// RecordFindingTransition appends an Event describing a Finding's
// Status changing from `from` to `to`.
func (s *AuditSink) RecordFindingTransition(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, findingID uuid.UUID, from, to FindingStatus, transitionErr error) (auditlog.Event, error) {
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
	ev.Action = auditActionFindingTransition
	ev.Target = findingID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordFindingTransition", err)
	}
	return appended, nil
}

// RecordRemediationVerify appends an Event describing a remediation
// re-run: whether the re-run confirmed the fix (verified=true) or the
// vulnerability still reproduced (verified=false).
func (s *AuditSink) RecordRemediationVerify(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, findingID uuid.UUID, verified bool, verifyErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "still_reproduces"
	if verified {
		outcome = "verified_fixed"
	}
	detail := fmt.Sprintf("verified=%t", verified)
	if verifyErr != nil {
		detail = fmt.Sprintf("%s error=%s", detail, verifyErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionRemediationVerify
	ev.Target = findingID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordRemediationVerify", err)
	}
	return appended, nil
}

// SecurityTestingActivity surfaces every security-testing-related
// audit event for tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check.
func (s *AuditSink) SecurityTestingActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("SecurityTestingActivity", err)
	}
	return events, nil
}
