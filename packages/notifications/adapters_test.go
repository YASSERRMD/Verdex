package notifications_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/notifications"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

func TestSignoffNotificationSink_ProducesNotificationForResolvedRecipients(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	judge := newTestUser(tenantID)
	caseID := uuid.New()

	resolver := func(_ context.Context, gotTenant, gotCase uuid.UUID) ([]uuid.UUID, error) {
		if gotTenant != tenantID || gotCase != caseID {
			t.Fatalf("resolver called with unexpected scope: tenant=%s case=%s", gotTenant, gotCase)
		}
		return []uuid.UUID{judge.ID}, nil
	}

	sink := notifications.NewSignoffNotificationSink(svc, resolver)

	// This is the exact call signoff.NotificationSink.Notify's callers
	// (packages/signoff's gate/rereview machinery) make.
	var asInterface signoff.NotificationSink = sink

	err := asInterface.Notify(context.Background(), signoff.PendingSignoffEvent{
		TenantID:    tenantID,
		CaseID:      caseID,
		Reason:      "case never reviewed",
		CaseVersion: 1,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	list, err := svc.List(ctxWithUser(judge), tenantID, judge.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1 notification for resolved recipient, got %d", len(list))
	}
	if list[0].Kind != notifications.KindPendingSignoff {
		t.Fatalf("List: expected Kind %q, got %q", notifications.KindPendingSignoff, list[0].Kind)
	}
	if list[0].CaseID == nil || *list[0].CaseID != caseID {
		t.Fatalf("List: expected CaseID %s on the notification", caseID)
	}
}

func TestAnnotationsMentionSink_ProducesNotificationForMentionedUser(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	mentioned := newTestUser(tenantID)
	caseID := uuid.New()
	annotationID := uuid.New()

	sink := notifications.NewAnnotationsMentionSink(svc)
	var asInterface annotations.MentionSink = sink

	err := asInterface.Notify(context.Background(), annotations.Mention{
		AnnotationID:    annotationID,
		CaseID:          caseID,
		TenantID:        tenantID,
		AuthorID:        uuid.New(),
		MentionedUserID: mentioned.ID,
		CreatedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	list, err := svc.List(ctxWithUser(mentioned), tenantID, mentioned.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1 notification for mentioned user, got %d", len(list))
	}
	if list[0].Kind != notifications.KindMention {
		t.Fatalf("List: expected Kind %q, got %q", notifications.KindMention, list[0].Kind)
	}
	if list[0].RelatedEntityID == nil || *list[0].RelatedEntityID != annotationID {
		t.Fatalf("List: expected RelatedEntityID %s on the notification", annotationID)
	}
}

func TestReasoningEvalAlertSink_ProducesNotificationForConfiguredRecipients(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	auditor := newTestUser(tenantID)

	sink := notifications.NewReasoningEvalAlertSink(svc, tenantID, []uuid.UUID{auditor.ID})
	var asInterface reasoningeval.AlertSink = sink

	alert := reasoningeval.NewRegressionAlert("US-CA", reasoningeval.RegressionResult{
		BaselineRunID: "run-a",
		CurrentRunID:  "run-b",
		BaselineAvg:   0.9,
		CurrentAvg:    0.6,
		Drop:          0.3,
		Regressed:     true,
	})

	if err := asInterface.Send(context.Background(), alert); err != nil {
		t.Fatalf("Send: %v", err)
	}

	list, err := svc.List(ctxWithUser(auditor), tenantID, auditor.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1 notification for configured recipient, got %d", len(list))
	}
	if list[0].Kind != notifications.KindQualityAlert {
		t.Fatalf("List: expected Kind %q, got %q", notifications.KindQualityAlert, list[0].Kind)
	}
}

func TestAccountingAlertSink_ProducesNotificationForConfiguredRecipients(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	admin := newTestUser(tenantID)

	sink := notifications.NewAccountingAlertSink(svc, []uuid.UUID{admin.ID})
	var asInterface accounting.AlertSink = sink

	err := asInterface.Send(context.Background(), accounting.AlertEvent{
		TenantID:       tenantID,
		AlertType:      accounting.AlertTypeBudgetExceeded,
		CurrentUsage:   120000,
		Limit:          100000,
		CurrentCostUSD: 42.5,
		LimitUSD:       40,
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	list, err := svc.List(ctxWithUser(admin), tenantID, admin.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1 notification for configured recipient, got %d", len(list))
	}
	if list[0].Kind != notifications.KindBudgetAlert {
		t.Fatalf("List: expected Kind %q, got %q", notifications.KindBudgetAlert, list[0].Kind)
	}
}

func TestNotifyIngestionComplete_ProducesNotification(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)
	caseID := uuid.New()

	n, err := notifications.NotifyIngestionComplete(context.Background(), svc, tenantID, caseID, recipient.ID, 3)
	if err != nil {
		t.Fatalf("NotifyIngestionComplete: %v", err)
	}
	if n == nil || n.Kind != notifications.KindIngestionComplete {
		t.Fatalf("NotifyIngestionComplete: expected a KindIngestionComplete notification, got %+v", n)
	}
}

func TestNotifyTaskAssignment_ProducesNotification(t *testing.T) {
	svc := newTestService(t)
	tenantID := uuid.New()
	recipient := newTestUser(tenantID)
	caseID := uuid.New()

	n, err := notifications.NotifyTaskAssignment(context.Background(), svc, tenantID, caseID, recipient.ID, "", "please review")
	if err != nil {
		t.Fatalf("NotifyTaskAssignment: %v", err)
	}
	if n == nil || n.Kind != notifications.KindTaskAssignment {
		t.Fatalf("NotifyTaskAssignment: expected a KindTaskAssignment notification, got %+v", n)
	}
}
