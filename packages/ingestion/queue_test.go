package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryJobQueue_EnqueueDequeue(t *testing.T) {
	q := NewInMemoryJobQueue(1)
	job := Job{JobID: "job-1", CaseID: "case-1", Kind: InputAudio, Audio: &dummyAudio}

	if err := q.Enqueue(job); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got.JobID != "job-1" {
		t.Errorf("JobID = %q, want %q", got.JobID, "job-1")
	}
}

func TestInMemoryJobQueue_DequeueBlocksThenReceives(t *testing.T) {
	q := NewInMemoryJobQueue(0)
	done := make(chan struct{})

	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		job, err := q.Dequeue(ctx)
		if err != nil {
			t.Errorf("Dequeue: %v", err)
			return
		}
		if job.JobID != "job-async" {
			t.Errorf("JobID = %q, want %q", job.JobID, "job-async")
		}
	}()

	time.Sleep(20 * time.Millisecond)
	if err := q.Enqueue(Job{JobID: "job-async", CaseID: "case-1", Kind: InputAudio, Audio: &dummyAudio}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Dequeue never returned")
	}
}

func TestInMemoryJobQueue_DequeueContextCancelled(t *testing.T) {
	q := NewInMemoryJobQueue(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := q.Dequeue(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestInMemoryJobQueue_CloseRejectsNewEnqueues(t *testing.T) {
	q := NewInMemoryJobQueue(1)
	q.Close()

	err := q.Enqueue(Job{JobID: "job-1", CaseID: "case-1", Kind: InputAudio, Audio: &dummyAudio})
	if !errors.Is(err, ErrQueueClosed) {
		t.Errorf("err = %v, want ErrQueueClosed", err)
	}
}

func TestInMemoryJobQueue_CloseDrainsBufferedJobs(t *testing.T) {
	q := NewInMemoryJobQueue(2)
	if err := q.Enqueue(Job{JobID: "job-1", CaseID: "case-1", Kind: InputAudio, Audio: &dummyAudio}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	q.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue buffered job after Close: %v", err)
	}
	if got.JobID != "job-1" {
		t.Errorf("JobID = %q, want %q", got.JobID, "job-1")
	}

	_, err = q.Dequeue(ctx)
	if !errors.Is(err, ErrQueueClosed) {
		t.Errorf("err after drain = %v, want ErrQueueClosed", err)
	}
}

func TestInMemoryJobQueue_CloseIdempotent(t *testing.T) {
	q := NewInMemoryJobQueue(1)
	q.Close()
	q.Close() // must not panic
}
