package dataresidency

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// Guard composes CheckTransfer with audit recording (task 7) and
// violation alerting (task 8) into a single call a caller (router,
// persistence, reportexport, ...) can invoke before any cross-region
// operation, so every check -- pass or fail -- is durably recorded and
// every failure raises an alert, without every call site having to
// remember to wire both by hand.
type Guard struct {
	audit *AuditSink
	alert AlertSink
	clock func() time.Time
}

// NewGuard builds a Guard backed by audit and alert. audit must not be
// nil (every check is required to be recorded — task 7 is
// non-optional). alert may be nil, in which case NoOpAlertSink is
// used.
func NewGuard(audit *AuditSink, alert AlertSink) (*Guard, error) {
	if audit == nil {
		return nil, ErrNilStore
	}
	if alert == nil {
		alert = NoOpAlertSink{}
	}
	return &Guard{audit: audit, alert: alert, clock: time.Now}, nil
}

func (g *Guard) now() time.Time {
	if g.clock != nil {
		return g.clock().UTC()
	}
	return time.Now().UTC()
}

// CheckTransfer runs CheckTransfer against policy, records the
// outcome via the audit sink, and -- on failure -- sends a
// ViolationEvent through the alert sink before returning the original
// guard error to the caller. The audit/alert side effects are
// best-effort: if recording the audit event itself fails, that error
// is joined with the original check error rather than silently
// swallowed, but a check that passed is never turned into a failure
// just because, e.g., alerting is unavailable (Send errors from a
// passing check are not possible since alerts only fire on
// violations).
func (g *Guard) CheckTransfer(ctx context.Context, tenantID, deploymentID uuid.UUID, sourceRegion, destRegion string, policy *ResidencyPolicy) error {
	checkErr := CheckTransfer(ctx, sourceRegion, destRegion, policy)

	if _, auditErr := g.audit.RecordTransferCheck(ctx, tenantID, deploymentID, sourceRegion, destRegion, checkErr); auditErr != nil {
		if checkErr != nil {
			return wrapf("Guard.CheckTransfer", errors.Join(checkErr, auditErr))
		}
		return wrapf("Guard.CheckTransfer", auditErr)
	}

	if checkErr != nil {
		_ = g.alert.Send(ctx, ViolationEvent{
			DeploymentID:  deploymentID,
			TenantID:      tenantID,
			ViolationType: ViolationTransferBlocked,
			SourceRegion:  sourceRegion,
			DestRegion:    destRegion,
			Reason:        checkErr.Error(),
			CreatedAt:     g.now(),
		})
		return checkErr
	}
	return nil
}

// CheckProviderLocality runs CheckProviderLocality against policy,
// records the outcome via the audit sink (reusing
// RecordTransferCheck's shape with cap.Region as both source and dest
// region, since a provider-locality check is a degenerate one-sided
// transfer: "would this call leave the deployment's allowed
// locality?"), and -- on failure -- sends a ViolationEvent through the
// alert sink, mirroring Guard.CheckTransfer's contract exactly so a
// caller selecting a provider gets the same audit+alert guarantee a
// cross-region data transfer does.
func (g *Guard) CheckProviderLocality(ctx context.Context, tenantID, deploymentID uuid.UUID, cap provider.Capability, policy *ResidencyPolicy) error {
	checkErr := CheckProviderLocality(ctx, cap, policy)

	if _, auditErr := g.audit.RecordTransferCheck(ctx, tenantID, deploymentID, cap.Region, cap.Region, checkErr); auditErr != nil {
		if checkErr != nil {
			return wrapf("Guard.CheckProviderLocality", errors.Join(checkErr, auditErr))
		}
		return wrapf("Guard.CheckProviderLocality", auditErr)
	}

	if checkErr != nil {
		_ = g.alert.Send(ctx, ViolationEvent{
			DeploymentID:  deploymentID,
			TenantID:      tenantID,
			ViolationType: ViolationTransferBlocked,
			SourceRegion:  cap.Region,
			DestRegion:    cap.Region,
			Reason:        checkErr.Error(),
			CreatedAt:     g.now(),
		})
		return checkErr
	}
	return nil
}
