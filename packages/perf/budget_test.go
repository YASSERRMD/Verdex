package perf

import (
	"errors"
	"testing"
	"time"
)

func TestEvaluate_PassingMeasurement(t *testing.T) {
	observed := Measurement{
		P50:        100 * time.Millisecond,
		P95:        400 * time.Millisecond,
		P99:        800 * time.Millisecond,
		Throughput: 25,
	}

	verdict, err := Evaluate(OpHybridRetrieval, observed)
	if err != nil {
		t.Fatalf("Evaluate returned unexpected error: %v", err)
	}
	if !verdict.Passed {
		t.Fatalf("expected overall Passed=true, got Verdict: %+v", verdict)
	}
	if !verdict.P50.Passed || !verdict.P95.Passed || !verdict.P99.Passed || !verdict.Throughput.Passed {
		t.Fatalf("expected every dimension to pass, got Verdict: %+v", verdict)
	}
	if verdict.Operation != OpHybridRetrieval {
		t.Fatalf("expected Operation %q, got %q", OpHybridRetrieval, verdict.Operation)
	}
}

func TestEvaluate_ViolatingMeasurement(t *testing.T) {
	// Every dimension deliberately violates packages/perf's
	// OpGraphTraversal budget (p50<80ms, p95<300ms, p99<600ms,
	// throughput>=40/s).
	observed := Measurement{
		P50:        500 * time.Millisecond,
		P95:        900 * time.Millisecond,
		P99:        2 * time.Second,
		Throughput: 5,
	}

	verdict, err := Evaluate(OpGraphTraversal, observed)
	if err != nil {
		t.Fatalf("Evaluate returned unexpected error: %v", err)
	}
	if verdict.Passed {
		t.Fatalf("expected overall Passed=false, got Verdict: %+v", verdict)
	}
	if verdict.P50.Passed {
		t.Errorf("expected P50 to fail (observed 500ms > target 80ms)")
	}
	if verdict.P95.Passed {
		t.Errorf("expected P95 to fail (observed 900ms > target 300ms)")
	}
	if verdict.P99.Passed {
		t.Errorf("expected P99 to fail (observed 2s > target 600ms)")
	}
	if verdict.Throughput.Passed {
		t.Errorf("expected Throughput to fail (observed 5/s < target 40/s)")
	}
}

func TestEvaluate_PartialViolation(t *testing.T) {
	// P50 passes, everything else fails: overall must still be false.
	observed := Measurement{
		P50:        50 * time.Millisecond,
		P95:        900 * time.Millisecond,
		P99:        2 * time.Second,
		Throughput: 5,
	}

	verdict, err := Evaluate(OpGraphTraversal, observed)
	if err != nil {
		t.Fatalf("Evaluate returned unexpected error: %v", err)
	}
	if !verdict.P50.Passed {
		t.Errorf("expected P50 to pass (observed 50ms <= target 80ms)")
	}
	if verdict.Passed {
		t.Fatalf("expected overall Passed=false when any dimension fails, got Verdict: %+v", verdict)
	}
}

func TestEvaluate_UnknownOperation(t *testing.T) {
	_, err := Evaluate(OperationName("does_not_exist"), Measurement{Throughput: 1})
	if !errors.Is(err, ErrUnknownOperation) {
		t.Fatalf("expected ErrUnknownOperation, got %v", err)
	}
}

func TestEvaluate_InvalidMeasurement(t *testing.T) {
	_, err := Evaluate(OpHybridRetrieval, Measurement{P50: -1})
	if !errors.Is(err, ErrInvalidMeasurement) {
		t.Fatalf("expected ErrInvalidMeasurement, got %v", err)
	}
}

func TestDefaultBudgets_AllValid(t *testing.T) {
	budgets := DefaultBudgets()
	if len(budgets) == 0 {
		t.Fatal("expected at least one default budget")
	}
	seen := make(map[OperationName]struct{})
	for _, b := range budgets {
		if err := b.validate(); err != nil {
			t.Errorf("budget for %q failed validation: %v", b.Operation, err)
		}
		if _, dup := seen[b.Operation]; dup {
			t.Errorf("duplicate operation name %q in DefaultBudgets", b.Operation)
		}
		seen[b.Operation] = struct{}{}
	}

	for _, want := range []OperationName{OpHybridRetrieval, OpGraphTraversal, OpIngestionPipeline} {
		if _, ok := seen[want]; !ok {
			t.Errorf("expected DefaultBudgets to include operation %q", want)
		}
	}
}
