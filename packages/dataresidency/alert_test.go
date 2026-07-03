package dataresidency_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
)

// recordingAlertSink captures every ViolationEvent it receives, for
// assertions in tests.
type recordingAlertSink struct {
	mu     sync.Mutex
	events []dataresidency.ViolationEvent
}

func (r *recordingAlertSink) Send(_ context.Context, event dataresidency.ViolationEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *recordingAlertSink) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func TestLoggingAlertSink_SendDoesNotError(t *testing.T) {
	sink := &dataresidency.LoggingAlertSink{}
	event := dataresidency.ViolationEvent{
		DeploymentID:  uuid.New(),
		ViolationType: dataresidency.ViolationTransferBlocked,
		SourceRegion:  "eu",
		DestRegion:    "cn",
		Reason:        "not allowed",
		CreatedAt:     time.Now(),
	}
	if err := sink.Send(context.Background(), event); err != nil {
		t.Fatalf("LoggingAlertSink.Send: %v", err)
	}
}

func TestMultiAlertSink_FansOutToAllSinks(t *testing.T) {
	a := &recordingAlertSink{}
	b := &recordingAlertSink{}
	multi := &dataresidency.MultiAlertSink{Sinks: []dataresidency.AlertSink{a, b}}

	event := dataresidency.ViolationEvent{DeploymentID: uuid.New(), ViolationType: dataresidency.ViolationVerifyFailed}
	if err := multi.Send(context.Background(), event); err != nil {
		t.Fatalf("MultiAlertSink.Send: %v", err)
	}
	if a.count() != 1 || b.count() != 1 {
		t.Fatalf("expected both sinks to receive the event, got a=%d b=%d", a.count(), b.count())
	}
}

type erroringAlertSink struct{}

func (erroringAlertSink) Send(context.Context, dataresidency.ViolationEvent) error {
	return errors.New("boom")
}

func TestMultiAlertSink_ContinuesAfterOneSinkErrors(t *testing.T) {
	b := &recordingAlertSink{}
	multi := &dataresidency.MultiAlertSink{Sinks: []dataresidency.AlertSink{erroringAlertSink{}, b}}

	err := multi.Send(context.Background(), dataresidency.ViolationEvent{DeploymentID: uuid.New()})
	if err == nil {
		t.Fatal("expected MultiAlertSink to surface the first error")
	}
	if b.count() != 1 {
		t.Fatal("expected the second sink to still receive the event despite the first erroring")
	}
}

func TestNoOpAlertSink_NeverErrors(t *testing.T) {
	if err := (dataresidency.NoOpAlertSink{}).Send(context.Background(), dataresidency.ViolationEvent{}); err != nil {
		t.Fatalf("NoOpAlertSink.Send: %v", err)
	}
}
