# Verdex Party & Timeline Model

## Overview

`packages/timeline` models the two parties in a case (their roles,
names, and counsel), attributes factual assertions to whichever party made
them, extracts a chronological timeline of events from segment text, and
flags conflicts where the two parties assert incompatible claims about the
same subject — so downstream IRAC (Issue/Rule/Application/Conclusion)
reasoning has a structured "who said what happened, and when" view of the
case.

Like `packages/evidence`'s classification and `packages/category`'s
suggestion engine, every heuristic in this package (date extraction,
conflict detection) is deterministic and lexical/pattern-based. No
component depends on a machine-learning model at runtime.

---

## Parties

```go
type PartyRole string

const (
    PartyFirst  PartyRole = "first_party"
    PartySecond PartyRole = "second_party"
    PartyThird  PartyRole = "third_party"
)

type Party struct {
    ID      string
    Role    PartyRole
    Name    string
    Counsel *string
}
```

`PartyRole` is a local equivalent of `packages/evidence.PartyRole` rather
than a hard cross-module dependency: `packages/evidence`'s `PartyRole`
answers "which side is this *segment* attributed to?" via a per-segment
attribution heuristic, while `packages/timeline`'s `Party` is a standalone
case participant record — role, display name, and optional counsel of
record — that other entities in this package (`PartyFact`, `Claim`,
`Relationship`) reference by `ID`. `Party.Validate()` rejects a party with
an empty `ID`/`Name` or an unrecognized `Role`.

---

## Fact Attribution

```go
type PartyFact struct {
    ID        string
    PartyID   string
    SegmentID string
    Text      string
    Span      segmentation.SourceSpan
    Subject   string
}
```

A `PartyFact` links a `Party.ID` to a factual assertion found in a single
`segmentation.Segment`, carrying the segment's `SourceSpan` forward so the
fact can be traced back to its exact position in the original source.
`Subject` is a short, normalized token (e.g. `"rent-payment"`, `"notice"`)
used by conflict detection (below) to find facts from different parties
that address the same underlying topic. `NewPartyFact` constructs one
directly from a `segmentation.Segment`.

---

## Events and Date Extraction

```go
type Event struct {
    ID          string
    Description string
    OccurredAt  *time.Time // nil = unknown/unresolved date
    Confidence  float64
    SegmentID   string
    PartyID     string
}

func ExtractDate(text string) (t time.Time, confidence float64, ok bool)
func ExtractEvent(id string, seg segmentation.Segment, partyID string) Event
```

`OccurredAt` is deliberately nullable: many events in a legal record have
approximate or entirely unknown dates, and forcing a concrete date onto
every event would fabricate precision the source doesn't support.
`ExtractDate` is a deterministic, regex-based extractor checked in
priority order:

1. **ISO dates** (`2024-03-15`) — confidence `0.95`, the least ambiguous
   shape.
2. **Long-form dates** (`March 15, 2024` / `March 15 2024`) — confidence
   `0.9`, a spelled-out month name is a strong, low-ambiguity signal.
3. **Slash dates** (`03/15/2024`) — confidence `0.6`, scored lower because
   month/day order is ambiguous across locales.

Every candidate date is validated to reject out-of-range values (month 13,
day 32) that `time.Date` would otherwise silently normalize into a
different calendar date.

---

## Timeline Assembly

```go
type Timeline struct {
    CaseID string
    Events []Event
}

func AssembleTimeline(caseID string, events []Event) Timeline
```

`AssembleTimeline` orders events into two groups: **dated events**,
sorted ascending by `OccurredAt`, followed by **undated events**, grouped
at the end. Both groups use a stable sort, so events that tie (the same
date, or all being undated) retain their original input order — repeated
assembly of the same input is fully deterministic. `Timeline.DatedEvents()`
and `Timeline.UndatedEvents()` return each subset directly.

---

## Conflict Detection

```go
type Conflict struct {
    ID      string
    FactAID string
    FactBID string
    Subject string
    Reason  string
}

func DetectConflicts(facts []PartyFact, sameOrOverlappingDate func(a, b PartyFact) bool) []Conflict
```

`DetectConflicts` is a **starting heuristic**, not a semantically-grounded
contradiction detector. It scans `PartyFact` pairs and flags a `Conflict`
when:

1. The two facts come from **different parties** (`PartyID` differs).
2. They share the same non-empty `Subject`.
3. (Optionally) their underlying events fall on the same date, via the
   caller-supplied `sameOrOverlappingDate` gate — `EventsSameOrOverlappingDate`
   builds one from a `segmentID -> *time.Time` map.
4. Their `Text` contains a **contradictory keyword pair** — e.g. `"did
   not"` vs `"did"`, `"denied"` vs `"admitted"`, `"breached"` vs
   `"complied"` — from a small, fixed list in `conflict.go`.

This lexical approach will produce both false positives and false
negatives; it is meant to surface *candidates* for human review, not to
adjudicate which party's account is correct. A future phase could replace
or augment it with a more sophisticated (still pluggable) detector without
changing `DetectConflicts`'s callers, the same way `packages/evidence`'s
`Classifier` and `packages/category`'s `Suggester` are pluggable
extension points.

---

## Claims

```go
type Claim struct {
    ID          string
    PartyID     string
    Description string
    EventIDs    []string
    FactIDs     []string
}

func ValidateClaimLinkage(claim Claim, knownEventIDs, knownFactIDs map[string]bool) error
```

A `Claim` links a `Party` to one or more `Event`/`PartyFact` entries it
relies on to support an assertion. `Claim.Validate()` checks basic shape
(non-empty `ID`/`PartyID`/`Description`, at least one supporting
reference); `ValidateClaimLinkage` is a separate integrity check that
verifies every referenced `EventIDs`/`FactIDs` entry resolves against a
store's actual persisted records, so a `Claim` can never dangle a
reference to an event or fact that doesn't exist.

---

## Relationships

```go
type Relationship struct {
    ID          string
    PartyAID    string
    PartyBID    string
    Kind        string
    Description string
}
```

`Relationship` describes how two parties relate — `KindLandlordTenant`,
`KindEmployerEmployee`, `KindContractual`, `KindFamilial`, `KindNeighbor`,
or any free-form `Kind` string a case calls for. This is independent of
the case's legal category (see `packages/category`): the *relationship*
between the parties and the *legal category* of the dispute between them
are separate axes — a landlord-tenant relationship might underlie a civil
or a criminal case, for instance.

---

## Persistence: CaseGraph and TimelineStore

```go
type CaseGraph struct {
    CaseID        string
    Parties       []Party
    Facts         []PartyFact
    Events        []Event
    Claims        []Claim
    Conflicts     []Conflict
    Relationships []Relationship
}

type TimelineStore interface {
    SaveGraph(ctx context.Context, graph CaseGraph) error
    GetGraph(ctx context.Context, caseID string) (CaseGraph, error)
    DeleteGraph(ctx context.Context, caseID string) error
    ListCaseIDs(ctx context.Context) ([]string, error)
}
```

`TimelineStore` mirrors `packages/evidence/store.go`'s
`ClassificationStore` pattern: a small, storage-agnostic contract that a
relational, document, or (as implemented here, `InMemoryTimelineStore`)
in-memory backend can satisfy. Unlike `ClassificationStore` (keyed by
segment ID), `TimelineStore` is keyed by **case ID** and stores the case's
entire party/timeline graph — parties, facts, events, claims, conflicts,
and relationships — as a single unit, since these entities are only
meaningful together, scoped to one case.

---

## The Pipeline: TimelineService

```go
svc := timeline.NewTimelineService()
result, err := svc.BuildTimeline(ctx, timeline.BuildRequest{
    CaseID:  "case-123",
    Parties: []timeline.Party{ /* ... */ },
    Segments: []timeline.SegmentAttribution{
        {Segment: seg, PartyID: "p1", Subject: "rent-payment"},
        // ...
    },
    Relationships: []timeline.Relationship{ /* ... */ },
})
// result.Timeline, result.Graph
```

`TimelineService.BuildTimeline` runs, for a single case: extract an
`Event` (and, for attributed segments, a `PartyFact`) from every
`SegmentAttribution` → assemble the resulting events into a `Timeline` →
detect conflicts among the extracted facts, gated by shared event dates →
validate and attach any supplied `Claims`/`Relationships` → persist the
full `CaseGraph` via `TimelineStore` → return both the assembled
`Timeline` and the persisted `CaseGraph`.

Generated `Event`/`PartyFact` IDs follow a deterministic scheme
(`"<CaseID>-event-<n>"` / `"<CaseID>-fact-<n>"`, keyed by the segment's
index within the request), so repeated calls with the same input produce
the same IDs — useful both for test assertions and for a caller
constructing `Claims` that reference those IDs in a two-pass workflow.

This mirrors `packages/evidence`'s `EvidenceService` and
`packages/category`'s `CategoryService` orchestration pattern: a single
entry point wiring together this package's otherwise independent,
individually testable building blocks.
