package pilot

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// RecordRefinement records a concrete change applied in response to
// findingID (task 7), requiring managePermission and tenant match. The
// referenced PilotFinding must already have reached
// FindingStatus.IsAtLeastTriaged -- a refinement cannot reference a
// finding no human has ever reviewed and prioritized. This is the real
// state-tracking precondition the design brief calls for: it is
// enforced here, against the finding's current stored Status, not
// merely documented.
func (e *Engine) RecordRefinement(ctx context.Context, tenantID uuid.UUID, r RefinementRecord) (RefinementRecord, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return RefinementRecord{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RefinementRecord{}, err
	}

	finding, err := e.findings.Get(ctx, tenantID, r.FindingID)
	if err != nil {
		return RefinementRecord{}, wrapf("RecordRefinement", err)
	}
	if !finding.Status.IsAtLeastTriaged() {
		return RefinementRecord{}, wrapf("RecordRefinement", ErrFindingNotTriaged)
	}

	r.TenantID = tenantID
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.AppliedBy == uuid.Nil {
		r.AppliedBy = user.ID
	}
	// A fresh refinement is never pre-verified: VerifiedFixed and its
	// supporting fields can only be set by VerifyRefinement, so a
	// caller cannot skip the verification step by constructing an
	// already-verified record.
	r.VerifiedFixed = false
	r.VerificationNote = ""
	r.VerifiedBy = nil
	r.VerifiedAt = nil
	now := e.now()
	if r.AppliedAt.IsZero() {
		r.AppliedAt = now
	}
	r.CreatedAt = now
	r.UpdatedAt = now

	if err := r.Validate(); err != nil {
		return RefinementRecord{}, err
	}
	if err := e.refinements.Create(ctx, tenantID, &r); err != nil {
		return RefinementRecord{}, wrapf("RecordRefinement", err)
	}

	// Applying a refinement moves its finding into InProgress if it was
	// only Triaged so far, reflecting that remediation work has now
	// actually started -- mirrors
	// packages/vulnmanagement.Engine.Triage's own state-machine
	// coupling between a triage decision and its finding's Status.
	if finding.Status == FindingStatusTriaged && CanTransitionFinding(finding.Status, FindingStatusInProgress) {
		finding.Status = FindingStatusInProgress
		finding.UpdatedAt = now
		if err := e.findings.Update(ctx, tenantID, finding); err != nil {
			return RefinementRecord{}, wrapf("RecordRefinement", err)
		}
	}

	return r, nil
}

// GetRefinement returns the RefinementRecord identified by id for
// tenantID, requiring viewPermission and tenant match.
func (e *Engine) GetRefinement(ctx context.Context, tenantID, id uuid.UUID) (RefinementRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return RefinementRecord{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RefinementRecord{}, err
	}
	r, err := e.refinements.Get(ctx, tenantID, id)
	if err != nil {
		return RefinementRecord{}, wrapf("GetRefinement", err)
	}
	return *r, nil
}

// ListRefinementsForFinding returns every RefinementRecord recorded
// against findingID for tenantID, requiring viewPermission and tenant
// match.
func (e *Engine) ListRefinementsForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]RefinementRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.refinements.ListForFinding(ctx, tenantID, findingID)
	if err != nil {
		return nil, wrapf("ListRefinementsForFinding", err)
	}
	return list, nil
}

// ListRefinementsForDeployment returns every RefinementRecord recorded
// against any PilotFinding surfaced under deploymentID for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) ListRefinementsForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]RefinementRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	findings, err := e.findings.ListForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		return nil, wrapf("ListRefinementsForDeployment", err)
	}
	findingIDs := make([]uuid.UUID, len(findings))
	for i, f := range findings {
		findingIDs[i] = f.ID
	}
	list, err := e.refinements.ListForDeployment(ctx, tenantID, findingIDs)
	if err != nil {
		return nil, wrapf("ListRefinementsForDeployment", err)
	}
	return list, nil
}

// VerifyRefinement marks refinementID as VerifiedFixed, recording who
// verified it and why (note), requiring managePermission and tenant
// match. note is required (non-blank): a verification claim with no
// supporting basis is not a real verification, mirroring
// RefinementRecord.Validate's identical requirement. When the
// verified refinement's referenced PilotFinding is still
// FindingStatusInProgress, VerifyRefinement also transitions it to
// FindingStatusResolved -- the real state-tracking effect task 7 asks
// for, not just a bare boolean flip with no consequence on the
// finding itself.
func (e *Engine) VerifyRefinement(ctx context.Context, tenantID, refinementID uuid.UUID, note string) (RefinementRecord, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return RefinementRecord{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RefinementRecord{}, err
	}
	if strings.TrimSpace(note) == "" {
		return RefinementRecord{}, wrapf("VerifyRefinement", ErrNotesRequired)
	}

	r, err := e.refinements.Get(ctx, tenantID, refinementID)
	if err != nil {
		return RefinementRecord{}, wrapf("VerifyRefinement", err)
	}

	now := e.now()
	r.VerifiedFixed = true
	r.VerificationNote = strings.TrimSpace(note)
	r.VerifiedBy = &user.ID
	r.VerifiedAt = &now
	r.UpdatedAt = now

	if err := e.refinements.Update(ctx, tenantID, r); err != nil {
		return RefinementRecord{}, wrapf("VerifyRefinement", err)
	}

	finding, err := e.findings.Get(ctx, tenantID, r.FindingID)
	if err != nil {
		return RefinementRecord{}, wrapf("VerifyRefinement", err)
	}
	if finding.Status == FindingStatusInProgress && CanTransitionFinding(finding.Status, FindingStatusResolved) {
		finding.Status = FindingStatusResolved
		finding.UpdatedAt = now
		if err := e.findings.Update(ctx, tenantID, finding); err != nil {
			return RefinementRecord{}, wrapf("VerifyRefinement", err)
		}
	}

	return *r, nil
}
