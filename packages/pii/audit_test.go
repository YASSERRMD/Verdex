package pii_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestCapturingAuditSink_RecordsEvents(t *testing.T) {
	sink := &pii.CapturingAuditSink{}

	event := pii.AuditEvent{
		EventType:  pii.EventDetect,
		Actor:      "user-1",
		MatchCount: 3,
	}
	if err := sink.Emit(context.Background(), event); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	if len(sink.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(sink.Events))
	}
	if sink.Events[0].EventType != pii.EventDetect {
		t.Errorf("Events[0].EventType = %q, want %q", sink.Events[0].EventType, pii.EventDetect)
	}
	if sink.Events[0].Actor != "user-1" {
		t.Errorf("Events[0].Actor = %q, want %q", sink.Events[0].Actor, "user-1")
	}
}

func TestNoOpAuditSink_DiscardsSilently(t *testing.T) {
	sink := pii.NoOpAuditSink{}
	if err := sink.Emit(context.Background(), pii.AuditEvent{EventType: pii.EventRedact}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
}

func TestLoggingAuditSink_DoesNotError(t *testing.T) {
	sink := pii.NewLoggingAuditSink(nil)
	err := sink.Emit(context.Background(), pii.AuditEvent{
		EventType:        pii.EventReveal,
		Actor:            "auditor-1",
		JurisdictionCode: "AE",
		Token:            "PERSON_1",
		Allowed:          true,
	})
	if err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
}

func TestPIIService_Process_EmitsDetectAndRedactAuditEvents(t *testing.T) {
	sink := &pii.CapturingAuditSink{}
	svc := pii.NewPIIService()
	svc.AuditSink = sink

	_, err := svc.Process(context.Background(), pii.ProcessRequest{
		Text:  "Contact Jane Doe at jane.doe@example.com.",
		Actor: "user-1",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(sink.Events) < 2 {
		t.Fatalf("len(Events) = %d, want at least 2 (detect + redact)", len(sink.Events))
	}

	var sawDetect, sawRedact bool
	for _, e := range sink.Events {
		if e.EventType == pii.EventDetect {
			sawDetect = true
		}
		if e.EventType == pii.EventRedact {
			sawRedact = true
		}
		if e.Actor != "user-1" {
			t.Errorf("event Actor = %q, want %q", e.Actor, "user-1")
		}
	}
	if !sawDetect || !sawRedact {
		t.Errorf("sawDetect=%v sawRedact=%v, want both true", sawDetect, sawRedact)
	}
}

func TestPIIService_Reveal_EmitsAuditEventRegardlessOfOutcome(t *testing.T) {
	sink := &pii.CapturingAuditSink{}
	pmap := pii.NewPseudonymMap(pii.AccessPolicyFunc(func(context.Context, string) bool { return false }))
	svc := &pii.PIIService{
		Detector:   pii.NewRuleBasedDetector(),
		Mode:       pii.ModePseudonymize,
		Pseudonyms: pmap,
		AuditSink:  sink,
	}

	token := pmap.TokenFor(pii.CategoryName, "Jane Doe")

	_, err := svc.Reveal(context.Background(), "attacker-1", "AE", token)
	if err == nil {
		t.Fatal("Reveal() error = nil, want ErrAccessDenied")
	}

	found := false
	for _, e := range sink.Events {
		if e.EventType == pii.EventReveal && e.Token == token && !e.Allowed {
			found = true
		}
	}
	if !found {
		t.Errorf("no denied EventReveal audit event recorded; events=%+v", sink.Events)
	}
}
