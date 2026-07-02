# Verdex Fact Model

## Overview

`packages/fact` builds `irac.FactNode`s (see `packages/irac/node.go`) from
classified case segments (`packages/evidence.Classification`), enriching
each raw fact with evidence backing, party attribution, dispute status,
temporal anchoring, corroboration links, and a reliability score, before
persisting it into the case's IRAC reasoning tree.

Like `packages/evidence`'s classifier, `packages/timeline`'s conflict
detector, and `packages/issue`'s extraction pipeline, every heuristic in
this package is deterministic and lexical/pattern-based. No component
depends on a machine-learning model at runtime — every stage is a
pluggable extension point.

---

## The Pipeline

```
build -> attach evidence ref -> attribute party -> flag dispute
  -> anchor temporally -> link corroboration -> score reliability
  -> persist -> return []irac.FactNode
```

`FactConstructionService.ConstructFacts` runs the full pipeline over a
batch of classified segments (plus optional `timeline.Party`s,
`timeline.Event`s, and existing `irac.ApplicationNode` IDs) and returns
the resulting `irac.FactNode`s, already persisted via a `graph.GraphStore`.
`ConstructFactsDetailed` returns the same nodes bundled with every
intermediate signal the pipeline derived (`FactDetail`).

```go
svc := fact.NewFactConstructionService()
nodes, err := svc.ConstructFacts(ctx, fact.ConstructRequest{
    CaseID:   "case-1",
    Segments: segments, // []fact.SegmentInput: classification + text + span
    Parties:  parties,  // optional
    Events:   events,   // optional
})
```

This mirrors `packages/issue`'s `IssueExtractionService` orchestration
pattern: a single entry point wiring together this package's otherwise
independent, individually testable building blocks.

---

## 1. Build

```go
func BuildFactNode(classification evidence.Classification, segmentText string, span SourceSpan, id, caseID string, createdAt time.Time) (irac.FactNode, error)
```

`evidence.Classification` carries only a `SegmentID`, not the segment's
own text or source span — so `BuildFactNode` takes both explicitly,
mirroring `packages/issue`'s `segmentText map[string]string` bridging
convention (see `packages/issue/claim_map.go`). The resulting
`irac.FactNode`'s `Text` is the segment's text, its `Spans` trace back to
the segment's source location, and its `Confidence` starts from the
classification's own confidence.

## 2. Evidence reference

```go
type EvidenceRef struct {
    FactID, SegmentID, ClassificationID string
    EvidenceType evidence.EvidenceType
    PartyRole    evidence.PartyRole
    Confidence   float64
}
```

Every fact traces to the evidentiary basis it was built from: testimony
(`TypeWitnessStatement`), a documentary or physical exhibit, a statutory
citation, or argument text. `EvidenceRef.IsTestimonial()` and
`IsExhibit()` are convenience predicates over the common cases.

## 3. Party attribution

```go
func AttributeParty(factID string, partyRole evidence.PartyRole, parties []timeline.Party) PartyAttribution
```

Maps the originating classification's `evidence.PartyRole` (first/
second/unattributed) to its `timeline.PartyRole` equivalent, then
resolves the specific `timeline.Party.ID` in the case roster whose `Role`
matches. `evidence.PartyUnattributed` has no `timeline.PartyRole`
equivalent, so it resolves to an empty `PartyAttribution`.

## 4. Dispute flagging

```go
type DisputeStatus string // Undisputed | Disputed | Unknown

func DetermineDisputeStatus(candidate FactWithParty, peers []FactWithParty) (DisputeStatus, string)
```

A fact is `Disputed` when a peer fact attributed to a *different* party
contains a contradictory keyword pair (e.g. "did not pay" vs. "paid",
"denied" vs. "admitted"). This re-implements, locally and independently,
the contradiction-heuristic idea from `packages/timeline/conflict.go`'s
`contradictionPairs` — deliberately without depending on that file's
`PartyFact`/`DetectConflicts` machinery, since this package compares
`irac.FactNode` text and party attribution directly. `Unknown` is
returned when there is no party attribution or no peers to compare
against.

## 5. Temporal anchoring

```go
func AnchorToEvent(factID, factText, segmentID string, events []timeline.Event) TemporalAnchor
```

Links a fact to a `timeline.Event` by shared originating `SegmentID`
(the strongest signal — the fact and event share a common source
segment) or, failing that, by description-text token overlap. The
resulting `TemporalAnchor.OccurredAt` mirrors `timeline.Event.OccurredAt`'s
"nil means unknown" convention.

## 6. Corroboration linkage

```go
func DetectCorroboration(candidates []CorroborationCandidate) []CorroborationLink
```

Flags a `CorroborationLink` between two fact nodes attributed to
different, non-empty parties whose text has sufficient symmetric
(Jaccard) token overlap — independent corroboration is most meaningful
when it comes from more than one source. Facts with no party attribution
are still compared against everything else, since excluding them would
silently drop legitimate corroboration signal.

## 7. Reliability scoring

```go
func ReliabilityScore(input ReliabilityInput) float64 // in [0, 1]
```

Blends classification confidence (weight 0.5), corroboration count
(weight 0.3, saturating at 3+ corroborating facts), and dispute status
(weight 0.2 — full credit for `Undisputed`, half for `Unknown`, none for
`Disputed`) into a single signal, deliberately separate from the node's
raw `irac.Node.Confidence`. The score is monotonic: more corroboration
never lowers it, and `Disputed` never scores higher than `Undisputed` or
`Unknown`, holding the other inputs fixed.

## 8. Persistence

```go
func PersistFacts(ctx context.Context, store graph.GraphStore, facts []irac.FactNode, applicationIDs []string, supportsApplicationIDs map[string][]string) ([]irac.FactNode, error)
```

Persists every fact node via `store.CreateNode`, then creates
`Fact--supports-->Application` edges — the only legal edge triple with a
`FactNode` source in `packages/irac/edge.go`'s constraint table — linking
facts to any `irac.ApplicationNode`s that already exist for the case.
Edges to an `ApplicationNode` ID not present in `applicationIDs` are
skipped (the fact node itself is still persisted), since creating an edge
to a nonexistent node would violate `GraphStore.CreateEdge`'s
dangling-reference expectations.

---

## Design Principles

- **No ML models.** Every heuristic (dispute detection, corroboration
  detection, temporal anchoring's text-overlap fallback) is a
  deterministic function of lexical/token-overlap pattern matching,
  mirroring `packages/evidence`, `packages/timeline`, and
  `packages/issue`'s shared "no ML models, rule based" design principle.
- **Reliability is distinct from confidence.** A fact node's raw
  `irac.Node.Confidence` reflects only extraction/classification
  confidence. `ReliabilityScore` is a separate signal folding in
  corroboration and dispute status, exposed via `FactDetail` alongside
  (never in place of) the raw `Confidence`.
- **No hard dependency on `packages/timeline/conflict.go`'s internals.**
  Dispute detection re-implements the contradiction-keyword heuristic
  locally.
- **No edges without both endpoints.** `Fact--supports-->Application`
  edges are only created when the target `ApplicationNode` already
  exists in the store for the case; `PersistFacts` never creates
  `ApplicationNode`s itself.
