package perf

import (
	"context"
	"testing"
	"time"
)

func benchmarkRun(runID string, p95 time.Duration, throughput float64, recordedAt time.Time) BenchmarkRun {
	return BenchmarkRun{
		RunID:     runID,
		Operation: OpGraphTraversal,
		Measurement: Measurement{
			P50:        p95 / 2,
			P95:        p95,
			P99:        p95 * 2,
			Throughput: throughput,
		},
		DeploymentTag: "test",
		RecordedAt:    recordedAt,
	}
}

func TestDetectRegression_NoRegressionWhenComparable(t *testing.T) {
	historical := []BenchmarkRun{
		benchmarkRun("baseline-1", 100*time.Millisecond, 50, time.Now().Add(-2*time.Hour)),
		benchmarkRun("baseline-2", 110*time.Millisecond, 48, time.Now().Add(-time.Hour)),
	}
	current := benchmarkRun("current-1", 105*time.Millisecond, 49, time.Now())

	if DetectRegression(current, historical) {
		t.Fatal("expected no regression for a comparable current run")
	}
}

func TestDetectRegression_LatencyRegression(t *testing.T) {
	historical := []BenchmarkRun{
		benchmarkRun("baseline-1", 100*time.Millisecond, 50, time.Now().Add(-time.Hour)),
	}
	// P95 more than doubled: a 100% increase, well past the 20% threshold.
	current := benchmarkRun("current-1", 250*time.Millisecond, 50, time.Now())

	if !DetectRegression(current, historical) {
		t.Fatal("expected a regression when P95 latency more than doubles")
	}
}

func TestDetectRegression_ThroughputRegression(t *testing.T) {
	historical := []BenchmarkRun{
		benchmarkRun("baseline-1", 100*time.Millisecond, 100, time.Now().Add(-time.Hour)),
	}
	// Throughput dropped by 50%, well past the 20% threshold.
	current := benchmarkRun("current-1", 100*time.Millisecond, 50, time.Now())

	if !DetectRegression(current, historical) {
		t.Fatal("expected a regression when throughput drops by half")
	}
}

func TestDetectRegression_EmptyHistoricalIsNotARegression(t *testing.T) {
	current := benchmarkRun("current-1", 100*time.Millisecond, 50, time.Now())
	if DetectRegression(current, nil) {
		t.Fatal("expected no regression when there is no historical baseline to compare against")
	}
}

func TestCompareRuns_IgnoresDifferentOperations(t *testing.T) {
	historical := []BenchmarkRun{
		{
			RunID:       "other-op",
			Operation:   OpHybridRetrieval,
			Measurement: Measurement{P95: time.Nanosecond, Throughput: 1},
			RecordedAt:  time.Now().Add(-time.Hour),
		},
	}
	current := benchmarkRun("current-1", 100*time.Millisecond, 50, time.Now())

	result := CompareRuns(current, historical)
	if result.BaselineP95 != 0 {
		t.Fatalf("expected a run of a different Operation to be ignored, got BaselineP95=%v", result.BaselineP95)
	}
}

func TestBenchmarkRun_ValidateRejectsMalformedRuns(t *testing.T) {
	valid := BenchmarkRun{RunID: "r1", Operation: OpIngestionPipeline, Measurement: Measurement{Throughput: 1}}
	if err := valid.validate(); err != nil {
		t.Fatalf("expected well-formed run to validate, got %v", err)
	}

	missingRunID := valid
	missingRunID.RunID = ""
	if err := missingRunID.validate(); err == nil {
		t.Error("expected validation error for missing RunID")
	}

	missingOperation := valid
	missingOperation.Operation = ""
	if err := missingOperation.validate(); err == nil {
		t.Error("expected validation error for missing Operation")
	}

	invalidMeasurement := valid
	invalidMeasurement.Measurement = Measurement{P50: -1}
	if err := invalidMeasurement.validate(); err == nil {
		t.Error("expected validation error for invalid Measurement")
	}
}

func TestInMemoryStore_SaveAndListRuns(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	run1 := benchmarkRun("r1", 100*time.Millisecond, 50, time.Now().Add(-time.Hour))
	run2 := benchmarkRun("r2", 110*time.Millisecond, 48, time.Now())

	if err := store.SaveRun(ctx, run1); err != nil {
		t.Fatalf("SaveRun returned unexpected error: %v", err)
	}
	if err := store.SaveRun(ctx, run2); err != nil {
		t.Fatalf("SaveRun returned unexpected error: %v", err)
	}

	all, err := store.ListRuns(ctx, "")
	if err != nil {
		t.Fatalf("ListRuns returned unexpected error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(all))
	}
	// Most recent first.
	if all[0].RunID != "r2" {
		t.Fatalf("expected most-recent-first ordering, got %q first", all[0].RunID)
	}

	filtered, err := store.ListRuns(ctx, OpGraphTraversal)
	if err != nil {
		t.Fatalf("ListRuns with filter returned unexpected error: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected both runs to match OpGraphTraversal filter, got %d", len(filtered))
	}

	none, err := store.ListRuns(ctx, OpIngestionPipeline)
	if err != nil {
		t.Fatalf("ListRuns with non-matching filter returned unexpected error: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected 0 runs for a non-matching operation filter, got %d", len(none))
	}
}

func TestInMemoryStore_SaveRunRejectsInvalidRun(t *testing.T) {
	store := NewInMemoryStore()
	err := store.SaveRun(context.Background(), BenchmarkRun{})
	if err == nil {
		t.Fatal("expected SaveRun to reject a zero-value BenchmarkRun")
	}
}

func TestInMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func(i int) {
			run := benchmarkRun(rangeRunID(i), time.Duration(i)*time.Millisecond, float64(i+1), time.Now())
			_ = store.SaveRun(ctx, run)
			_, _ = store.ListRuns(ctx, "")
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

func rangeRunID(i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	return "run-" + string(letters[i%len(letters)])
}
