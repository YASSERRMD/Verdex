package eval_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/eval"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// noOpWithContent wraps provider.NoOpProvider and lets tests override the fixed
// content returned so scorers see a controllable output.
type noOpWithContent struct {
	*provider.NoOpProvider
}

func newNoOp(content string) *noOpWithContent {
	p := provider.DefaultNoOpProvider()
	p.FixedContent = content
	return &noOpWithContent{p}
}

// --- RunAll tests ---

func TestRunAll_ReturnOneResultPerProviderPerTask(t *testing.T) {
	gs := eval.SeedLegalGoldenSet()
	p1 := newNoOp("answer one")
	p2 := newNoOp("answer two")

	runner := eval.NewEvalRunner([]provider.LLMProvider{p1, p2}, &gs)
	results, err := runner.RunAll(context.Background())
	if err != nil {
		t.Fatalf("RunAll returned error: %v", err)
	}

	want := len(gs.Tasks) * 2 // two providers
	if len(results) != want {
		t.Errorf("got %d results, want %d", len(results), want)
	}
}

func TestRunAll_EachResultHasExpectedProviderAndTask(t *testing.T) {
	gs := eval.GoldenSet{
		Version: "test-v1",
		Tasks: []eval.EvalTask{
			{
				ID:       "t1",
				Name:     "Task 1",
				Category: eval.CategoryReasoning,
				Prompt:   "Analyse a negligence case.",
				ScoringRubric: []eval.RubricCriteria{
					eval.ContainsKeywordsScorer([]string{"duty"}, 1.0),
				},
			},
		},
	}
	p := newNoOp("duty was established")

	runner := eval.NewEvalRunner([]provider.LLMProvider{p}, &gs)
	results, err := runner.RunAll(context.Background())
	if err != nil {
		t.Fatalf("RunAll returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.TaskID != "t1" {
		t.Errorf("TaskID: got %q, want %q", r.TaskID, "t1")
	}
	if r.ProviderID != "noop" {
		t.Errorf("ProviderID: got %q, want %q", r.ProviderID, "noop")
	}
}

func TestRunAll_NilGoldenSetReturnsError(t *testing.T) {
	runner := eval.NewEvalRunner([]provider.LLMProvider{newNoOp("x")}, nil)
	_, err := runner.RunAll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, eval.ErrNoGoldenSet) {
		t.Errorf("expected ErrNoGoldenSet, got %v", err)
	}
}

func TestRunAll_NoProvidersReturnsError(t *testing.T) {
	gs := eval.SeedLegalGoldenSet()
	runner := eval.NewEvalRunner(nil, &gs)
	_, err := runner.RunAll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, eval.ErrEvalFailed) {
		t.Errorf("expected ErrEvalFailed, got %v", err)
	}
}

func TestRun_UnknownProviderReturnsError(t *testing.T) {
	gs := eval.SeedLegalGoldenSet()
	runner := eval.NewEvalRunner([]provider.LLMProvider{newNoOp("x")}, &gs)
	_, err := runner.Run(context.Background(), "nonexistent", gs.Tasks[0])
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, eval.ErrEvalFailed) {
		t.Errorf("expected ErrEvalFailed, got %v", err)
	}
}

// --- Regression gate tests ---

func TestRegressionGate_PassesWhenScoreUnchanged(t *testing.T) {
	baseline := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.8},
		},
	}
	current := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.8},
		},
	}
	gate := eval.NewRegressionGate(baseline, 0.05)
	passed, regressions, err := gate.Check(current)
	if !passed {
		t.Errorf("expected pass, got regressions: %v", regressions)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestRegressionGate_DetectsScoreDrop(t *testing.T) {
	baseline := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.9},
		},
	}
	current := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.5},
		},
	}
	gate := eval.NewRegressionGate(baseline, 0.05)
	passed, regressions, err := gate.Check(current)
	if passed {
		t.Error("expected failure, got pass")
	}
	if len(regressions) == 0 {
		t.Error("expected at least one regression message")
	}
	if !errors.Is(err, eval.ErrRegressionDetected) {
		t.Errorf("expected ErrRegressionDetected, got %v", err)
	}
}

func TestRegressionGate_AllowsDropWithinThreshold(t *testing.T) {
	baseline := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.9},
		},
	}
	current := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.86}, // drop = 0.04, threshold = 0.05
		},
	}
	gate := eval.NewRegressionGate(baseline, 0.05)
	passed, _, err := gate.Check(current)
	if !passed {
		t.Errorf("expected pass within threshold, got err: %v", err)
	}
}

func TestRegressionGate_MissingProviderFails(t *testing.T) {
	baseline := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"noop": {ProviderID: "noop", AvgScore: 0.9},
		},
	}
	current := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{}, // noop missing
	}
	gate := eval.NewRegressionGate(baseline, 0.05)
	passed, regressions, err := gate.Check(current)
	if passed {
		t.Error("expected failure when provider is missing from current report")
	}
	if len(regressions) == 0 {
		t.Error("expected at least one regression message")
	}
	if !errors.Is(err, eval.ErrRegressionDetected) {
		t.Errorf("expected ErrRegressionDetected, got %v", err)
	}
}

// --- GenerateReport tests ---

func TestGenerateReport_SummaryContainsAllProviders(t *testing.T) {
	results := []eval.EvalResult{
		{TaskID: "t1", ProviderID: "alpha", Score: 0.8},
		{TaskID: "t2", ProviderID: "alpha", Score: 0.6},
		{TaskID: "t1", ProviderID: "beta", Score: 0.7},
	}
	report := eval.GenerateReport(results)
	if _, ok := report.Summary["alpha"]; !ok {
		t.Error("expected summary for 'alpha'")
	}
	if _, ok := report.Summary["beta"]; !ok {
		t.Error("expected summary for 'beta'")
	}
	alphaAvg := report.Summary["alpha"].AvgScore
	if alphaAvg != 0.7 {
		t.Errorf("alpha avg score: got %.4f, want 0.7", alphaAvg)
	}
}

func TestRankProviders_OrdersByDescendingScore(t *testing.T) {
	report := eval.EvalReport{
		Summary: map[string]eval.ProviderSummary{
			"low":  {ProviderID: "low", AvgScore: 0.3},
			"high": {ProviderID: "high", AvgScore: 0.9},
			"mid":  {ProviderID: "mid", AvgScore: 0.6},
		},
	}
	ranked := eval.RankProviders(report)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked providers, got %d", len(ranked))
	}
	if ranked[0] != "high" {
		t.Errorf("expected 'high' first, got %q", ranked[0])
	}
	if ranked[1] != "mid" {
		t.Errorf("expected 'mid' second, got %q", ranked[1])
	}
	if ranked[2] != "low" {
		t.Errorf("expected 'low' third, got %q", ranked[2])
	}
}
