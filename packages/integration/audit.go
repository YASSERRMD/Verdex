package integration

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
	auditActionConnectorRegister = "integration.connector_register"
	auditActionCredentialsSet    = "integration.credentials_set"
	auditActionImportRun         = "integration.import_run"
	auditActionDeliveryRun       = "integration.delivery_run"
	auditActionReconciliation    = "integration.reconciliation_run"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/compliance and packages/privacy's
// systemActor idiom.
const systemActor = "system:integration"

// AuditSink records every connector registration, credentials change,
// import run, delivery run, and reconciliation attempt via
// packages/auditlog.Store (task 7), reusing the existing hash-chained,
// queryable audit trail rather than a second table -- exactly the
// composition pattern packages/compliance's and packages/privacy's own
// AuditSink established. Every import/delivery/reconciliation attempt
// is recorded regardless of outcome: callers pass the operation's error
// (nil on success) and AuditSink records "denied"/"failed" outcomes
// just as faithfully as successes.
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

// RecordConnectorRegister appends an Event describing a
// ConnectorConfig being registered or updated.
func (s *AuditSink) RecordConnectorRegister(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, cfg ConnectorConfig, registerErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "registered"
	detail := fmt.Sprintf("connector_type=%s display_name=%s enabled=%t", cfg.ConnectorType, cfg.DisplayName, cfg.Enabled)
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
	ev.Action = auditActionConnectorRegister
	ev.Target = cfg.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordConnectorRegister", err)
	}
	return appended, nil
}

// RecordCredentialsSet appends an Event describing a
// ConnectorCredentials record being set or updated for tenantID. The
// event Detail deliberately never includes SecretRef or any secret
// material -- only the credential Kind and ClientID, which are safe to
// log.
func (s *AuditSink) RecordCredentialsSet(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, creds ConnectorCredentials, setErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "set"
	detail := fmt.Sprintf("kind=%s client_id=%s", creds.Kind, creds.ClientID)
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
	ev.Action = auditActionCredentialsSet
	ev.Target = creds.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordCredentialsSet", err)
	}
	return appended, nil
}

// RecordImportRun appends an Event describing one ImportRun's
// completion (or failure).
func (s *AuditSink) RecordImportRun(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, run ImportRun, runErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := string(run.Status)
	detail := fmt.Sprintf("connector_config=%s imported=%d mapped=%d", run.ConnectorConfigID, run.ImportedCount, run.MappedCount)
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
	ev.Action = auditActionImportRun
	ev.Target = run.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordImportRun", err)
	}
	return appended, nil
}

// RecordDeliveryRun appends an Event describing one DeliveryRun's
// completion (or failure).
func (s *AuditSink) RecordDeliveryRun(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, run DeliveryRun, runErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := string(run.Status)
	detail := fmt.Sprintf("connector_config=%s case_external_id=%s report_kind=%s", run.ConnectorConfigID, run.CaseExternalID, run.ReportKind)
	if runErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, runErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindExport,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionDeliveryRun
	ev.Target = run.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordDeliveryRun", err)
	}
	return appended, nil
}

// RecordReconciliation appends an Event describing one
// ReconciliationResult.
func (s *AuditSink) RecordReconciliation(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, result ReconciliationResult, runErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "clean"
	if result.HasDrift() {
		outcome = "drift_detected"
	}
	detail := fmt.Sprintf("connector_config=%s kind=%s expected=%d observed=%d missing=%d unexpected=%d",
		result.ConnectorConfigID, result.Kind, result.ExpectedCount, result.ObservedCount,
		len(result.MissingExternalIDs), len(result.UnexpectedExternalIDs))
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
	ev.Action = auditActionReconciliation
	ev.Target = result.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordReconciliation", err)
	}
	return appended, nil
}

// IntegrationActivity surfaces every integration-related audit event
// for tenantID matching filter, queried through
// packages/auditlog.Store's own PermAuditRead-gated Query -- this
// package never bypasses that authorization check.
func (s *AuditSink) IntegrationActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("IntegrationActivity", err)
	}
	return events, nil
}
