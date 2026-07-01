// Package eval provides a model evaluation harness for comparing LLM providers
// on legal reasoning tasks.
//
// The harness defines structured evaluation tasks (EvalTask) with golden
// answers and scoring rubrics. It can run a set of tasks against one or more
// providers, collect results (EvalResult), aggregate them into reports
// (EvalReport), and enforce regression gates that catch score regressions
// relative to a known-good baseline.
//
// # Core Concepts
//
// An EvalTask describes a single legal reasoning scenario. Each task has a
// Prompt (the text sent to the model), a GoldenAnswer (the expected ideal
// response), and a ScoringRubric that is a weighted list of RubricCriteria.
// Each criterion carries a ScorerFn: a pure function that takes the model
// output and the golden answer and returns a float64 in [0, 1].
//
// Built-in scorers cover exact match, keyword presence, citation presence,
// non-binding compliance, and jurisdiction accuracy.  Custom scorers can be
// composed freely.
//
// EvalRunner drives execution: for every (provider, task) pair it calls
// provider.Chat with Temperature=0 to ensure determinism, times the round
// trip, applies the rubric, and stores an EvalResult.
//
// GenerateReport aggregates a slice of EvalResults into an EvalReport,
// computing per-provider AvgScore and latency percentiles.  RegressionGate
// compares a new report against a baseline and returns an error if any
// provider's average score drops by more than the configured threshold.
//
// # Quick Start
//
//	gs := eval.SeedLegalGoldenSet()
//	store := eval.NewInMemoryGoldenStore()
//	_ = store.Save(ctx, gs)
//
//	runner := eval.NewEvalRunner([]provider.LLMProvider{myProvider}, &gs)
//	results, err := runner.RunAll(ctx)
//	report := eval.GenerateReport(results)
//
// # Non-binding Compliance
//
// All scorers are purely informational. Verdex surfaces rubric scores to help
// practitioners select models but does not issue legal advice or binding
// determinations.
package eval
