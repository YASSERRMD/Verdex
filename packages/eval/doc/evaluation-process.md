# Model Evaluation Process

This document describes the evaluation harness used in Verdex to compare LLM
providers on legal reasoning tasks.

---

## Overview

The evaluation harness lives in `packages/eval`. It lets practitioners:

1. Define a **GoldenSet** of canonical legal reasoning tasks.
2. Run those tasks against one or more **LLM providers**.
3. Score each response with a weighted **rubric**.
4. Aggregate scores into an **EvalReport**.
5. Enforce **regression gates** so that regressions surface before reaching production.

---

## Task Categories

| Category | `Category` constant | Description |
|---|---|---|
| Retrieval | `CategoryRetrieval` | Locate relevant facts or statutory text |
| Reasoning | `CategoryReasoning` | IRAC analysis, rule application, multi-step inference |
| Citation fidelity | `CategoryCitationFidelity` | Accurate case names, docket numbers, statutory citations |
| Jurisdiction accuracy | `CategoryJurisdictionAccuracy` | Correct identification of governing jurisdiction |

---

## EvalTask Schema

```go
type EvalTask struct {
    ID            string
    Name          string
    Category      Category
    Prompt        string
    GoldenAnswer  string
    ScoringRubric []RubricCriteria
    Seed          int64
}
```

- **ID** must be globally unique across all golden sets (e.g. `negligence-001`).
- **GoldenAnswer** is the reference response used by all scorers.
- **Seed** is recorded for reproducibility; it does not affect provider calls
  because the runner always uses `Temperature=0`.

---

## Scoring Rubrics

Each `RubricCriteria` pairs a `Weight` with a `ScorerFn`:

```go
type ScorerFn func(got, expected string) float64
```

The final task score is the **weighted average** of all per-criterion scores,
normalised to `[0.0, 1.0]`.

### Built-in Scorers

| Constructor | What it measures |
|---|---|
| `ExactMatchScorer(weight)` | 1.0 if output equals golden (case-insensitive trim) |
| `ContainsKeywordsScorer(keywords, weight)` | Fraction of keywords present |
| `CitationPresenceScorer(citations, weight)` | Fraction of citations mentioned |
| `NonBindingComplianceScorer(weight)` | 1.0 if no binding-verdict phrasing detected |
| `JurisdictionAccuracyScorer(expected, weight)` | 1.0 if jurisdiction string found |

Scorers can be combined freely. A typical legal reasoning task uses two or three
criteria with different weights.

---

## Deterministic Evaluation

`EvalRunner.Run` always sets `ChatRequest.Temperature = 0`. This makes
responses as deterministic as the underlying provider allows and ensures that
repeated runs on the same task produce consistent scores.

---

## Running an Evaluation

```go
// 1. Load or create a golden set.
gs := eval.SeedLegalGoldenSet()

// 2. Create providers.
providers := []provider.LLMProvider{anthropicProvider, openAIProvider}

// 3. Run.
runner := eval.NewEvalRunner(providers, &gs)
results, err := runner.RunAll(ctx)

// 4. Generate report.
report := eval.GenerateReport(results)
report.GoldenVersion = gs.Version

// 5. Rank providers.
ranked := eval.RankProviders(report)
fmt.Println("Best provider:", ranked[0])
```

---

## Regression Gates

A `RegressionGate` compares a new report against a stored baseline:

```go
gate := eval.NewRegressionGate(baselineReport, 0.05)
passed, regressions, err := gate.Check(currentReport)
if !passed {
    for _, msg := range regressions {
        log.Println("REGRESSION:", msg)
    }
}
```

A provider **fails** the gate when its average score drops by more than
`Threshold` absolute points (e.g. 0.05 = 5 percentage points). Providers
present in the baseline but absent from the current report are treated as a
zero score.

Integrate the gate into CI to block model-version upgrades that regress
performance.

---

## Persistence

| Interface | In-memory implementation | Notes |
|---|---|---|
| `GoldenStore` | `InMemoryGoldenStore` | Versioned golden sets |
| `ResultStore` | `InMemoryResultStore` | Run reports, listed newest-first |

Both interfaces are designed for easy replacement with database or object-store
backends.

---

## Non-Binding Disclaimer

The evaluation harness and all scorer outputs are informational only. Verdex
does not issue legal advice, orders, or binding determinations. The
`NonBindingComplianceScorer` specifically detects and penalises model outputs
that contain language purporting to issue a verdict, order, or judgment.

---

## Extending the Harness

### Adding a custom scorer

```go
func ProximityScorer(weight float64) eval.RubricCriteria {
    return eval.RubricCriteria{
        Name:   "proximity",
        Weight: weight,
        Fn: func(got, expected string) float64 {
            // implement your metric
            return 0.0
        },
    }
}
```

### Adding new golden tasks

Append `EvalTask` entries to `SeedLegalGoldenSet()` or create a separate
`GoldenSet` for a specific practice area and save it with a new `Version`.

### Replacing the result store

Implement `ResultStore` against a PostgreSQL table or S3 bucket and inject it
wherever `InMemoryResultStore` is currently used.
