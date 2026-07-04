package vulnmanagement

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Triage records a human decision about findingID -- who decided, what
// they decided (decision, the new Status), and why (notes) -- and
// applies the resulting Status transition to the Finding itself (task
// 5's triage workflow). Mirrors packages/accessgovernance's Attest and
// packages/signoff's explicit-acknowledgement pattern by reference:
// every decision requires non-blank notes and records the actor,
// rather than allowing a bare status flip with no accountability
// trail. Requires managePermission and tenant match. The transition
// from the finding's current Status to decision must be legal per
// CanTransition, or Triage fails with ErrIllegalStatusTransition before
// any state changes -- an illegal attempt is still recorded via
// AuditSink as a denied TriageDecision.
func (e *Engine) Triage(ctx context.Context, tenantID, findingID uuid.UUID, decision Status, notes string, actor uuid.UUID) (Finding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, actorFromCtx(ctx), TriageDecision{FindingID: findingID, ToStatus: decision, Notes: notes}, err)
		}
		return Finding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, TriageDecision{FindingID: findingID, ToStatus: decision, Notes: notes}, err)
		}
		return Finding{}, err
	}

	// actor defaults to the authenticated caller when the zero value is
	// supplied, but an explicitly different actor (e.g. a decision
	// entered on behalf of another reviewer during an offline triage
	// meeting) is recorded verbatim -- Triage never silently
	// substitutes the caller's own identity over a caller-supplied one.
	if actor == uuid.Nil {
		actor = user.ID
	}

	if strings.TrimSpace(notes) == "" {
		wrapped := wrapf("Triage", ErrNotesRequired)
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, TriageDecision{FindingID: findingID, ToStatus: decision, Notes: notes, Actor: actor}, wrapped)
		}
		return Finding{}, wrapped
	}
	if !decision.IsValid() {
		wrapped := wrapf("Triage", ErrInvalidTriageDecision)
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, TriageDecision{FindingID: findingID, ToStatus: decision, Notes: notes, Actor: actor}, wrapped)
		}
		return Finding{}, wrapped
	}

	finding, err := e.findings.Get(ctx, tenantID, findingID)
	if err != nil {
		wrapped := wrapf("Triage", err)
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, TriageDecision{FindingID: findingID, ToStatus: decision, Notes: notes, Actor: actor}, wrapped)
		}
		return Finding{}, wrapped
	}

	fromStatus := finding.Status
	if !CanTransition(fromStatus, decision) {
		wrapped := wrapf("Triage", ErrIllegalStatusTransition)
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, TriageDecision{FindingID: findingID, FromStatus: fromStatus, ToStatus: decision, Notes: notes, Actor: actor}, wrapped)
		}
		return Finding{}, wrapped
	}

	now := e.now()
	record := TriageDecision{
		ID:         uuid.New(),
		TenantID:   tenantID,
		FindingID:  findingID,
		FromStatus: fromStatus,
		ToStatus:   decision,
		Notes:      strings.TrimSpace(notes),
		Actor:      actor,
		DecidedAt:  now,
	}
	if err := record.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, record, err)
		}
		return Finding{}, err
	}
	if err := e.triage.Create(ctx, tenantID, &record); err != nil {
		wrapped := wrapf("Triage", err)
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, record, wrapped)
		}
		return Finding{}, wrapped
	}

	finding.Status = decision
	finding.UpdatedAt = now
	if err := e.findings.Update(ctx, tenantID, finding); err != nil {
		wrapped := wrapf("Triage", err)
		if e.audit != nil {
			_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, record, wrapped)
		}
		return Finding{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordTriage(ctx, tenantID, user.ID, record, nil)
	}
	return *finding, nil
}

// TriageHistory returns every TriageDecision recorded for tenantID
// against findingID, ordered as stored, requiring viewPermission and
// tenant match.
func (e *Engine) TriageHistory(ctx context.Context, tenantID, findingID uuid.UUID) ([]TriageDecision, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.triage.ListForFinding(ctx, tenantID, findingID)
	if err != nil {
		return nil, wrapf("TriageHistory", err)
	}
	return list, nil
}
