package ingestion

import (
	"testing"
	"time"
)

func TestPercentCompleteForStage(t *testing.T) {
	tests := []struct {
		stage Stage
		want  int
	}{
		{StageIntake, 20},
		{StageExtraction, 40},
		{StageNormalize, 60},
		{StageSegment, 80},
		{StageClassify, 95},
		{StageComplete, 100},
		{StageFailed, 0},
		{Stage("bogus"), 0},
	}
	for _, tt := range tests {
		if got := PercentCompleteForStage(tt.stage); got != tt.want {
			t.Errorf("PercentCompleteForStage(%s) = %d, want %d", tt.stage, got, tt.want)
		}
	}
}

func TestInMemoryProgressReporter_ReportAndGet(t *testing.T) {
	r := NewInMemoryProgressReporter()

	if _, ok := r.Get("job-1"); ok {
		t.Fatal("expected no progress before any Report")
	}

	r.Report(Progress{JobID: "job-1", Stage: StageIntake, PercentComplete: 20})
	got, ok := r.Get("job-1")
	if !ok {
		t.Fatal("expected progress after Report")
	}
	if got.Stage != StageIntake || got.PercentComplete != 20 {
		t.Errorf("got = %+v", got)
	}

	r.Report(Progress{JobID: "job-1", Stage: StageExtraction, PercentComplete: 40})
	got, _ = r.Get("job-1")
	if got.Stage != StageExtraction {
		t.Errorf("Get after second Report = %+v, want Stage=%s", got, StageExtraction)
	}
}

func TestInMemoryProgressReporter_Subscribe(t *testing.T) {
	r := NewInMemoryProgressReporter()
	ch, unsubscribe := r.Subscribe("job-1")
	defer unsubscribe()

	r.Report(Progress{JobID: "job-1", Stage: StageIntake, PercentComplete: 20})

	select {
	case p := <-ch:
		if p.Stage != StageIntake {
			t.Errorf("Stage = %s, want %s", p.Stage, StageIntake)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive progress update")
	}

	// Updates for a different job must not be delivered to this subscriber.
	r.Report(Progress{JobID: "job-2", Stage: StageIntake, PercentComplete: 20})
	select {
	case p := <-ch:
		t.Fatalf("unexpected progress for other job: %+v", p)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestInMemoryProgressReporter_UnsubscribeStopsDelivery(t *testing.T) {
	r := NewInMemoryProgressReporter()
	ch, unsubscribe := r.Subscribe("job-1")
	unsubscribe()

	r.Report(Progress{JobID: "job-1", Stage: StageComplete, PercentComplete: 100})

	select {
	case p, ok := <-ch:
		if ok {
			t.Fatalf("received progress after unsubscribe: %+v", p)
		}
	case <-time.After(50 * time.Millisecond):
		// No delivery is also an acceptable outcome (channel not closed,
		// simply no longer registered).
	}
}

func TestInMemoryProgressReporter_ReportNeverBlocksOnFullSubscriber(t *testing.T) {
	r := NewInMemoryProgressReporter()
	_, unsubscribe := r.Subscribe("job-1")
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < subscriberBuffer*3; i++ {
			r.Report(Progress{JobID: "job-1", Stage: StageIntake, PercentComplete: i})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Report blocked on a slow/full subscriber")
	}
}
