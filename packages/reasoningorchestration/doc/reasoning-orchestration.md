# Reasoning orchestration (`packages/reasoningorchestration`)

Phase 059's goal, verbatim from the implementation plan:

> Coordinate the full agent sequence end to end.

By Phase 058, every unit of actual reasoning work already existed as an
independent, fully tested package: issue framing (`issueagent`,
Phase 050), first-party argument construction (`firstpartyagent`,
Phase 051), second-party argument construction and rebuttal
(`secondpartyagent`, Phase 052), evidence weighing (`evidenceweighing`,
Phase 053), law application (`lawapplication`, Phase 054), opinion
synthesis (`synthesisagent`, Phase 055), uncertainty surfacing
(`uncertainty`, Phase 056), the non-binding guardrail
(`guardrail`, Phase 057), and jurisdiction-parameterized reasoning weights
(`reasoningprofile`, Phase 058). Nothing coordinated them into a single
run for a case. This package is that coordinator.

## The pipeline, in dependency order

| # | Stage | Entrypoint called | Needs (from earlier stages) |
|---|-------|--------------------|------------------------------|
| 1 | `StageIssueFraming` | `issueagent.Analyze` | the case's tree (via `knowledgeapi`) only |
| 2 | `StageFirstPartyArguments` | `firstpartyagent.Argue` | `IssueAnalysisResult` (1) |
| 3 | `StageSecondPartyArguments` | `secondpartyagent.Argue` | `IssueAnalysisResult` (1), first party's `ArgumentSet` (2) |
| 4 | `StageEvidenceWeighing` | `evidenceweighing.Weigh` | both `ArgumentSet`s (2, 3), the tree's `FactRef`s |
| 5 | `StageLawApplication` | `lawapplication.Apply` | `IssueAnalysisResult` (1), both `ArgumentSet`s (2, 3), evidence `Result` (4) |
| 6 | `StageSynthesis` | `synthesisagent.Synthesize` | everything above (1-5) |
| 7 | `StageUncertaintySurfacing` | `uncertainty.Surface` | `IssueAnalysisResult` (1), evidence `Result` (4), law `Result` (5), `Opinion` (6) |
| 8 | `StageGuardrailCheck` | `guardrail.CheckText` + `guardrail.CanFinalize` | `Opinion` (6) |

## Why (mostly) sequential

This is not an implementation shortcut — every dependency above is each
upstream package's own documented design, not a constraint this package
invented:

- `secondpartyagent.New` takes the first party's `firstpartyagent.ArgumentSet`
  as a required constructor argument (for rebuttal targeting), so stage 3
  cannot start before stage 2 finishes.
- `evidenceweighing.Weigh` and `lawapplication.Apply` both take
  `firstpartyagent.ArgumentSet` and `secondpartyagent.ArgumentSet` as
  request fields; there is no way to call either with only one side's
  arguments computed if the plan calls for both.
- `synthesisagent.New` takes all five upstream results as constructor
  arguments — it is explicitly the package that reconciles everything
  else, so it must run last among the reasoning stages.
- `uncertainty.Surface` reads confidence/conflict signals out of the
  `Opinion`, `Result`, and `IssueAnalysisResult` types directly; it has
  nothing to rank before synthesis exists.
- `guardrail.CheckText`/`CanFinalize` check the synthesized opinion's
  text and the case's sign-off state — both only meaningful once an
  `Opinion` exists.

## Concurrency: exactly one genuine opportunity

Every stage above has a hard ordering dependency on the previous one,
**except** for one thing that runs alongside stage 1:
`reasoningprofile.ResolveFamily`/`WeightsForFamily`. Resolving this case's
legal-family weighting profile needs only the case's jurisdiction/
legal-family context (a plain string in `RunConfig.LegalFamily`) — not
`issueagent.Analyze`'s output — and issue framing does not need the
resolved `Weights` either. `runIssueFraming` (`stages.go`) therefore
starts the weights resolution on its own goroutine before calling
`issueagent.Analyze`, and joins it (`<-weightsDone`) before returning.

This is safe because:

- **No shared mutable state.** The goroutine only reads
  `cfg.LegalFamily` (a copy, since `RunConfig` is passed by value) and
  writes its result to a dedicated, single-use, buffered channel.
- **No ordering requirement.** Neither computation's result feeds the
  other's input.

No other stage pair in the chain has this property — every other stage
consumes a typed struct field that literally does not exist until the
prior stage returns, so there is nothing else to parallelize without
changing what each Part-5 package actually requires as input.

### Checkpoint persistence is the other place concurrency shows up

Separately from the reasoning-stage dependency chain, **persisting** a
completed stage's `Checkpoint` has no ordering dependency on the *next
stage starting*: the next stage always reads its inputs from the
in-process `pipelineContext` value carried through `drive`, never from
the `CheckpointStore`. `persistCheckpointAsync` (`run.go`) therefore
saves each `Checkpoint` on its own goroutine, so a slow or momentarily
unavailable `CheckpointStore` never blocks the pipeline's critical path.

This is bounded, not "fire and never wait": `drive` tracks every such
goroutine on a `sync.WaitGroup` and blocks on `Wait()` before returning
(via `defer`), so by the time a caller receives a `RunResult`, every
checkpoint for that run is guaranteed to be durable — the async behavior
only ever affects stage-to-stage latency, never the caller-visible
completion guarantee or the audit trail's completeness.

## Checkpointing and resume

`CheckpointStore` (`checkpoint.go`) persists two things, both keyed by
case:

- One `Checkpoint` per completed `Stage`, carrying that stage's real
  typed output (`IssueAnalysisResult`, either `ArgumentSet`,
  `evidenceweighing.Result`, `lawapplication.Result`,
  `synthesisagent.Opinion`, `uncertainty.Report`, or the guardrail
  approval flag) — exactly one populated field per `Checkpoint`, selected
  by its `Stage`.
- The overall `RunState` (`CurrentStage`, `CompletedStages`,
  `Termination`, and failure detail if any).

`Resume(ctx, caseID, cfg)`:

1. Loads the case's last-persisted `RunState`.
2. Rebuilds an in-memory `pipelineContext` by reading back every
   `Checkpoint` named in `RunState.CompletedStages` — this is why every
   stage's real output (not a placeholder) must be checkpointed: a
   resumed run's later stages need the actual `ArgumentSet`/`Result`
   values, not just a boolean "this stage happened".
3. Continues `drive` from the first stage not already in
   `CompletedStages`.

Every stage already in `CompletedStages` is never re-executed — this is
the whole point: `issueagent.Analyze`, `firstpartyagent.Argue`,
`secondpartyagent.Argue`, and `synthesisagent.Synthesize` are billed LLM
calls, so re-running one after a crash/restart would silently double that
cost. `InMemoryCheckpointStore` is the only implementation provided here,
mirroring `evidenceweighing.InMemoryRepository`'s and
`lawapplication.InMemoryRepository`'s own in-memory-first convention; a
production deployment supplies its own durable `CheckpointStore`.

## Budget model

Two independent budgets apply, at two different scopes:

- **`PipelineBudget.PerStageBudget`** (`agentframework.Budget`) is passed
  straight through as the `Budget` field of every LLM-agent stage's own
  `Config` (`AnalyzeConfig`/`ArgueConfig`/`SynthesizeConfig`) — the same
  per-run step/wall-clock/token bound each Part-5 package already
  enforces on its own, unchanged.
- **`PipelineBudget.MaxTotalWallClock`** bounds the pipeline's cumulative
  spend across *every* stage, LLM-backed and deterministic alike. `drive`
  checks `elapsed := now - RunState.StartedAt` against this budget
  **before starting each stage**, never mid-stage: if it would be
  exceeded, the run halts immediately with
  `RunState.Termination = TerminationBudgetExhausted` and
  `RunResult.Err` wrapping `ErrBudgetExhausted`. A resumed run's elapsed
  time is measured from the *original* `StartedAt`, so `MaxTotalWallClock`
  bounds a case's total processing time across every attempt, not any
  single process's uptime.

## Failure isolation

If any stage's entrypoint returns an error, `drive` stops immediately:
`RunState.Termination = TerminationFailed`, `RunState.FailedStage` names
the stage, `RunState.FailureReason` carries the error text, and — this is
the important guarantee — **no later stage is ever attempted** with
partial or zero-value input. `RunResult.Err` is set in this case too, so
a caller that only checks the returned error (rather than inspecting
`RunState.Termination`) still gets correct behavior. Every earlier
stage's `Checkpoint` remains intact in the `CheckpointStore` regardless
of a later failure, so a subsequent `Resume` call (once the underlying
issue is fixed) picks up exactly where the pipeline stopped.

## How a caller is expected to use this package

A case-workspace UI (Part 6) or an API caller drives one case through the
full reasoning pipeline by calling `Run` once (or `Resume` after a
crash/restart), rather than calling `issueagent`, `firstpartyagent`,
`secondpartyagent`, `evidenceweighing`, `lawapplication`,
`synthesisagent`, `uncertainty`, and `guardrail` individually in the
correct order themselves. `RunConfig` bundles every dependency (the
`knowledgeapi.KnowledgeAPI`, the `router.Router`, both parties'
identifiers, jurisdiction context, the `CheckpointStore`, and the
budgets) so a caller supplies wiring exactly once per case rather than
per stage. Every stage's output remains individually retrievable via
`CheckpointStore.GetCheckpoint` after the run — this doubles as the
audit trail a future case-workspace UI needs to show "what did the
system conclude, and from what" without re-deriving it.

## What this package deliberately does not do

- It does not call any `provider.LLMProvider` directly — every model call
  is reached through the same `*router.Router` each Part-5 package
  already requires, preserving Verdex's model-agnostic-by-construction
  rule.
- It does not implement its own retry/backoff policy beyond what
  `agentframework.Runner` already provides per LLM-agent stage; a failed
  stage stops the run (see "Failure isolation" above) rather than being
  silently retried.
- It does not implement a durable `CheckpointStore` backend —
  `InMemoryCheckpointStore` is the only implementation here, mirroring
  every sibling package's in-memory-first convention.
- It does not implement human sign-off itself. `StageGuardrailCheck`
  calls `guardrail.CanFinalize` against whatever `guardrail.SignoffGate`
  the caller supplies (defaulting to the fail-closed
  `guardrail.NoSignoffRecordedGate`), the same extension point Phase 068
  is expected to implement — a not-yet-approved sign-off is recorded on
  the `StageGuardrailCheck` `Checkpoint`, not treated as a pipeline
  failure, since the reasoning work itself is still complete either way.
- It does not reweigh, reinterpret, or override any upstream package's
  computation — `reasoningprofile.Weights` is threaded through only as
  informational context (resolved concurrently with issue framing, see
  above); `evidenceweighing` and `lawapplication` continue to key their
  own internal profiles off `LegalFamily` directly, exactly as they did
  before this package existed.
