package ingestion

import (
	"context"
	"errors"
	"testing"
)

func TestRunWithRetry_SucceedsFirstTry(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	job := newAudioJob("job-1", "case-1")
	calls := 0

	fn := func(_ context.Context, _ Job, state WorkflowState) (WorkflowState, error) {
		calls++
		return state, nil
	}

	_, err := RunWithRetry(context.Background(), store, RetryPolicy{}, job, StageIntake, WorkflowState{}, fn)
	if err != nil {
		t.Fatalf("RunWithRetry: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}

	rec, ok := store.Get(job.JobID, StageIntake)
	if !ok || !rec.Completed {
		t.Errorf("record = %+v, ok=%v; want Completed=true", rec, ok)
	}
}

func TestRunWithRetry_SucceedsAfterTransientFailures(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	job := newAudioJob("job-1", "case-1")
	calls := 0

	fn := func(_ context.Context, _ Job, state WorkflowState) (WorkflowState, error) {
		calls++
		if calls < 3 {
			return state, errors.New("transient")
		}
		return state, nil
	}

	_, err := RunWithRetry(context.Background(), store, RetryPolicy{MaxAttempts: 5}, job, StageExtraction, WorkflowState{}, fn)
	if err != nil {
		t.Fatalf("RunWithRetry: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRunWithRetry_ExhaustsRetries(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	job := newAudioJob("job-1", "case-1")
	calls := 0

	fn := func(_ context.Context, _ Job, state WorkflowState) (WorkflowState, error) {
		calls++
		return state, errors.New("permanent failure")
	}

	_, err := RunWithRetry(context.Background(), store, RetryPolicy{MaxAttempts: 3}, job, StageNormalize, WorkflowState{}, fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Errorf("err = %v, want wrapping ErrRetriesExhausted", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}

	rec, ok := store.Get(job.JobID, StageNormalize)
	if !ok || rec.Completed {
		t.Errorf("record = %+v, ok=%v; want Completed=false", rec, ok)
	}
	if rec.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", rec.Attempts)
	}
}

func TestRunWithRetry_IdempotentNoOpWhenAlreadyCompleted(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	job := newAudioJob("job-1", "case-1")
	store.MarkAttempt(job.JobID, StageSegment)
	store.MarkCompleted(job.JobID, StageSegment)

	calls := 0
	fn := func(_ context.Context, _ Job, state WorkflowState) (WorkflowState, error) {
		calls++
		return state, nil
	}

	_, err := RunWithRetry(context.Background(), store, RetryPolicy{}, job, StageSegment, WorkflowState{}, fn)
	if err != nil {
		t.Fatalf("RunWithRetry: %v", err)
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0 (should be a no-op)", calls)
	}
}

func TestRunWithRetry_DefaultMaxAttempts(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	job := newAudioJob("job-1", "case-1")
	calls := 0

	fn := func(_ context.Context, _ Job, state WorkflowState) (WorkflowState, error) {
		calls++
		return state, errors.New("always fails")
	}

	_, err := RunWithRetry(context.Background(), store, RetryPolicy{}, job, StageClassify, WorkflowState{}, fn)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != DefaultMaxAttempts {
		t.Errorf("calls = %d, want %d", calls, DefaultMaxAttempts)
	}
}

func TestInMemoryIdempotencyStore_Reset(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	store.MarkAttempt("job-1", StageIntake)
	store.MarkCompleted("job-1", StageIntake)

	store.Reset("job-1", StageIntake)

	_, ok := store.Get("job-1", StageIntake)
	if ok {
		t.Error("expected no record after Reset")
	}
}
