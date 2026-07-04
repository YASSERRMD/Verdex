package perf

import (
	"context"
	"testing"
	"time"
)

func TestLimiter_AcquireRelease(t *testing.T) {
	l := NewLimiter(ResourceLimits{Traversal: 2})
	ctx := context.Background()

	if err := l.Acquire(ctx, ClassTraversal); err != nil {
		t.Fatalf("first Acquire returned unexpected error: %v", err)
	}
	if err := l.Acquire(ctx, ClassTraversal); err != nil {
		t.Fatalf("second Acquire returned unexpected error: %v", err)
	}
	l.Release(ClassTraversal)
	l.Release(ClassTraversal)
}

// TestLimiter_AcquireBeyondLimitBlocks proves (rather than merely asserts
// synchronously) that a third Acquire on a limit-2 semaphore blocks until a
// Release frees a slot: it launches the third Acquire in its own goroutine
// and uses a channel + select-with-timeout to confirm it has NOT completed
// immediately, then confirms it DOES complete promptly once Release runs.
func TestLimiter_AcquireBeyondLimitBlocks(t *testing.T) {
	l := NewLimiter(ResourceLimits{Retrieval: 2})
	ctx := context.Background()

	if err := l.Acquire(ctx, ClassRetrieval); err != nil {
		t.Fatalf("Acquire 1 returned unexpected error: %v", err)
	}
	if err := l.Acquire(ctx, ClassRetrieval); err != nil {
		t.Fatalf("Acquire 2 returned unexpected error: %v", err)
	}

	acquired := make(chan error, 1)
	go func() {
		acquired <- l.Acquire(ctx, ClassRetrieval)
	}()

	// The third Acquire must NOT complete while both slots are held.
	select {
	case err := <-acquired:
		t.Fatalf("expected third Acquire to block while limit is held, but it returned (err=%v)", err)
	case <-time.After(100 * time.Millisecond):
		// Expected: still blocked.
	}

	// Releasing one slot must unblock the waiter promptly.
	l.Release(ClassRetrieval)

	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("expected third Acquire to succeed after Release, got error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for third Acquire to unblock after Release")
	}

	// Clean up the two slots now held (the original 2nd Acquire, plus the
	// 3rd that just succeeded).
	l.Release(ClassRetrieval)
	l.Release(ClassRetrieval)
}

func TestLimiter_AcquireRespectsContextCancellation(t *testing.T) {
	l := NewLimiter(ResourceLimits{Ingestion: 1})
	ctx := context.Background()

	if err := l.Acquire(ctx, ClassIngestion); err != nil {
		t.Fatalf("Acquire 1 returned unexpected error: %v", err)
	}

	cancelCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := l.Acquire(cancelCtx, ClassIngestion)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Acquire to return an error once its context deadline passed")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected Acquire to return promptly after context deadline, took %v", elapsed)
	}
}

func TestLimiter_UnlimitedClassNeverBlocks(t *testing.T) {
	l := NewLimiter(ResourceLimits{}) // every class defaults to 0 == unlimited
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		if err := l.Acquire(ctx, ClassTraversal); err != nil {
			t.Fatalf("Acquire on unlimited class returned unexpected error: %v", err)
		}
	}
	// No Release needed/possible to overfill: unlimited classes are
	// always no-ops.
	l.Release(ClassTraversal)
}

func TestLimiter_UnknownClassErrors(t *testing.T) {
	l := NewLimiter(DefaultResourceLimits())
	err := l.Acquire(context.Background(), ResourceClass("does_not_exist"))
	if err == nil {
		t.Fatal("expected an error for an unrecognized ResourceClass")
	}
}

func TestDefaultResourceLimits_AllPositive(t *testing.T) {
	limits := DefaultResourceLimits()
	if limits.Ingestion <= 0 || limits.Retrieval <= 0 || limits.Traversal <= 0 {
		t.Fatalf("expected every DefaultResourceLimits field to be positive, got %+v", limits)
	}
}
