# Verdex Document Segmentation Model

## Overview

`packages/segmentation` splits normalized case text — from `packages/stt`
transcripts, `packages/ocr` extractions, or plain documents (including
`packages/multilingual`-normalized text) — into logical units (`Segment`s)
that later Verdex phases (evidence classification, party/timeline modeling,
IRAC extraction) operate on.

Like `packages/multilingual`'s tokenizer and script/language detection, and
`packages/stt`/`packages/ocr`'s pluggable-provider pattern, every detection
step in this package is deterministic and rule-based. No component uses a
machine-learning model.

---

## The Segment Entity

A `Segment` is the atomic unit of a segmented document:

```go
type Segment struct {
    ID           string
    Type         SegmentType
    Text         string
    Language     string
    SpeakerLabel SpeakerLabel
    Span         SourceSpan
    Sequence     int
    PrevID       string
    NextID       string
    Confidence   float64
}
```

`SegmentType` classifies the kind of logical unit:

| Type                | Meaning                                                          |
| -------------------- | ----------------------------------------------------------------- |
| `SegmentParagraph`   | Ordinary body text — the default unit from sentence splitting.  |
| `SegmentStatement`   | A speaker-attributed utterance from transcript input.            |
| `SegmentExhibit`     | Introduces or refers to an exhibit ("Exhibit A", "Ex. 3").       |
| `SegmentHeading`     | A heading or section title.                                     |
| `SegmentCitation`    | A statute or case-law citation.                                 |

---

## Pipeline Stages

`SegmentationService.Segment` orchestrates the full pipeline:

```
input (Text or STT-shaped Turns)
  │
  ▼
SplitSentences() / AttributeSpeakers()   ← split into raw Segments
  │
  ▼
TagHeadings()                             ← numbering / all-caps / title-case
  │
  ▼
TagExhibitsAndCitations()                 ← exhibit & citation markers
  │
  ▼
AssignDefaultConfidence()                 ← propagate or default to 1.0
  │
  ▼
assign IDs (IDGenerator)
  │
  ▼
AssignOrder()                             ← Sequence + PrevID/NextID
  │
  ▼
[]Segment
```

When `SegmentRequest.Turns` is supplied and at least one turn carries a
`Speaker` label, speaker-attributed segmentation (`AttributeSpeakers`) is
used instead of plain sentence splitting, and every resulting `Segment` is
tagged `SegmentStatement`. Otherwise `SplitSentences` produces
`SegmentParagraph` segments from `SegmentRequest.Text`.

Heading detection and exhibit/citation detection both run over every
segment regardless of origin, but never override a more specific
classification already assigned: `TagHeadings` only retags
`SegmentParagraph` segments, and `TagExhibitsAndCitations` only retags
`SegmentParagraph` or `SegmentStatement` segments (a `SegmentHeading` is
left untouched even if its text also matches an exhibit or citation
pattern).

---

## Sentence and Clause Splitting

`SplitSentences`/`SplitClauses` (`splitter.go`) are deterministic,
rule-based punctuation splitters:

- **Sentence terminators**: `.`, `!`, `?`, plus the Arabic full stop (`۔`)
  and Devanagari danda (`।`) used by some South Asian scripts.
- **Clause separators** (in addition to sentence terminators): `,`, `;`,
  `:`, and the Arabic comma/semicolon (`،`/`؛`). Every clause boundary
  produced by `SplitClauses` is at least as fine-grained as a sentence
  boundary — every sentence boundary is also a clause boundary.
- **Abbreviation awareness**: a curated list of common abbreviations (Mr.,
  Dr., Prof., etc., vs., Section/Sec., Article/Art., month abbreviations,
  U.S./U.K., ...) suppresses a false sentence boundary at the trailing
  period.
- **Closing-quote extension**: a terminator immediately followed by a
  closing quote or bracket (`"`, `'`, `”`, `’`, `)`, `]`) extends the
  boundary to include it, so `He said "guilty."` splits after the closing
  quote, not before it.

Both functions guarantee **full coverage**: the returned spans, taken in
order, cover the complete rune range of the input text with no gaps and no
overlaps. This is the same fidelity invariant `SourceSpan` enforces at the
`Segment` level (see below), and it is asserted directly in
`splitter_test.go`.

---

## Heading and Section Detection

`IsHeadingLine`/`TagHeadings` (`heading.go`) use purely structural
heuristics — no ML model:

1. **Numbering patterns**: lines beginning with `1.`, `1.1`, `I.`, `(a)`,
   `Section 3`, `Article 12`, `Chapter IV`, and similar.
2. **Short ALL-CAPS lines**: entirely upper-case (ignoring digits,
   punctuation, whitespace), at least one letter, at most 12 words, and not
   ending in a sentence terminator — e.g. `STATEMENT OF FACTS`.
3. **Short Title-Case lines**: every word capitalized (allowing common
   lower-case connectors — "of", "the", "and", "a", "an", "in", "on",
   "for", "to", "or"), at most 12 words, no trailing sentence terminator —
   e.g. `Statement Of Facts`, `Order Of The Court`.

`TagHeadings` only retags segments still at the default `SegmentParagraph`
type, so a segment already classified as an exhibit or citation is never
downgraded to a heading.

---

## Speaker-Attributed Segmentation

`speaker.go` defines `TranscriptTurn`, an STT-shaped diarized turn
(`Speaker`, `Text`, `StartMS`, `EndMS`, `Confidence`) that mirrors
`packages/stt`'s `TranscriptSegment` without importing `packages/stt`
directly — callers adapt `stt.TranscriptSegment` to `TranscriptTurn` at the
call site, keeping this package decoupled from the STT provider stack.

`AttributeSpeakers` converts non-empty turns into `SegmentStatement`
segments, each carrying:

- `SpeakerLabel` — the turn's diarized speaker label (empty when no
  diarization hint was available for that turn).
- `Span.StartMS`/`Span.EndMS` — carried through directly from the turn.
- `Confidence` — the turn's upstream STT confidence.

`HasSpeakerHints` reports whether at least one turn carries a non-empty
speaker label; `SegmentationService.Segment` uses it to decide whether
speaker-attributed segmentation should run at all (a transcript with no
diarization is still segmented, but as plain paragraphs rather than
per-speaker statements, so timing metadata is never silently dropped).

---

## Exhibit and Citation Boundary Detection

`exhibit.go` detects two kinds of structural markers via regular
expressions:

- **Exhibit markers**: `Exhibit A`, `Exhibit 12`, `Ex. 3`, `Ex 3`,
  `Annexure B`, `Schedule 1` (case-insensitive).
- **Citation shapes**: U.S. Code citations (`42 U.S.C. § 1983`), section
  references (`Section 302 IPC`, `S. 420`), constitutional articles
  (`Article 21`), Indian law reporters (`(2020) 3 SCC 45`,
  `AIR 1978 SC 597`), and case names (`Smith v. Jones`).

`TagExhibitsAndCitations` reclassifies matching `SegmentParagraph`/
`SegmentStatement` segments to `SegmentExhibit`/`SegmentCitation`
(exhibit markers take precedence when a segment matches both patterns).
`SplitOnExhibitBoundaries` additionally splits raw text so an exhibit
reference always begins a new span rather than being buried mid-paragraph,
preserving the same full-coverage, no-gaps/no-overlaps guarantee as
`SplitSentences`/`SplitClauses`.

---

## Source-Span Offsets

`SourceSpan` (`span.go`) locates a `Segment`'s text within its original
source:

```go
type SourceSpan struct {
    Start       int // inclusive rune offset
    End         int // exclusive rune offset
    Page        int
    BoundingBox BoundingBox
    StartMS     int64
    EndMS       int64
}
```

`Start`/`End` are rune offsets into the normalized source text — always
populated. `Page`/`BoundingBox` carry through OCR origin metadata
(mirroring `packages/ocr`'s `TextBlock.Page`/`TextBlock.BoundingBox`);
`StartMS`/`EndMS` carry through STT origin metadata (mirroring
`packages/stt`'s `TranscriptSegment.StartMS`/`EndMS`). A `Segment` produced
from plain text populates only `Start`/`End`; one produced from a
transcript turn populates `StartMS`/`EndMS` (and, once placed in the
service's synthetic joined source text, `Start`/`End` as well); one carried
through from an OCR pipeline would additionally populate `Page`/
`BoundingBox`.

`ValidateSpanCoverage` is the enforcement mechanism for this package's core
fidelity guarantee: given a slice of spans in document order and the total
rune length of the source, it verifies the first span starts at 0, each
span's `End` equals the next span's `Start` (no gaps, no overlaps), and the
last span's `End` equals the total length. It returns `ErrSpanOverlap` on
any violation.

---

## Ordering and Linkage

`AssignOrder` (`order.go`) assigns a stable, zero-based `Sequence` to each
segment in the order given, and populates `PrevID`/`NextID` from each
segment's immediate neighbors (empty at the first/last segment
respectively). `ValidateOrder` checks the resulting invariant: `Sequence`
values are exactly `0, 1, 2, ...` in slice order, and every `PrevID`/
`NextID` is internally consistent with the neighboring segment's `ID`.

---

## Confidence

`Segment.Confidence` is a score in the closed interval `[0, 1]`.
`AssignDefaultConfidence` (`confidence.go`) sets `Confidence` to
`DefaultConfidence` (`1.0`) on any segment whose `Confidence` is still the
zero value — distinguishing "no upstream signal" (plain text) from a
genuine zero-confidence OCR/STT result — and clamps any already-set value
into `[0, 1]`. `AggregateConfidence` combines multiple upstream per-unit
confidences (e.g. several OCR `TextBlock`s or STT `TranscriptSegment`s
assembled into one `Segment`) into a single score via unweighted arithmetic
mean, mirroring `packages/ocr`'s `AverageConfidence`. `LowConfidenceSegments`
mirrors `packages/ocr`'s `LowConfidenceBlocks` for flagging segments that
may need human review.

---

## Testing Strategy

- `splitter_test.go` — sentence/clause splitting: abbreviation suppression,
  question/exclamation boundaries, closing-quote handling, empty/
  whitespace input, and full-coverage fidelity for every case.
- `heading_test.go` — numbering/roman-numeral/lettered/keyword patterns,
  all-caps and title-case short lines, negative cases (plain sentences,
  long all-caps blocks), and non-mutation of `TagHeadings`' input.
- `speaker_test.go` — `AttributeSpeakers` round-trips speaker label, text,
  confidence, and timing for every non-empty turn; whitespace-only turns
  are skipped; `HasSpeakerHints` across hinted/unhinted/mixed/empty sets.
- `exhibit_test.go` — exhibit/citation marker detection across all
  supported shapes, tagging precedence (headings are never reclassified),
  and `SplitOnExhibitBoundaries`' full-coverage invariant.
- `span_test.go` — `SourceSpan.Len`/`Overlaps` and `ValidateSpanCoverage`
  across full-coverage, gap, overlap, and boundary-misalignment cases.
- `order_test.go` — `AssignOrder`'s `Sequence`/`PrevID`/`NextID` assignment
  (including single-segment and empty cases, and non-mutation of input) and
  `ValidateOrder`'s detection of sequence gaps and broken linkage.
- `service_test.go` — end-to-end `SegmentationService.Segment` coverage:
  plain-text pipeline (heading/exhibit/citation tagging, default
  confidence, valid ordering, unique IDs), the speaker-turn pipeline
  (confidence propagated from STT, non-overlapping spans), empty-input
  rejection, and a pluggable `IDGenerator`.

No test depends on network access or an external service; every detector
and splitter in this package is a deterministic, self-contained function of
its input text.
