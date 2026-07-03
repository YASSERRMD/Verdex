package dataresidency

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// auditTransferAction and auditVerifyAction are the stable
// "<noun>.<verb>" verb-phrases recorded for every Event this package
// appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/auditlog's own adapters (e.g. "case.signoff").
const (
	auditTransferAction = "residency.transfer_check"
	auditVerifyAction   = "residency.verify"
)

// AuditSink records every CheckTransfer/CheckProviderLocality result
// and every Verify report via packages/auditlog.Store (task 7),
// reusing the centralized, hash-chained, queryable audit trail rather
// than a new logging channel. It mirrors
// packages/auditlog.SignoffAuditSink's shape: a thin wrapper around a
// *auditlog.Store with one Record* method per event category this
// package produces.
type AuditSink struct {
	store *auditlog.Store
}

// NewAuditSink builds an AuditSink backed by store. Returns
// ErrNilStore if store is nil.
func NewAuditSink(store *auditlog.Store) (*AuditSink, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	return &AuditSink{store: store}, nil
}

// RecordTransferCheck appends an Event describing the outcome of a
// CheckTransfer (or CheckProviderLocality) call: which tenant/
// deployment, source/dest region, and whether it passed. checkErr is
// the error returned by the guard (nil on success).
func (s *AuditSink) RecordTransferCheck(ctx context.Context, tenantID uuid.UUID, deploymentID uuid.UUID, sourceRegion, destRegion string, checkErr error) (auditlog.Event, error) {
	outcome := "allowed"
	detail := fmt.Sprintf("source=%s dest=%s", sourceRegion, destRegion)
	if checkErr != nil {
		outcome = "blocked"
		detail = fmt.Sprintf("%s reason=%s", detail, checkErr.Error())
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindSystem,
		Detail:   detail,
	}
	ev.Actor = "system:dataresidency"
	ev.Action = auditTransferAction
	ev.Target = deploymentID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordTransferCheck", err)
	}
	return appended, nil
}

// RecordVerification appends an Event summarizing a Verify report: pass/
// fail and the list of failed check kinds, if any.
func (s *AuditSink) RecordVerification(ctx context.Context, tenantID uuid.UUID, report Report) (auditlog.Event, error) {
	outcome := "pass"
	if !report.Passed() {
		outcome = "fail"
	}

	detail := fmt.Sprintf("checks=%d", len(report.Checks))
	if fails := report.Failures(); len(fails) > 0 {
		detail = fmt.Sprintf("%s failed=%v", detail, failureKinds(fails))
	}

	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindSystem,
		Detail:   detail,
	}
	ev.Actor = "system:dataresidency"
	ev.Action = auditVerifyAction
	ev.Target = report.DeploymentID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordVerification", err)
	}
	return appended, nil
}

func failureKinds(fails []CheckResult) []CheckKind {
	out := make([]CheckKind, 0, len(fails))
	for _, f := range fails {
		out = append(out, f.Kind)
	}
	return out
}
