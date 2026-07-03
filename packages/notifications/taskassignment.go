package notifications

import (
	"context"

	"github.com/google/uuid"
)

// NotifyTaskAssignment persists a KindTaskAssignment Notification
// telling recipientID they have been assigned to review/act on caseID
// (e.g. "you are assigned to review case X"). Per task 5, this package
// introduces no new assignment engine — reviewer/assignee selection
// remains packages/caselifecycle's and packages/signoff's concern.
// This function is simply the entrypoint those (or any future) call
// sites invoke once they have decided who is assigned, mirroring how
// NotifyIngestionComplete (ingestion.go) is a documented entrypoint
// rather than an integration this package reaches out and performs
// itself.
func NotifyTaskAssignment(ctx context.Context, service *Service, tenantID, caseID, recipientID uuid.UUID, title, body string) (*Notification, error) {
	if title == "" {
		title = "You have been assigned to a case"
	}
	return service.Notify(ctx, NotifyInput{
		TenantID:    tenantID,
		RecipientID: recipientID,
		Kind:        KindTaskAssignment,
		Title:       title,
		Body:        body,
		CaseID:      &caseID,
	})
}
