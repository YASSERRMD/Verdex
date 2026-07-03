// Package reasoningeval continuously evaluates the quality of live
// production reasoning outputs — the synthesized packages/synthesisagent
// Opinions actually shown to judges and advocates — as opposed to
// packages/eval, which benchmarks candidate models against a fixed golden
// set before a model is ever deployed.
//
// # Relationship to packages/eval
//
// packages/eval answers "is this model good enough to deploy?" using
// hand-written prompts, golden answers, and a ScorerFn that compares raw
// text similarity. reasoningeval answers "is the reasoning we are
// producing right now, on real cases, still good?" by scoring actual
// Opinions against a weighted Rubric whose dimensions call into other
// packages' own verification logic rather than re-deriving it from text
// comparison. Where useful this package reuses packages/eval's shapes
// directly (RegressionGate's threshold-drop algorithm and
// InMemoryResultStore's store-by-run-id pattern) instead of duplicating
// them.
//
// # Rubric dimensions
//
// A Rubric (rubric.go) is a named, weighted list of Dimensions. This
// package ships three built-in dimensions:
//
//   - Grounding: delegates to packages/grounding.Check and folds its
//     Report.OpinionScore into the rubric.
//   - Citation: delegates to packages/citation's Finding/Severity
//     convention (via the grounding.Report's own CitationFindings, which
//     already carries packages/citation.Finding values) and scores the
//     fraction of citation findings that are not critical.
//   - Coherence: a structural, non-binding-safe heuristic over the
//     Opinion's own text (issue coverage, conclusion completeness) that
//     never overrides or weakens packages/guardrail's verdict-language
//     ban — see coherence.go.
//
// # Automated scoring, expert review, regression detection
//
// Score (score.go) runs a Rubric over a single Opinion (using a
// grounding.Report as an input, since Check itself requires network/store
// access this package does not perform on the caller's behalf) and
// produces a QualityScore. ExpertReview (review.go) is a separate,
// human-authored assessment of a sampled Opinion, stored alongside but
// never merged with automated QualityScores. RegressionDetector
// (regression.go) compares two sets of QualityScores (e.g. before/after a
// prompt template or model change) and flags a statistically meaningful
// drop, mirroring packages/eval.RegressionGate's threshold convention.
//
// # Aggregation, dashboard, alerting, persistence
//
// Aggregate (aggregate.go) groups QualityScores by jurisdiction code and
// (optionally) packages/reasoningprofile legal family. Dashboard
// (dashboard.go) is a small read-only facade over a Store, mirroring
// packages/knowledgeapi's stable-facade style. AlertSink (alert.go)
// mirrors packages/observability and packages/accounting's sink
// interface; QualityAlertChecker wires a drop threshold to it. Store
// (store.go) persists QualityScores, ExpertReviews, and Alerts, mirroring
// packages/eval.ResultStore's shape with an in-memory implementation.
//
// # Non-binding guardrail
//
// Every QualityScore, Alert, and dashboard summary produced by this
// package concerns the *quality of a non-binding draft analysis*, never a
// verdict on the underlying case. This package raises no conclusion-like
// text of its own; where a dashboard summary or alert message might read
// as authoritative, callers should treat it exactly as
// packages/guardrail's DraftAnalysisLabel already treats an Opinion's own
// text: informational, non-binding, subject to human review.
//
// See doc/reasoning-quality-evaluation.md for the full design.
package reasoningeval
