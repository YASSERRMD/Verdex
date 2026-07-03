package signoff_test

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/signoff"
)

func TestService_MarkAwaitingSignoff_FiresNotification(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, notifier := newTestService(caseID)

	rec, err := svc.MarkAwaitingSignoff(context.Background(), tenantID, caseID)
	if err != nil {
		t.Fatalf("MarkAwaitingSignoff: %v", err)
	}
	if rec == nil {
		t.Fatal("expected a SignoffRecord")
	}
	if notifier.count() != 1 {
		t.Fatalf("expected 1 notification, got %d", notifier.count())
	}
}

func TestLoggingNotificationSink_WritesEvent(t *testing.T) {
	var buf bytes.Buffer
	sink := &signoff.LoggingNotificationSink{Logger: log.New(&buf, "", 0)}

	caseID := uuid.New()
	tenantID := uuid.New()
	err := sink.Notify(context.Background(), signoff.PendingSignoffEvent{
		TenantID:    tenantID,
		CaseID:      caseID,
		Reason:      "case awaiting human sign-off",
		CaseVersion: 1,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, caseID.String()) {
		t.Fatalf("expected log output to contain case id, got %q", out)
	}
	if !strings.Contains(out, "pending sign-off") {
		t.Fatalf("expected log output to mention pending sign-off, got %q", out)
	}
}

func TestMultiNotificationSink_FansOutToAllSinks(t *testing.T) {
	a := &recordingNotificationSink{}
	b := &recordingNotificationSink{}
	multi := &signoff.MultiNotificationSink{Sinks: []signoff.NotificationSink{a, b}}

	err := multi.Notify(context.Background(), signoff.PendingSignoffEvent{
		TenantID:    uuid.New(),
		CaseID:      uuid.New(),
		Reason:      "test",
		CaseVersion: 1,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if a.count() != 1 || b.count() != 1 {
		t.Fatalf("expected both sinks to receive the event, got a=%d b=%d", a.count(), b.count())
	}
}

func TestNoOpNotificationSink_DiscardsSilently(t *testing.T) {
	var sink signoff.NoOpNotificationSink
	if err := sink.Notify(context.Background(), signoff.PendingSignoffEvent{}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
