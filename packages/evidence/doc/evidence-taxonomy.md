# Verdex Evidence Classification Taxonomy

## Overview

`packages/evidence` tags each `segmentation.Segment` produced by
`packages/segmentation` with an evidentiary role and a party attribution, so
downstream IRAC (Issue/Rule/Application/Conclusion) reasoning can
distinguish testimony from documentary proof, statutory authority, and
argument — and can tell whose evidence it is looking at.

Like `packages/segmentation`'s splitting/heading/exhibit detection and
`packages/pii`'s rule-based detector, the default classifier in this
package is deterministic and lexical/pattern-based. No component depends on
a machine-learning model at runtime — but classification is designed as a
pluggable extension point so a real classifier model can be swapped in
later without touching any caller.

---

## The EvidenceType Taxonomy

| Type                      | Meaning                                                                                          |
| -------------------------- | -------------------------------------------------------------------------------------------------- |
| `TypeWitnessStatement`    | First-person testimonial language: sworn statements, depositions, and speaker-attributed accounts. |
| `TypeDocumentaryEvidence` | References to documents, exhibits, or records introduced as proof.                                 |
| `TypeStatutoryCitation`   | Statute, code section, or case-law citations invoked as legal authority.                            |
| `TypeArgument`            | Advocacy or reasoning text: a party's or counsel's contention or submission.                        |
| `TypePhysicalExhibit`     | References to tangible, non-documentary physical evidence (a weapon, a sample, a garment).          |
| `TypeOther`               | Text that does not fit a more specific evidence type.                                               |

`Describe(EvidenceType)` returns the canonical description for each
constant, and `AllEvidenceTypes()` enumerates every recognized value, so
this table and the source code cannot drift.

---

## Classification: A Pluggable Interface

```go
type Classifier interface {
    Classify(ctx context.Context, seg segmentation.Segment) (Classification, error)
}
```

`RuleBasedClassifier` is the default implementation. It checks, in priority
order:

1. **Statutory citation** — a `segmentation.SegmentCitation` segment, or
   text matching a statute/case-law citation shape (`Section 302 IPC`,
   `42 U.S.C. § 1983`, `Smith v. Jones`, `(2020) 3 SCC 45`).
2. **Witness statement** — a `segmentation.SegmentStatement` segment
   (speaker-attributed, per `packages/segmentation`'s diarization support),
   or text containing first-person testimonial language (`I saw`,
   `I testify`, `to the best of my knowledge`) or an explicit
   deposition/affidavit marker.
3. **Physical exhibit** — text referencing tangible, non-documentary
   physical evidence (a weapon, a garment, a DNA/blood sample).
4. **Documentary evidence** — a `segmentation.SegmentExhibit` segment, or
   text referencing a document by name with a dating/attachment marker
   (`the contract dated ... was attached`).
5. **Argument** — text containing advocacy/reasoning markers (`we submit`,
   `it is argued`, `counsel contends`).
6. **Other** — the fallback when nothing above matches.

Citation is checked first because citation shapes are the most lexically
specific pattern and rarely overlap ambiguously with the looser
witness/documentary heuristics.

Each of these heuristics is exposed as its own function
(`IsStatutoryCitation`, `IsWitnessStatement`, `IsPhysicalExhibit`,
`IsDocumentaryEvidence`) so they can be tested, reused, or recomposed
independently of `RuleBasedClassifier`.

---

## Party Attribution

```go
type PartyRole string

const (
    PartyFirst        PartyRole = "first_party"
    PartySecond       PartyRole = "second_party"
    PartyUnattributed PartyRole = "unattributed"
)
```

`AttributeParty` determines which side a segment belongs to, checking in
order:

1. The segment's `SpeakerLabel` (set by `packages/segmentation`'s speaker
   attribution) for a first-party marker (`plaintiff`, `prosecution`,
   `petitioner`, `appellant`, `complainant`, `claimant`) or second-party
   marker (`defendant`, `respondent`, `appellee`, `accused`).
2. An explicit textual marker within the segment's text (`on behalf of the
   respondent`, `counsel for the petitioner`).
3. The segment's raw text for the same marker words, as a last resort.

A segment with no matching signal is `PartyUnattributed` rather than
guessed.

---

## Confidence

Every `Classification` carries a `Confidence` score in the closed interval
`[0, 1]`. A structurally boundary-tagged segment (already a
`SegmentExhibit` or `SegmentCitation`, produced by `packages/segmentation`)
scores higher than a bare lexical match on an otherwise-untagged
`SegmentParagraph`, since structural tagging is a stronger signal than
lexical pattern matching alone.

---

## Manual Override

```go
type ManualOverride struct {
    SegmentID  string
    Type       EvidenceType
    Party      PartyRole
    Reason     string
    ReviewedBy string
    ReviewedAt time.Time
    Previous   *Classification
}

func ApplyOverride(original Classification, override ManualOverride) (Classification, error)
```

A human reviewer's correction always takes precedence over the automated
classifier. `ApplyOverride` does not mutate `original` in place: it returns
a new `Classification` with `Confidence` set to `1.0` and an `Override`
field that records the correction *and* a snapshot of the classifier's
original determination (`Override.Previous`) — so the automated result is
never silently discarded, only superseded, and both remain inspectable for
audit.

`ApplyOverride` returns `ErrInvalidOverride` if the override has an empty
`SegmentID`, an unrecognized `EvidenceType`/`PartyRole`, or targets a
different segment than `original`.

---

## Persistence

```go
type ClassificationStore interface {
    Save(ctx context.Context, c Classification) error
    Get(ctx context.Context, segmentID string) (Classification, error)
    List(ctx context.Context) ([]Classification, error)
    Delete(ctx context.Context, segmentID string) error
}
```

`InMemoryClassificationStore` is the default implementation: a
mutex-guarded in-memory map keyed by segment ID, mirroring
`packages/jurisdiction`'s `Repository` pattern. No real database dependency
is required; a future phase can add a Postgres-backed implementation
without changing `EvidenceService` or any caller.

---

## The Pipeline: EvidenceService

```go
svc := evidence.NewEvidenceService()
results, err := svc.ClassifySegments(ctx, evidence.ClassifyRequest{
    Segments:  segments,
    Overrides: overrides, // map[segmentID]ManualOverride, optional
})
```

`EvidenceService.ClassifySegments` runs, for every segment: classify (which
internally detects the witness/documentary/statutory subtype and attributes
party) → apply any matching `ManualOverride` → persist via
`ClassificationStore` → collect the result. Segments with empty
(whitespace-only) text are skipped rather than aborting the whole batch;
any other error aborts the batch immediately.

This mirrors `packages/segmentation`'s `SegmentationService` and
`packages/pii`'s `PIIService` orchestration pattern: a single entry point
wiring together this package's otherwise independent, individually testable
building blocks.
