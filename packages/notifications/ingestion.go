package notifications

import (
	"context"
	"strconv"

	"github.com/google/uuid"
)

// NotifyIngestionComplete persists a KindIngestionComplete
// Notification telling recipientID that an evidence ingestion run for
// caseID has finished processing.
//
// Per task 2, this package deliberately does not modify
// packages/ingestion to call this function directly — packages/ingestion
// has no existing sink-style hook (unlike packages/signoff,
// packages/annotations, and packages/reasoningeval, which this phase
// wires real adapters for) and wiring one in is out of scope for a
// "don't modify ingestion unless trivial and justified" phase. Instead
// this is the documented entrypoint: once packages/ingestion's
// orchestration (see packages/ingestion's pipeline completion path)
// is ready to notify, its call site is exactly:
//
//	notifications.NotifyIngestionComplete(ctx, notificationService, tenantID, caseID, recipientID, documentCount)
//
// where recipientID is typically the user who initiated the ingestion
// run (or a case-assigned reviewer, once packages/caselifecycle
// exposes that lookup to packages/ingestion).
func NotifyIngestionComplete(ctx context.Context, service *Service, tenantID, caseID, recipientID uuid.UUID, documentCount int) (*Notification, error) {
	return service.Notify(ctx, NotifyInput{
		TenantID:    tenantID,
		RecipientID: recipientID,
		Kind:        KindIngestionComplete,
		Title:       "Ingestion complete",
		Body:        ingestionCompleteBody(documentCount),
		CaseID:      &caseID,
	})
}

func ingestionCompleteBody(documentCount int) string {
	switch {
	case documentCount == 1:
		return "1 document finished processing and is ready for review."
	case documentCount > 1:
		return strconv.Itoa(documentCount) + " documents finished processing and are ready for review."
	default:
		return "Ingestion finished processing and is ready for review."
	}
}
