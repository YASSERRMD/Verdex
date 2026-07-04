package pilot

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// RecordFinding creates a new PilotFinding for tenantID, sourced from
// one or more FeedbackEntry records (task 6's ingestion side),
// requiring managePermission and tenant match. A fresh finding always
// starts at FindingStatusOpen regardless of what the caller supplies,
// mirroring packages/vulnmanagement.Engine.RecordFinding's identical
// "freshly recorded, not yet triaged" guarantee.
func (e *Engine) RecordFinding(ctx context.Context, tenantID uuid.UUID, f PilotFinding) (PilotFinding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return PilotFinding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return PilotFinding{}, err
	}

	f.TenantID = tenantID
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	f.Status = FindingStatusOpen
	f.TriageNotes = ""
	f.TriagedBy = nil
	f.TriagedAt = nil
	now := e.now()
	if f.DiscoveredAt.IsZero() {
		f.DiscoveredAt = now
	}
	f.CreatedAt = now
	f.UpdatedAt = now

	if err := f.Validate(); err != nil {
		return PilotFinding{}, err
	}
	if err := e.findings.Create(ctx, tenantID, &f); err != nil {
		return PilotFinding{}, wrapf("RecordFinding", err)
	}
	return f, nil
}

// GetFinding returns the PilotFinding identified by id for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) GetFinding(ctx context.Context, tenantID, id uuid.UUID) (PilotFinding, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return PilotFinding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return PilotFinding{}, err
	}
	f, err := e.findings.Get(ctx, tenantID, id)
	if err != nil {
		return PilotFinding{}, wrapf("GetFinding", err)
	}
	return *f, nil
}

// ListFindingsForDeployment returns every PilotFinding recorded under
// deploymentID for tenantID, requiring viewPermission and tenant
// match.
func (e *Engine) ListFindingsForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotFinding, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.findings.ListForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		return nil, wrapf("ListFindingsForDeployment", err)
	}
	return list, nil
}

// TriageFinding records a human decision about findingID -- a
// Priority, the new FindingStatus, and why (notes) -- and applies the
// resulting status transition (task 6's triage workflow). Mirrors
// packages/vulnmanagement.Engine.Triage's accountability pattern
// exactly: every decision requires non-blank notes and is recorded via
// AuditSink regardless of outcome. The transition from the finding's
// current Status to status must be legal per CanTransitionFinding, or
// TriageFinding fails with ErrIllegalStatusTransition before any state
// changes.
func (e *Engine) TriageFinding(ctx context.Context, tenantID, findingID uuid.UUID, priority Priority, status FindingStatus, notes string) (PilotFinding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, actorFromCtx(ctx), findingID, priority, status, err)
		}
		return PilotFinding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, err)
		}
		return PilotFinding{}, err
	}

	if strings.TrimSpace(notes) == "" {
		wrapped := wrapf("TriageFinding", ErrNotesRequired)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, wrapped)
		}
		return PilotFinding{}, wrapped
	}
	if !priority.IsValid() {
		wrapped := wrapf("TriageFinding", ErrInvalidFinding)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, wrapped)
		}
		return PilotFinding{}, wrapped
	}
	if !status.IsValid() {
		wrapped := wrapf("TriageFinding", ErrInvalidFinding)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, wrapped)
		}
		return PilotFinding{}, wrapped
	}

	finding, err := e.findings.Get(ctx, tenantID, findingID)
	if err != nil {
		wrapped := wrapf("TriageFinding", err)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, wrapped)
		}
		return PilotFinding{}, wrapped
	}

	if !CanTransitionFinding(finding.Status, status) {
		wrapped := wrapf("TriageFinding", ErrIllegalStatusTransition)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, wrapped)
		}
		return PilotFinding{}, wrapped
	}

	now := e.now()
	finding.Priority = priority
	finding.Status = status
	finding.TriageNotes = strings.TrimSpace(notes)
	finding.TriagedBy = &user.ID
	finding.TriagedAt = &now
	finding.UpdatedAt = now

	if err := e.findings.Update(ctx, tenantID, finding); err != nil {
		wrapped := wrapf("TriageFinding", err)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, wrapped)
		}
		return PilotFinding{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordFindingTriage(ctx, tenantID, user.ID, findingID, priority, status, nil)
	}
	return *finding, nil
}
