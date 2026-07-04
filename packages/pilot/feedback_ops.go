package pilot

import (
	"context"

	"github.com/google/uuid"
)

// SubmitFeedback records a reviewer's structured FeedbackEntry against
// pilotCaseID (task 4), requiring managePermission and tenant match --
// submitting feedback is treated as a managed write like every other
// pilot-lifecycle mutation in this package, not a separate
// lower-privilege action, since a pilot's collected feedback is
// exactly the sensitive input its quality/trust measurement and
// findings pipeline runs on. The referenced PilotCase must already
// exist for tenantID.
func (e *Engine) SubmitFeedback(ctx context.Context, tenantID uuid.UUID, entry FeedbackEntry) (FeedbackEntry, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return FeedbackEntry{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return FeedbackEntry{}, err
	}

	if _, err := e.cases.Get(ctx, tenantID, entry.PilotCaseID); err != nil {
		return FeedbackEntry{}, wrapf("SubmitFeedback", err)
	}

	entry.TenantID = tenantID
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.ReviewerUserID == uuid.Nil {
		entry.ReviewerUserID = user.ID
	}
	entry.Comments = trimmedComments(entry.Comments)
	now := e.now()
	if entry.SubmittedAt.IsZero() {
		entry.SubmittedAt = now
	}
	entry.CreatedAt = now
	entry.UpdatedAt = now

	if err := entry.Validate(); err != nil {
		return FeedbackEntry{}, err
	}
	if err := e.feedback.Create(ctx, tenantID, &entry); err != nil {
		return FeedbackEntry{}, wrapf("SubmitFeedback", err)
	}
	return entry, nil
}

// GetFeedback returns the FeedbackEntry identified by id for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) GetFeedback(ctx context.Context, tenantID, id uuid.UUID) (FeedbackEntry, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return FeedbackEntry{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return FeedbackEntry{}, err
	}
	f, err := e.feedback.Get(ctx, tenantID, id)
	if err != nil {
		return FeedbackEntry{}, wrapf("GetFeedback", err)
	}
	return *f, nil
}

// ListFeedbackForCase returns every FeedbackEntry recorded against
// pilotCaseID for tenantID, requiring viewPermission and tenant match.
func (e *Engine) ListFeedbackForCase(ctx context.Context, tenantID, pilotCaseID uuid.UUID) ([]FeedbackEntry, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.feedback.ListForCase(ctx, tenantID, pilotCaseID)
	if err != nil {
		return nil, wrapf("ListFeedbackForCase", err)
	}
	return list, nil
}

// ListFeedbackForDeployment returns every FeedbackEntry recorded
// against any PilotCase assigned under deploymentID for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) ListFeedbackForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]FeedbackEntry, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	cases, err := e.cases.ListForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		return nil, wrapf("ListFeedbackForDeployment", err)
	}
	caseIDs := make([]uuid.UUID, len(cases))
	for i, c := range cases {
		caseIDs[i] = c.ID
	}
	list, err := e.feedback.ListForDeployment(ctx, tenantID, caseIDs)
	if err != nil {
		return nil, wrapf("ListFeedbackForDeployment", err)
	}
	return list, nil
}
