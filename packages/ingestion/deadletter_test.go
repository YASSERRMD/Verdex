package ingestion

import (
	"testing"
	"time"
)

func TestInMemoryDeadLetterQueue_AddAndGet(t *testing.T) {
	q := NewInMemoryDeadLetterQueue()
	if _, ok := q.Get("job-1"); ok {
		t.Fatal("expected no entry before Add")
	}

	dl := DeadLetter{
		JobID:    "job-1",
		CaseID:   "case-1",
		Stage:    StageExtraction,
		Reason:   "retries exhausted",
		Attempts: 3,
		FailedAt: time.Now(),
	}
	q.Add(dl)

	got, ok := q.Get("job-1")
	if !ok {
		t.Fatal("expected entry after Add")
	}
	if got.Stage != StageExtraction || got.Attempts != 3 {
		t.Errorf("got = %+v", got)
	}
}

func TestInMemoryDeadLetterQueue_AddOverwrites(t *testing.T) {
	q := NewInMemoryDeadLetterQueue()
	q.Add(DeadLetter{JobID: "job-1", Stage: StageIntake, Reason: "first"})
	q.Add(DeadLetter{JobID: "job-1", Stage: StageClassify, Reason: "second"})

	got, _ := q.Get("job-1")
	if got.Reason != "second" || got.Stage != StageClassify {
		t.Errorf("got = %+v, want most recent entry", got)
	}
}

func TestInMemoryDeadLetterQueue_List(t *testing.T) {
	q := NewInMemoryDeadLetterQueue()
	q.Add(DeadLetter{JobID: "job-1", Stage: StageIntake})
	q.Add(DeadLetter{JobID: "job-2", Stage: StageSegment})

	got := q.List()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestInMemoryDeadLetterQueue_Remove(t *testing.T) {
	q := NewInMemoryDeadLetterQueue()
	q.Add(DeadLetter{JobID: "job-1", Stage: StageIntake})
	q.Remove("job-1")

	if _, ok := q.Get("job-1"); ok {
		t.Error("expected no entry after Remove")
	}
	if len(q.List()) != 0 {
		t.Error("expected empty List after Remove")
	}
}
