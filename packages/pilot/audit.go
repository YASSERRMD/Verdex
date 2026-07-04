package pilot

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/compliance/audit.go and packages/vulnmanagement/audit.go.
const (
	auditActionDeploymentStatusChange = "pilot.deployment_status_change"
	auditActionFindingTriage          = "pilot.finding_triage"
	auditActionReportCapture          = "pilot.report_capture"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/compliance and packages/vulnmanagement's
// systemActor idiom.
const systemActor = "system:pilot"

// AuditSink records every pilot-deployment status change, finding
// triage decision, and report capture via packages/auditlog.Store,
// reusing the existing hash-chained, queryable audit trail rather than
// a second table -- exactly the composition pattern
// packages/compliance's and packages/vulnmanagement's own AuditSink
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

// RecordDeploymentStatusChange appends an Event describing a
// PilotDeployment transitioning from one DeploymentStatus to another
// (or a rejected attempt to do so).
func (s *AuditSink) RecordDeploymentStatusChange(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, deploymentID uuid.UUID, from, to DeploymentStatus, changeErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "transitioned"
	detail := fmt.Sprintf("from=%s to=%s", from, to)
	if changeErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, changeErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionDeploymentStatusChange
	ev.Target = deploymentID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordDeploymentStatusChange", err)
	}
	return appended, nil
}

// RecordFindingTriage appends an Event describing a PilotFinding being
// triaged (or a rejected attempt to do so).
func (s *AuditSink) RecordFindingTriage(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, findingID uuid.UUID, priority Priority, status FindingStatus, triageErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "triaged"
	detail := fmt.Sprintf("priority=%s status=%s", priority, status)
	if triageErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, triageErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindDataChange,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionFindingTriage
	ev.Target = findingID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordFindingTriage", err)
	}
	return appended, nil
}

// RecordReportCapture appends an Event describing a pilot Report being
// captured for a deployment.
func (s *AuditSink) RecordReportCapture(ctx context.Context, tenantID uuid.UUID, actorUserID uuid.UUID, deploymentID uuid.UUID, captureErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "captured"
	detail := fmt.Sprintf("deployment=%s", deploymentID)
	if captureErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, captureErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionReportCapture
	ev.Target = deploymentID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordReportCapture", err)
	}
	return appended, nil
}

// PilotActivity surfaces every pilot-related audit event for tenantID
// matching filter, queried through packages/auditlog.Store's own
// PermAuditRead-gated Query -- this package never bypasses that
// authorization check.
func (s *AuditSink) PilotActivity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilAuditSink
	}
	events, err := s.store.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("PilotActivity", err)
	}
	return events, nil
}
