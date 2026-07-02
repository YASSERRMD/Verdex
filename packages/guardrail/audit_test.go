package guardrail_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

func TestNoOpAlertSinkDiscardsSilently(t *testing.T) {
	// Must not panic.
	guardrail.NoOpAlertSink{}.Notify(guardrail.Event{Kind: guardrail.ViolationVerdictLanguage})
}

func TestFuncAlertSinkNilSafe(t *testing.T) {
	var sink guardrail.FuncAlertSink
	// Must not panic when the wrapped function is nil.
	sink.Notify(guardrail.Event{Kind: guardrail.ViolationMissingLabel})
}

func TestFuncAlertSinkCallsFunc(t *testing.T) {
	var got guardrail.Event
	sink := guardrail.FuncAlertSink(func(e guardrail.Event) { got = e })

	sink.Notify(guardrail.Event{Kind: guardrail.ViolationFinalizeBlocked, CaseID: "case-1"})

	if got.Kind != guardrail.ViolationFinalizeBlocked || got.CaseID != "case-1" {
		t.Fatalf("FuncAlertSink did not forward event correctly: %+v", got)
	}
}

func TestMultiAlertSinkFansOut(t *testing.T) {
	var count1, count2 int
	sink := guardrail.MultiAlertSink{Sinks: []guardrail.AlertSink{
		guardrail.FuncAlertSink(func(guardrail.Event) { count1++ }),
		nil, // must tolerate a nil child sink
		guardrail.FuncAlertSink(func(guardrail.Event) { count2++ }),
	}}

	sink.Notify(guardrail.Event{Kind: guardrail.ViolationVerdictLanguage})

	if count1 != 1 || count2 != 1 {
		t.Fatalf("MultiAlertSink did not fan out to all children: count1=%d count2=%d", count1, count2)
	}
}

func TestRecorderRecordsAndForwards(t *testing.T) {
	var forwarded []guardrail.Event
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	rec := guardrail.NewRecorder(
		guardrail.FuncAlertSink(func(e guardrail.Event) { forwarded = append(forwarded, e) }),
		func() time.Time { return fixedTime },
	)

	rec.Record(guardrail.Event{Kind: guardrail.ViolationVerdictLanguage, CaseID: "case-1", Detail: "test"})

	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	if events[0].OccurredAt != fixedTime {
		t.Fatalf("Events()[0].OccurredAt = %v, want %v", events[0].OccurredAt, fixedTime)
	}
	if len(forwarded) != 1 || forwarded[0].CaseID != "case-1" {
		t.Fatalf("event was not forwarded to sink: %+v", forwarded)
	}
}

func TestRecorderDefaultsNilSinkAndClock(t *testing.T) {
	rec := guardrail.NewRecorder(nil, nil)
	rec.Record(guardrail.Event{Kind: guardrail.ViolationMissingLabel})

	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	if events[0].OccurredAt.IsZero() {
		t.Fatal("Events()[0].OccurredAt is zero, want a real timestamp from the default clock")
	}
}

func TestRecorderEventsReturnsDefensiveCopy(t *testing.T) {
	rec := guardrail.NewRecorder(nil, nil)
	rec.Record(guardrail.Event{Kind: guardrail.ViolationVerdictLanguage})

	events := rec.Events()
	events[0].CaseID = "mutated"

	again := rec.Events()
	if again[0].CaseID == "mutated" {
		t.Fatal("Events() did not return a defensive copy; mutation leaked into internal state")
	}
}

func TestRecorderConcurrentAccess(t *testing.T) {
	rec := guardrail.NewRecorder(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.Record(guardrail.Event{Kind: guardrail.ViolationVerdictLanguage})
			_ = rec.Events()
		}()
	}
	wg.Wait()

	if len(rec.Events()) != 50 {
		t.Fatalf("Events() len = %d, want 50", len(rec.Events()))
	}
}

func TestRecorderRecordCheckTextFailure(t *testing.T) {
	rec := guardrail.NewRecorder(nil, nil)

	// nil error is a no-op.
	rec.RecordCheckTextFailure("case-1", nil)
	if len(rec.Events()) != 0 {
		t.Fatalf("RecordCheckTextFailure(nil) recorded an event, want none")
	}

	rec.RecordCheckTextFailure("case-1", errors.New("boom"))
	events := rec.Events()
	if len(events) != 1 || events[0].Kind != guardrail.ViolationVerdictLanguage {
		t.Fatalf("RecordCheckTextFailure did not record ViolationVerdictLanguage: %+v", events)
	}
}

func TestRecorderRecordLabelFailure(t *testing.T) {
	rec := guardrail.NewRecorder(nil, nil)

	rec.RecordLabelFailure("case-1", nil)
	if len(rec.Events()) != 0 {
		t.Fatalf("RecordLabelFailure(nil) recorded an event, want none")
	}

	rec.RecordLabelFailure("case-1", errors.New("missing label"))
	events := rec.Events()
	if len(events) != 1 || events[0].Kind != guardrail.ViolationMissingLabel {
		t.Fatalf("RecordLabelFailure did not record ViolationMissingLabel: %+v", events)
	}
}

func TestRecorderRecordFinalizeBlocked(t *testing.T) {
	rec := guardrail.NewRecorder(nil, nil)

	rec.RecordFinalizeBlocked("case-1", nil)
	if len(rec.Events()) != 0 {
		t.Fatalf("RecordFinalizeBlocked(nil) recorded an event, want none")
	}

	rec.RecordFinalizeBlocked("case-1", errors.New("not approved"))
	events := rec.Events()
	if len(events) != 1 || events[0].Kind != guardrail.ViolationFinalizeBlocked {
		t.Fatalf("RecordFinalizeBlocked did not record ViolationFinalizeBlocked: %+v", events)
	}
}
