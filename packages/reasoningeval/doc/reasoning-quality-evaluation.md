# Reasoning quality evaluation (`packages/reasoningeval`)

Phase 062's goal, verbatim from the implementation plan:

> Continuously evaluate reasoning output quality.

## Why this is not `packages/eval`

`packages/eval` (Phase 018) is a **pre-deployment model evaluation
harness**: it runs hand-written `EvalTask` prompts against candidate LLM
providers, compares the raw text output to a golden answer via a
`ScorerFn`, and gates deployment on a `RegressionGate` before a model
ever serves real cases.

`packages/reasoningeval` (this package) evaluates **live production
reasoning output** — the actual `synthesisagent.Opinion` a judge or
advocate is shown for a real case — continuously, after deployment. It
never re-derives the correctness of an Opinion from scratch the way
`eval`'s `ScorerFn`s compare text to a golden answer; instead its rubric
dimensions call directly into the packages that already independently
verify an Opinion's claims (`packages/grounding`, `packages/citation`)
and a lightweight structural heuristic of its own (coherence). Where
useful, this package reuses `eval`'s shapes and algorithms rather than
reinventing them:

- `RegressionDetector` (`regression.go`) mirrors `eval.RegressionGate`'s
  baseline/threshold comparison algorithm, adapted from "per-provider
  average score" to "per-run average `QualityScore.Overall`".
- `InMemoryStore` (`store.go`) mirrors `eval.InMemoryResultStore`'s
  in-process, thread-safe, insertion-ordered persistence shape, extended
  to reasoningeval's three record kinds (scores, reviews, alerts) instead
  of `eval`'s single `EvalReport`.
- `Dimension`/`Rubric` (`types.go`) mirror `eval.RubricCriteria`'s
  named/weighted/scorer shape, but the scorer signature takes a
  structured `ScoreInput` instead of two bare strings, since a
  reasoning-quality dimension needs a `grounding.Report`, not a text diff.

## The rubric

A `Rubric` (`rubric.go`) is a named, weighted list of `Dimension`s.
`DefaultRubric()` ships three:

| Dimension | Weight (default) | What it measures | Delegates to |
|---|---|---|---|
| `DimensionGrounding` | 0.4 | How well the Opinion's assertions are grounded in case facts/law | `packages/grounding.Report.OpinionScore`, folded in directly |
| `DimensionCitation` | 0.3 | Citation fidelity | Fraction of `packages/citation.Finding`s (carried on the `grounding.Report`) that are **not** critical |
| `DimensionCoherence` | 0.3 | Structural completeness of the reasoning | Issue coverage, conclusion substance, and reported confidence — see below |

Each `Dimension.Scorer` returns a value in `[0.0, 1.0]`; `Score` (in
`score.go`) computes the weighted aggregate the same way
`eval.applyRubric` does: `sum(raw_i * weight_i) / sum(weight_i)`, clamped
to `[0, 1]`.

Callers may compose their own `Rubric` from the built-in `Dimension`
constructors (`GroundingDimension`, `CitationDimension`,
`CoherenceDimension`) with different weights, or add entirely custom
dimensions — `DimensionScorer` is a plain function type, not a closed
interface.

### Why grounding and citation are not re-derived here

This package deliberately never calls `grounding.Check` itself: that
function requires a `graph.GraphStore` and an authenticated
`context.Context`, both of which belong to the caller orchestrating a
full evaluation pass, not to a pure scoring function. Instead, `ScoreInput`
takes an already-computed `grounding.Report` (via `WrapGroundingReport`)
and folds its verdict in. This keeps `Score` itself a pure function safe
to call in a tight loop over historical Opinions, and it guarantees the
grounding dimension can never silently drift from what
`packages/grounding` itself considers "grounded" — there is exactly one
grounding algorithm in the codebase, owned by `packages/grounding`.

### Coherence and the non-binding guardrail

`CoherenceDimension` (`coherence.go`) is intentionally **structural
only**: it counts how many issues reached a conclusion versus were
skipped, whether each conclusion's text clears a minimum substantive
length, and the mean of the pipeline's own per-conclusion `Confidence`
values. It never inspects an Opinion's prose for verdict-like language —
that check belongs exclusively to `packages/guardrail`
(`irac.ContainsVerdictLanguage`), which `packages/synthesisagent` already
runs on every `TentativeConclusion` before it can ever reach an `Opinion`
(see `synthesisagent`'s `ConclusionProvider` adapter). By construction,
every Opinion this package scores has already passed guardrail's check,
so `CoherenceDimension` composes with that guarantee instead of
duplicating (and risking drifting out of sync with) it.

## Automated scoring vs. expert review

`Score` / `ScoreBatch` (`score.go`) produce a `QualityScore`: the
mechanical, rubric-driven assessment. `ExpertReview` (`review.go`) is a
**separate, human-authored** structure — a reviewer's own `Score`,
free-text `Comments`, and any `FlaggedIssues` — stored alongside but never
merged into a `QualityScore`. This separation is deliberate: a caller
comparing automated and human judgment on the same Opinion needs both
numbers intact, not a single blended score that hides disagreement
between the rubric and a legal expert.

## Regression detection

`RegressionDetector.Compare` (`regression.go`) takes two slices of
`QualityScore` — a baseline run and a current run, distinguished by
`QualityScore.RunID` — and computes the drop in mean `Overall` score, plus
a per-`DimensionName` breakdown of which axis moved. `Regressed` is `true`
only when the drop exceeds `Threshold`, an absolute value in `[0, 1]`
(mirroring `eval.RegressionGate.Threshold`'s semantics exactly, so a
threshold tuned for one package translates directly to the other).
`CompareErr` wraps a positive result in `ErrRegressionDetected` for
callers preferring the "check returns an error" idiom `eval.RegressionGate.Check`
uses.

## Per-jurisdiction and legal-family tracking

`AggregateByJurisdiction` and `AggregateByLegalFamily` (`aggregate.go`)
group `QualityScore`s by `JurisdictionCode` and (for scores that carry
one) `LegalFamily` respectively. `LegalFamily` is expected to be resolved
by the caller via `packages/reasoningprofile.ResolveFamily` from a
`packages/jurisdiction.Jurisdiction` before constructing a `ScoreInput` —
this package does not import `packages/reasoningprofile`'s resolution
logic itself, only carries the resulting string, so a jurisdiction whose
family assignment changes does not require a `reasoningeval` release to
pick up.

## Alerting on quality drop

`AlertSink` (`alert.go`) mirrors `packages/accounting.AlertSink`'s and
`packages/reasoningprofile.AlertSink`'s interface shape exactly:
`LoggingAlertSink`, `NoOpAlertSink`, and `MultiAlertSink` are provided.
`QualityAlertChecker` wires a `RegressionDetector` to a sink: `Check`
always returns the `RegressionResult`, but sends exactly one `Alert` only
when `Regressed` is true. Every `Alert.Message` carries a
non-binding-quality-signal suffix, since an alert reporting a reasoning
*quality* regression must never be misread as a conclusion about the
merits of the underlying cases it was computed from.

`packages/observability`'s `Counter`/`Gauge`/`Histogram` primitives are a
natural place to also emit a metric per `Check` call (e.g. a gauge for
the latest `Overall` average per jurisdiction) — this package exposes the
data needed to do so (`RegressionResult`, `JurisdictionSummary`) but does
not itself depend on `packages/observability`, leaving the wiring to
whatever service constructs the periodic evaluation job, consistent with
`packages/accounting`'s own choice not to hard-depend on a specific
metrics backend from its alerting types.

## Persistence and the dashboard API

`Store` (`store.go`) persists `QualityScore`, `ExpertReview`, and `Alert`
records, mirroring `eval.ResultStore`'s shape. `InMemoryStore` is the
in-process implementation; a durable implementation (e.g. backed by
`packages/persistence`) can satisfy the same interface without this
package changing.

`Dashboard` (`dashboard.go`) is a thin, stable read-only facade over a
`Store`, mirroring `packages/knowledgeapi.KnowledgeAPI`'s "facade over a
lower-level store" convention: `JurisdictionTrend`, `LegalFamilyTrend`,
`RecentAlerts`, and `CaseReviews` are its four read operations, each
gated on `RequireViewPermission` (requiring `identity.PermViewCase`,
mirroring `packages/grounding.RequireCheckPermission` and
`packages/reasoningtrace.RequireViewPermission`) before touching the
store. An HTTP layer over `Dashboard` — if a later phase needs one — can
follow `packages/knowledgeapi/http.go`'s `Handler` pattern directly.

## Access control

Every `Store` read reachable through `Dashboard` requires
`identity.PermViewCase` on the calling context's `identity.User`, the
same permission `packages/grounding` and `packages/reasoningtrace` gate
case-scoped reasoning content on. `Store` itself (used directly, bypassing
`Dashboard`) does not enforce this — the same trust boundary
`eval.InMemoryResultStore` and `grounding`'s own store layer assume: a
`Store` implementation may sit behind its own service boundary with its
own access control, and `Dashboard` is the case-scoped entry point
intended for reasoning-quality consumers.
