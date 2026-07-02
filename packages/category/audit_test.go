package category

import (
	"context"
	"testing"
	"time"
)

func TestNoOpAuditSink_Emit(t *testing.T) {
	var sink NoOpAuditSink
	err := sink.Emit(context.Background(), CategoryAuditEvent{EventType: AuditEventSuggested})
	if err != nil {
		t.Errorf("Emit() error = %v, want nil", err)
	}
}

func TestCapturingAuditSink_Emit(t *testing.T) {
	sink := &CapturingAuditSink{}

	events := []CategoryAuditEvent{
		{EventType: AuditEventSuggested, CaseID: "case-1", CategoryCode: CodeCivil, Timestamp: time.Now()},
		{EventType: AuditEventValidated, CaseID: "case-1", CategoryCode: CodeCivil, Timestamp: time.Now()},
		{EventType: AuditEventOverridden, CaseID: "case-1", CategoryCode: CodeConsumer, Timestamp: time.Now()},
		{EventType: AuditEventChanged, CaseID: "case-1", CategoryCode: CodeConsumer, Timestamp: time.Now()},
	}

	for _, e := range events {
		if err := sink.Emit(context.Background(), e); err != nil {
			t.Fatalf("Emit() error = %v, want nil", err)
		}
	}

	if len(sink.Events) != len(events) {
		t.Fatalf("got %d captured events, want %d", len(sink.Events), len(events))
	}

	// Audit trail completeness: every distinct EventType we emitted must be
	// present and in the order emitted.
	for i, want := range events {
		got := sink.Events[i]
		if got.EventType != want.EventType {
			t.Errorf("event[%d].EventType = %q, want %q", i, got.EventType, want.EventType)
		}
		if got.CaseID != want.CaseID {
			t.Errorf("event[%d].CaseID = %q, want %q", i, got.CaseID, want.CaseID)
		}
		if got.CategoryCode != want.CategoryCode {
			t.Errorf("event[%d].CategoryCode = %q, want %q", i, got.CategoryCode, want.CategoryCode)
		}
	}
}

func TestLoggingAuditSink_Emit(t *testing.T) {
	sink := NewLoggingAuditSink(nil)
	err := sink.Emit(context.Background(), CategoryAuditEvent{
		EventType:        AuditEventSuggested,
		CaseID:           "case-1",
		JurisdictionCode: "IN",
		CategoryCode:     CodeCivil,
		Actor:            "system:keyword-suggester",
		Confidence:       0.6,
		Timestamp:        time.Now(),
	})
	if err != nil {
		t.Errorf("Emit() error = %v, want nil", err)
	}
}

func TestAuditEventTypeConstants(t *testing.T) {
	// Guard against accidental renames/typos of the event-type labels
	// relied on by CategoryService and by external consumers of the audit
	// trail.
	tests := map[string]string{
		AuditEventSuggested:  "category.suggested",
		AuditEventOverridden: "category.overridden",
		AuditEventChanged:    "category.changed",
		AuditEventValidated:  "category.validated",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
