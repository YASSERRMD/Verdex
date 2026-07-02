package hybridretrieval

import (
	"context"
	"testing"
	"time"
)

func TestBudgetTracker_ZeroDurationDisabled(t *testing.T) {
	b := newBudgetTracker(0)
	if b.exhausted() {
		t.Errorf("expected a disabled tracker to never report exhausted")
	}
}

func TestBudgetTracker_NegativeDurationDisabled(t *testing.T) {
	b := newBudgetTracker(-time.Second)
	if b.exhausted() {
		t.Errorf("expected a negative-duration tracker to be disabled, not exhausted")
	}
}

func TestBudgetTracker_TinyDurationExhaustsQuickly(t *testing.T) {
	b := newBudgetTracker(time.Nanosecond)
	time.Sleep(time.Millisecond)
	if !b.exhausted() {
		t.Errorf("expected a 1ns budget to be exhausted after sleeping 1ms")
	}
}

func TestBudgetTracker_GenerousDurationNotExhausted(t *testing.T) {
	b := newBudgetTracker(time.Hour)
	if b.exhausted() {
		t.Errorf("expected a 1-hour budget to not be exhausted immediately")
	}
	if b.remaining() <= 0 {
		t.Errorf("expected positive remaining time")
	}
}

func TestBudgetTracker_WithDeadline_DisabledReturnsUncancelledCtx(t *testing.T) {
	b := newBudgetTracker(0)
	ctx, cancel := b.withDeadline(context.Background())
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Errorf("expected no deadline on ctx from a disabled tracker")
	}
}

func TestBudgetTracker_WithDeadline_EnabledSetsDeadline(t *testing.T) {
	b := newBudgetTracker(time.Minute)
	ctx, cancel := b.withDeadline(context.Background())
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Errorf("expected a deadline on ctx from an enabled tracker")
	}
}
