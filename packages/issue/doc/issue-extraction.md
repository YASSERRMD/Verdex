# Verdex Issue Extraction

## Overview

`packages/issue` derives legal issues from case facts and claims, producing
`irac.IssueNode`s (see `packages/irac/node.go`) that seed the Issue
position of a case's Issue-Rule-Application-Conclusion (IRAC) reasoning
tree.

Like `packages/evidence`'s classifier and `packages/segmentation`'s
splitting/heading/exhibit detection, the default implementation in this
package is deterministic and lexical/pattern-based. No component depends
on a machine-learning model at runtime — but every stage of the pipeline
is a pluggable extension point so a real model can be swapped in later
without touching any caller.

---

## The Pipeline

```
identify -> map claims -> dedup/merge -> decompose sub-issues
  -> link parties/facts -> score confidence -> apply any override
  -> persist -> return []irac.IssueNode
```

`IssueExtractionService.ExtractIssues` runs the full pipeline over a batch
of `segmentation.Segment`s (plus optional `evidence.Classification`s and
`timeline.Party`s) and returns the resulting `irac.IssueNode`s, already
persisted via a `graph.GraphStore`.

```go
svc := issue.NewIssueExtractionService()
nodes, err := svc.ExtractIssues(ctx, issue.ExtractRequest{
    CaseID:          "case-1",
    Segments:        segments,
    Classifications: classifications, // optional
    Parties:         parties,         // optional
    Overrides:       overrides,       // optional, map[issueID]ManualOverride
})
```

This mirrors `packages/evidence`'s `EvidenceService` and
`packages/segmentation`'s `SegmentationService` orchestration pattern: a
single entry point wiring together this package's otherwise independent,
individually testable building blocks.

---

## 1. Identification

```go
type IssueIdentifier interface {
    Identify(ctx context.Context, segments []segmentation.Segment) ([]CandidateIssue, error)
}
```

`RuleBasedIdentifier` is the default implementation. It scans each
segment's text for dispute/question-indicating language patterns —
`whether`, `dispute`/`disputed`, `claims that`, `denies`/`denied`,
`alleges`/`alleged`, `contends`/`contested`, and trailing `?` — assigning a
confidence per matched pattern (`whether` scores highest, at 0.85, since
it is the most specific marker of a posed legal question).

Separately, `RuleBasedIdentifier` scans adjacent
`segmentation.SegmentStatement` segments attributed to different speakers
for contradictory statement pairs (e.g. one party's "I received the
deposit" against another's "I did not receive any deposit"), producing one
`CandidateIssue` per contradictory pair with `SourceSpans` covering both
segments.

Each match becomes a `CandidateIssue`:

```go
type CandidateIssue struct {
    ID             string
    Text           string
    SourceSpans    []irac.SourceSpan
    Confidence     float64
    ParentIssueID  *string
}
```

---

## 2. Claim Mapping

`MapClaimsToIssues` matches `evidence.Classification` entries of type
`TypeArgument` (advocacy/contention text) or `TypeWitnessStatement`
(first-person testimony) — the two evidentiary roles most likely to state
or bear on a disputed issue — against candidate issues via a keyword-
overlap heuristic (the fraction of the classification's segment-text
tokens that also appear in the issue's text). Overlap at or above `0.2`
produces a `ClaimLink`.

---

## 3. Deduplication and Merging

`Dedup` merges near-duplicate `CandidateIssue`s using Jaccard similarity
(`|A ∩ B| / |A ∪ B|`) over each issue's normalized, stopword-filtered word
set. Two issues at or above `0.6` similarity are merged into one, keeping:

- the earlier-seen candidate's `ID` and `Text` as canonical
- the **union** of both candidates' `SourceSpans`
- the **maximum** of both candidates' `Confidence`

Merging is transitive within a single pass: an issue merged into an
earlier survivor is not itself compared against later candidates.

---

## 4. Sub-Issue Decomposition

`Decompose` splits a compound issue's text on an `and` conjunction joining
two distinct legal questions (e.g. "whether the contract was breached and
whether damages are owed") into a parent plus sub-issues. Each fragment
must have at least 3 words to be treated as its own legal question,
preventing a trailing clause fragment (e.g. "...and fees") from being
mistakenly split out.

Sub-issues carry `ParentIssueID` pointing back to the parent's `ID`, so a
consumer of the reasoning tree can reconstruct the parent/child
relationship without re-parsing issue text.

---

## 5. Party and Fact Linkage

```go
type IssueLink struct {
    IssueIndex int
    PartyIDs   []string
    FactIDs    []string
}
```

`LinkIssues` associates each candidate issue with:

- the `timeline.Party` IDs it mentions by name (case-insensitive
  substring match, or token overlap at or above `0.15`)
- the related fact/segment IDs whose text overlaps the issue's text at or
  above the same `0.15` threshold

---

## 6. Confidence Scoring

`ScoreConfidence` aggregates three independent signals into a single,
normalized `Confidence` in `[0, 1]`:

| Signal                | Weight | Source                                          |
| ---------------------- | ------ | ------------------------------------------------ |
| Identification         | 0.6    | The candidate's own confidence from `Identify`/`Dedup` |
| Claim support           | 0.25   | The best `ClaimLink.Overlap` referencing this issue |
| Dedup corroboration     | 0.15   | Whether the issue carries more than one `SourceSpan` (i.e. was merged from independently identified candidates) |

Weights favor identification as the strongest signal while letting
corroborating signals only ever raise the score — an issue with claim
support or multi-span corroboration is, if anything, more likely to be a
genuine issue, never less.

---

## 7. Human Review and Override

```go
type ManualOverride struct {
    IssueID    string
    Text       string
    Material   bool
    Reason     string
    ReviewedBy string
    ReviewedAt time.Time
    Previous   *CandidateIssue
}

func ApplyOverride(original CandidateIssue, override ManualOverride) (OverriddenIssue, error)
```

Mirroring `packages/evidence`'s `ManualOverride`/`ApplyOverride` pattern, a
human reviewer's correction always takes precedence over the automated
extraction. `ApplyOverride` does not mutate `original` in place: it
returns an `OverriddenIssue` whose `Text` reflects the correction and whose
`Confidence` is `1.0`, with an `Override` field that records the
correction *and* a snapshot of the extractor's original determination
(`Override.Previous`) — so the automated result is never silently
discarded, only superseded, and both remain inspectable for audit.

`ApplyOverride` returns `ErrInvalidOverride` if the override has an empty
`IssueID`, blank `Text`, or targets a different issue than `original`.

---

## 8. Persistence

```go
func ToIssueNode(candidate CandidateIssue, caseID string, createdAt time.Time, upstreamNodeIDs []string) irac.IssueNode
func PersistIssues(ctx context.Context, store graph.GraphStore, issues []CandidateIssue, caseID string, createdAt time.Time, linksByIndex map[int]IssueLink) ([]irac.IssueNode, error)
```

`ToIssueNode` converts a `CandidateIssue` into an `irac.IssueNode` via
`irac.NewIssueNode`, stamping a `Provenance` that identifies this
package's extraction pipeline (`verdex-issue-extractor-v1`) as the
generating process, with `Provenance.UpstreamNodeIDs` carrying forward any
linked fact/segment IDs from `IssueLink.FactIDs`.

`PersistIssues` persists every converted node via `graph.GraphStore.
CreateNode`. Note that `packages/irac`'s edge-constraint table
(`legalEdgeTriples`, `packages/irac/edge.go`) has no legal edge whose
source and target are both `NodeIssue`, and this pipeline produces no
`RuleNode`s — so `PersistIssues` persists nodes only. The
`Rule --governs--> Issue` edge is created once a later phase produces
`RuleNode`s that reference these issues.

---

## Design Principles

- **No ML models.** Every heuristic in this package is a deterministic
  function of `segmentation.SegmentType` and regular-expression/lexical
  pattern matching. A future phase can swap in a real identification
  model by implementing `IssueIdentifier`; no caller needs to change.
- **Confidence, not certainty.** Every `CandidateIssue` carries a
  `Confidence` score in `[0, 1]`, aggregated across every pipeline stage
  that produces a signal about how likely the candidate is to be a
  genuine issue.
- **Human correction is first-class, not a patch.** `ManualOverride` and
  `ApplyOverride` are dedicated types, not a mutation of `CandidateIssue`
  in place — the extractor's original determination is always
  recoverable from `OverriddenIssue.Override.Previous`.
