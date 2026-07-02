package multilingual_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/multilingual"
)

func TestNoOpAuditSink_Emit(t *testing.T) {
	sink := multilingual.NoOpAuditSink{}
	err := sink.Emit(context.Background(), multilingual.AuditEvent{
		Step:      multilingual.StepUnicodeNormalized,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Errorf("Emit() unexpected error: %v", err)
	}
}

func TestCapturingAuditSink_RecordsEvents(t *testing.T) {
	sink := &multilingual.CapturingAuditSink{}

	steps := []multilingual.AuditStep{
		multilingual.StepUnicodeNormalized,
		multilingual.StepScriptDetected,
		multilingual.StepRTLFlagged,
		multilingual.StepLegalTermNormalized,
		multilingual.StepTranslated,
		multilingual.StepTokenized,
	}

	for _, step := range steps {
		err := sink.Emit(context.Background(), multilingual.AuditEvent{
			Step:      step,
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Emit() unexpected error: %v", err)
		}
	}

	if len(sink.Events) != len(steps) {
		t.Fatalf("captured %d events, want %d", len(sink.Events), len(steps))
	}
	for i, step := range steps {
		if sink.Events[i].Step != step {
			t.Errorf("event[%d].Step = %v, want %v", i, sink.Events[i].Step, step)
		}
	}
}

func TestLoggingAuditSink_Emit(t *testing.T) {
	sink := multilingual.NewLoggingAuditSink(nil)
	err := sink.Emit(context.Background(), multilingual.AuditEvent{
		Step:       multilingual.StepTranslated,
		DocumentID: "doc-1",
		Language:   multilingual.LanguageArabic,
		Detail:     "applied=true target=en",
		Timestamp:  time.Now(),
	})
	if err != nil {
		t.Errorf("Emit() unexpected error: %v", err)
	}
}
