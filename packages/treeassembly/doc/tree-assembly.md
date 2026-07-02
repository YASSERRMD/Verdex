# Verdex Tree Assembly

## Overview

`packages/treeassembly` assembles the full IRAC reasoning tree for a case
from components already extracted by earlier phases:

- **Issues** — `packages/issue`'s `IssueExtractionService` (Phase 033)
- **Facts** — `packages/fact`'s `FactConstructionService` (Phase 034)
- **Rules and Applications** — `packages/application`'s
  `ApplicationService` (Phases 035-037)

It does not extract or construct any of these itself. Its job is to
gather what already exists, wire it together into one validated,
versioned `Tree`, detect where the reasoning is incomplete, and persist
the result — with a clean, documented extension point for the one thing
it deliberately does not produce: conclusions.

---

## The Pipeline

```
compose -> validate integrity -> detect gaps -> version
  -> record telemetry -> persist -> return
```

`TreeAssemblyService.Assemble` runs the full pipeline in one call:

```go
svc := &treeassembly.TreeAssemblyService{}

result, err := svc.Assemble(ctx, treeassembly.AssemblyInput{
    CaseID:       "case-42",
    Issues:       issues,
    Rules:        rules,
    Facts:        facts,
    Applications: applications,
})
// result.Tree, result.ValidationIssues, result.Gaps
```

Each stage is also independently usable:

- `ComposeTree` (compose.go) gathers every node into one `Tree` and
  reconstructs edges from each node's recorded `irac.Provenance`,
  matched against the legal `(FromNodeType, EdgeType, ToNodeType)`
  triples declared in `packages/irac/edge.go`.
- `ValidateIntegrity` (integrity.go) wraps `irac.ValidateTree`.
- `DetectGaps` (gap.go) runs semantic checks beyond structural validity.
- `NextRevision` (revision.go) versions a `Tree` via `irac.NextRevision`.
- `ReassembleIncremental` (incremental.go) extends a previous `Tree` with
  new facts/applications without rebuilding it from scratch.
- `AssemblyTelemetry` / `Recorder` (telemetry.go) capture per-run counts
  and duration.
- `PersistTree` / `SnapshotStore` (persist.go) write nodes/edges via
  `graph.GraphStore` and snapshot the tree via `graph.Export`.

If the composed tree fails structural validation with any
`irac.ValidationIssue`, `Assemble` returns `ErrCriticalIntegrityFailure`
and does **not** persist the tree. This mirrors CONTRIBUTING.md's
guardrail spirit: a reasoning artifact must be trustworthy before it is
used or handed downstream.

---

## Why Conclusions Are Pluggable, Not Generated Here

The fixed IRAC schema (Issue, Rule, Fact, Application, Conclusion) was
established in Phase 031. Assembling a tree out of the first four node
types is a mechanical composition problem: gather what upstream services
already produced, and reconstruct the edges their provenance implies.

Producing the fifth — a `ConclusionNode` — is a different kind of
problem entirely. A conclusion is a reasoned, non-binding *analysis* of
what an `Application` implies, and getting that right requires an LLM
reasoning agent applying legal judgment to the assembled facts and
rules. That capability is explicitly **Phase 055 — "Synthesis &
reasoned-opinion agent"**, and it is out of scope for this phase.

Rather than leaving this package unable to produce a complete tree until
Phase 055 exists, or coupling it to a specific (not-yet-built) synthesis
implementation, `ComposeTree` and `TreeAssemblyService` accept a small
interface:

```go
type ConclusionProvider interface {
    Provide(ctx context.Context, input AssemblyInput) ([]irac.ConclusionNode, error)
}
```

`NoOpConclusionProvider` — the default used everywhere in this phase —
always returns an empty slice. Today, every tree this package assembles
has zero `ConclusionNode`s, and `DetectGaps` deliberately does **not**
flag "no conclusions anywhere" as a gap (see `GapUnresolvedApplication`'s
doc comment): the total absence of conclusions is the expected state
before Phase 055 exists, not a defect in this package's output.

Once Phase 055's synthesis agent exists, it need only implement
`ConclusionProvider.Provide` and be supplied as
`TreeAssemblyService.Conclusions` (or passed directly to `ComposeTree`).
No change is required to composition, validation, gap-detection,
revisioning, telemetry, or persistence — the extension point was
designed in at this phase specifically so that plugging in real
synthesis later is additive, not invasive.

---

## Incremental Re-assembly

A live case accumulates evidence over time: a new document is ingested,
a new fact is constructed, or `packages/application` re-runs and
produces a new application. Rebuilding the whole tree from scratch on
every such event would be wasteful and would also re-validate parts of
the tree that never changed.

`ReassembleIncremental(prev, newFacts, newApplications)` instead:

1. Appends only the genuinely new nodes (already-present IDs are
   skipped, so it is safe to re-submit the same evidence).
2. Reconstructs only the new edges those nodes' provenance implies.
3. Runs `irac.ValidateTree` and `DetectGaps` over just the affected
   subset — the new nodes plus whatever existing nodes their new edges
   reference — rather than the whole tree.
4. Bumps the revision exactly once via `NextRevision`.

The full updated `Tree` returned is always structurally complete and
valid as a whole; only the *validation and gap-detection work* is scoped
to the delta, not the tree's contents.

---

## Persistence Model

`PersistTree` writes every node and edge in an assembled `Tree` into a
`graph.GraphStore` via `CreateNode`/`CreateEdge`. Both are treated as
idempotent upserts (per `packages/graph/store.go`'s documented
contract), so nodes already persisted by an upstream service
(`packages/issue`, `packages/fact`, `packages/application`) are safely
re-written rather than duplicated.

After writing nodes/edges, `PersistTree` calls `graph.Export` to produce
a lossless JSON snapshot of the whole case tree, and stores it in a
`SnapshotStore` keyed by `SnapshotKey{CaseID, RevisionNumber}` — never by
`CaseID` alone, since each assembly is an immutable, versioned snapshot
per `irac.TreeRevision`'s model, not an in-place mutation. A snapshot
round-trips cleanly through `graph.Import` into a fresh `GraphStore`.
