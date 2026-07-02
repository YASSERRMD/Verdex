# Tree integrity guarantees (`packages/treevalidation`)

Phase 040 is the capstone validation gate for Verdex's IRAC reasoning
trees. This document explains exactly what `packages/treevalidation`
guarantees, and — importantly — what it does **not** re-derive, because
earlier phases already own that responsibility.

## Composes with, does not duplicate

Three earlier phases already implement pieces of tree validation. This
package builds on top of all three rather than reimplementing any of
them:

| Phase | Package | Owns |
|---|---|---|
| 031 | `packages/irac` (`validate.go`) | Structural integrity: dangling edge references, illegal `(FromNodeType, EdgeType, ToNodeType)` triples, self-loops, unknown node/edge types, missing `draft_analysis` guardrail labels. |
| 037 | `packages/application` (`chain.go`) | Local rule-chain cycle detection: a repeated rule ID within one flat `RuleChain`. |
| 039 | `packages/treeassembly` (`gap.go`, `integrity.go`) | Semantic gap detection (an issue with no application, an application with no conclusion) plus a thin wrapper (`ValidateIntegrity`) around `irac.ValidateTree`. |

`packages/treevalidation`'s `TreeValidationService.Validate` calls
`treeassembly.ValidateIntegrity` and `treeassembly.DetectGaps` directly
and folds their results into its own `Report` as `Finding`s. It never
re-checks dangling edges, illegal triples, self-loops, guardrail labels,
or issue/application gaps itself — that would duplicate logic that
already exists, tested, in earlier phases.

## What this phase adds

Six checks that no earlier phase performs:

1. **Conclusion traceability** (`traceability.go`,
   `CheckConclusionTraceability`) — every `ConclusionNode` must walk its
   edges (`Conclusion --concludes_from--> Application --applies_to-->
   Fact` and `Application --applies_to--> Rule`) to reach **at least one**
   `FactNode` **and** **at least one** `RuleNode`. This is stricter than
   "has some edge": a conclusion that concludes from an application which
   only ever applies a rule (never cites a supporting fact) passes orphan
   detection but fails traceability.

2. **True orphan detection** (`orphan.go`, `DetectOrphans`) — any node,
   of any `irac.NodeType`, with zero edges at all (neither incoming nor
   outgoing). This catches nodes wholly disconnected from the rest of the
   tree, e.g. a `FactNode` no application ever cites.

3. **Full-graph cycle detection** (`cycle.go`, `DetectCycles`) — a
   DFS-based cycle detection over every node and every edge in the
   assembled tree, regardless of type. `packages/application`'s
   `RuleChain.Validate` only ever sees one flat, local chain of rules; it
   cannot detect a cycle that spans multiple applications, facts, or
   rules stitched together during assembly. `DetectCycles` can.

4. **Unsupported-claim flagging** (`unsupported.go`,
   `FlagUnsupportedClaims`) — nodes with empty source-span backing
   (`Spans`), or a `Confidence` below a caller-supplied threshold, are
   flagged as claims the tree cannot adequately back up.

5. **Confidence-propagation checks** (`propagation.go`,
   `CheckConfidencePropagation`) — a `ConclusionNode`'s confidence must
   never exceed the minimum confidence found across its supporting chain
   (the application it concludes from, and every fact/rule that
   application applies). A conclusion cannot be more confident than its
   weakest input.

6. **Jurisdiction-consistency checks**
   (`jurisdiction_consistency.go`, `CheckJurisdictionConsistency`) —
   every `RuleNode.JurisdictionCode` in the tree must match the case's
   declared jurisdiction, unless explicitly allow-listed as cited
   persuasive/foreign authority. This prevents cross-jurisdiction
   leakage into a case's reasoning.

## The report and the gate

`report.go` defines `Finding` (with `Severity` one of
`SeverityCritical`, `SeverityWarning`, `SeverityInfo`, plus a `Code`,
`Message`, and `NodeID`) and `Report`, which aggregates every `Finding`
produced by every check above (both the composed checks and this
package's own six) plus a `Summary()` method for a concise, human-
readable rollup.

`gate.go`'s `CanFinalize(report Report) (bool, error)` is the hard,
blocking gate: it returns `false` and a wrapped `ErrCriticalFindings`
whenever `report` contains at least one `SeverityCritical` `Finding`.
Per `CONTRIBUTING.md`'s non-binding guardrail section — a tree with
critical integrity failures must not be usable for further reasoning —
every future phase that consumes an assembled `treeassembly.Tree` (most
notably Phase 055's synthesis) is expected to call `CanFinalize`
(directly, or transitively through `TreeValidationService.Validate`'s
returned error) before treating that tree as trustworthy.

`Report` findings that are only `SeverityWarning` or `SeverityInfo` (for
example, semantic gaps surfaced from `treeassembly.DetectGaps`, or
unsupported-claim warnings) do not block finalization on their own —
only critical findings do.

## Orchestration entry point

`service.go`'s `TreeValidationService.Validate(tree treeassembly.Tree)
(*Report, error)` is the single entry point most callers should use. It
runs, in order:

1. `treeassembly.ValidateIntegrity` (composed, not reimplemented);
2. `treeassembly.DetectGaps` (composed, not reimplemented);
3. `CheckConclusionTraceability`;
4. `DetectOrphans`;
5. `DetectCycles`;
6. `FlagUnsupportedClaims`;
7. `CheckConfidencePropagation`;
8. `CheckJurisdictionConsistency` (only when a case jurisdiction code is
   configured);
9. aggregates every result into a `*Report`;
10. applies `CanFinalize` and returns its error (`ErrCriticalFindings`)
    alongside the fully populated `*Report` so callers can inspect
    exactly what went wrong, even when the tree fails the gate.
