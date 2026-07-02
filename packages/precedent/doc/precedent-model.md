# Verdex Precedent Model

## Overview

`packages/precedent` loads prior cases/precedents for common-law
jurisdictions as structured `irac.RuleNode`s (see
`packages/irac/node.go`'s `RuleNode` doc comment: "a legal rule, statute,
or precedent invoked to resolve an Issue"). The fixed IRAC schema
(Issue/Rule/Fact/Application/Conclusion) was established in Phase 031 and
is never modified afterward — there is no separate `PrecedentNode` type.
Precedents are represented as `irac.RuleNode`s via `irac.NewRuleNode(...)`,
exactly the way `packages/statute` (Phase 035) represents statutes.

All precedent-specific metadata that `RuleNode` does not carry — the
extracted holding, ratio decidendi, formatted citation, court-hierarchy
classification, and recency/authority score — lives in this package's
local `PrecedentRule` wrapper struct and its sibling pipeline-stage
structs (`TaggedPrecedent`, `HierarchyRule`, `EmbeddedPrecedent`,
`ScoredPrecedent`), alongside the `RuleNode` they wrap.

Like `packages/statute`'s ingestion pipeline, every heuristic in this
package (holding/ratio extraction, court classification, keyword
extraction) is deterministic and lexical/pattern-based — no ML model at
runtime, except for the embedding step, which is delegated entirely to
the existing `embedding.EmbeddingService`.

---

## The Pipeline

```
load -> extract holding/ratio -> build rule nodes with citations -> tag
  -> weight by court hierarchy -> embed -> score authority -> persist
  -> return []PrecedentRule
```

`PrecedentIngestionService.Ingest` runs the full pipeline over a raw
corpus source and a target jurisdiction, and returns the resulting
`PrecedentRule`s, already persisted via a `graph.GraphStore`.
`IngestDetailed` returns the same rules bundled with every intermediate
signal the pipeline derived (`IngestResult`).

```go
svc := precedent.NewPrecedentIngestionService()
svc.Embedding = myEmbeddingService // optional; nil skips embedding

rules, err := svc.Ingest(ctx, precedent.IngestRequest{
    Source:           corpusReader,
    JurisdictionCode: "UK",
    LegalFamily:      "common_law",
    CategoryCode:     "tort",
})
```

This mirrors `packages/statute`'s `StatuteIngestionService` orchestration
pattern: a single entry point wiring together this package's otherwise
independent, individually testable building blocks.

---

## 1. Load

```go
type Loader interface {
    Load(ctx context.Context, source io.Reader) ([]RawPrecedent, error)
}
```

`DefaultLoader` is a pure parser — no network fetch. It auto-detects two
input shapes:

- **JSON**: a top-level array of `RawPrecedent` objects, or an object with
  a top-level `"precedents"` array field.
- **Structured text**: a line-oriented format where each case begins with
  `CASE <citation>: <case name>`, optionally followed by `COURT: <court>`
  and `DECIDED: <YYYY-MM-DD>` lines, with every following line up to the
  next `CASE` header (or end of input) forming that case's `FullText`.

`RawPrecedent.FullText` is left unparsed — holding/ratio extraction is a
separate stage.

---

## 2. Extract Holding and Ratio Decidendi

```go
func ExtractHoldingAndRatio(fullText string) (HoldingExtractionResult, error)
```

`ExtractHoldingAndRatio` is a deterministic heuristic extractor. It
locates a `HELD:`/`HOLDING:` marker line and captures text up to the next
recognized section marker (or end of input) as the `Holding`. A separate
`RATIO:`/`RATIO DECIDENDI:`/`REASONING:` marker supplies `RatioDecidendi`
when present; otherwise the holding section is split at its first
sentence boundary, with the remainder used as a fallback ratio so
`RatioDecidendi` is rarely left empty when a holding was found.

This is an explicit **pluggable extension point**: `ExtractorFunc` is the
function signature `ExtractHoldingAndRatio` satisfies, and
`PrecedentIngestionService.HoldingExtractor` lets callers substitute a
different heuristic (or a model-backed one) without changing the rest of
the pipeline.

Returns `ErrHoldingNotFound` when no marker can be located; this is not
fatal to a whole-corpus ingestion — `BuildPrecedentRules` still builds a
rule from the raw full text and records the failure in
`IngestResult.FailedHoldingIDs`.

---

## 3. Build Precedent Rules With Citations

```go
type PrecedentRule struct {
    irac.RuleNode
    Holding        string
    RatioDecidendi string
    Citation       string
    Source         RawPrecedent
}

func BuildPrecedentRule(id string, raw RawPrecedent, opts RuleBuildOptions) (PrecedentRule, error)
func BuildPrecedentRules(raws []RawPrecedent, opts RuleBuildOptions) ([]PrecedentRule, []string, error)
```

`BuildPrecedentRule` converts a single `RawPrecedent` into a
`PrecedentRule`: it extracts holding/ratio, formats the citation via
`FormatCitation` (e.g. `"Donoghue v Stevenson [1932] AC 562"`), and
constructs the underlying `irac.RuleNode` via `irac.NewRuleNode`, using
the extracted Holding+RatioDecidendi as the rule's `Text` (falling back to
`FullText` when extraction finds nothing).

---

## 4. Tag By Category and Issue

```go
func TagPrecedents(rules []PrecedentRule, opts TagOptions) []TaggedPrecedent
func ExtractIssueKeywords(text string, max int) []string
```

`TagPrecedents` attaches a `CategoryCode` (an opaque string mirroring
`packages/category`'s own convention, with no hard module dependency) plus
optional `JurisdictionCode`/`LegalFamily` overrides, and candidate
`IssueKeywords` extracted from the holding+ratio text via a lightweight
lexical heuristic (tokenize, drop stop words, dedupe, cap at `max`) for
later issue-matching.

---

## 5. Court-Hierarchy Weighting

```go
type CourtLevel string

const (
    CourtSupreme   CourtLevel = "supreme"
    CourtAppellate CourtLevel = "appellate"
    CourtTrial     CourtLevel = "trial"
    CourtUnknown   CourtLevel = "unknown"
)

func (l CourtLevel) Weight() float64
func ClassifyCourtLevel(courtName string) CourtLevel
func ApplyCourtHierarchy(rules []TaggedPrecedent, overrideLevel CourtLevel) []HierarchyRule
```

`ClassifyCourtLevel` is a deterministic heuristic that maps a free-text
court name to a `CourtLevel` via case-insensitive substring matching
against known court-name fragments (e.g. "House of Lords" ->
`CourtSupreme`, "Court of Appeal" -> `CourtAppellate`, "High Court" ->
`CourtTrial`). `Weight()` reflects binding-authority strength: higher
courts weigh higher (`CourtSupreme` > `CourtAppellate` > `CourtTrial` >
`CourtUnknown`).

---

## 6. Embed Holding and Ratio Text

```go
func EmbedPrecedents(ctx context.Context, svc embedding.EmbeddingService, rules []HierarchyRule, opts EmbedOptions) ([]EmbeddedPrecedent, error)
```

`EmbedPrecedents` computes embeddings for each precedent's combined
`Holding`+`RatioDecidendi` text (not the full judgment text) via the
existing `embedding.EmbeddingService.EmbedChunked` — this package never
reimplements embedding logic or references a specific provider. Rules
with empty combined text, or when no `EmbeddingService` is configured, are
skipped (no `EmbedChunked` call) and returned with an empty `Embeddings`
field rather than failing the whole batch.

---

## 7. Recency and Authority Scoring

```go
func RecencyScore(decidedDate, asOf time.Time) float64
func AuthorityScore(precedent HierarchyRule) float64
func AuthorityScoreAsOf(precedent HierarchyRule, asOf time.Time) float64
func ScorePrecedents(rules []EmbeddedPrecedent, asOf time.Time) []ScoredPrecedent
```

`RecencyScore` applies exponential decay to a precedent's age since
`DecidedDate`, halving every 15 years (a deliberately gentle decay, so
old-but-never-overruled foundational authorities still carry meaningful
weight). `AuthorityScore`/`AuthorityScoreAsOf` combine `CourtLevel.Weight()`
and `RecencyScore` into a single `[0, 1]` score as a weighted sum
(court hierarchy weighted 0.7, recency weighted 0.3) — court hierarchy is
the dominant legal signal, with recency as a secondary adjustment, so an
old-but-Supreme decision is not driven toward zero purely by age.

---

## 8. Persist Per Jurisdiction

```go
func PersistPrecedents(ctx context.Context, store graph.GraphStore, jurisdictionCode string, rules []ScoredPrecedent) ([]irac.RuleNode, error)
func LoadPrecedentsForJurisdiction(ctx context.Context, store graph.GraphStore, ruleIDs []string) ([]irac.RuleNode, error)
```

`PersistPrecedents` persists every rule's `irac.RuleNode` via
`graph.GraphStore.CreateNode`. `GraphStore` has no native
jurisdiction-scoped query, so `LoadPrecedentsForJurisdiction` fetches
previously persisted nodes back by the IDs the caller tracked. Errors are
wrapped with `ErrPersistFailed`/`ErrRuleNotFound` respectively,
`errors.Is`-compatible regardless of the underlying `GraphStore`
implementation.

---

## Design Principles

- **No separate `PrecedentNode` type.** The IRAC schema is fixed; every
  precedent is an `irac.RuleNode`, with precedent-specific metadata
  carried in this package's local `PrecedentRule` wrapper and its sibling
  pipeline structs.
- **No network fetch, no ML models.** `Loader` is a pure parser over
  caller-supplied input. Holding/ratio extraction, court classification,
  and keyword extraction are deterministic regex/text heuristics — the
  only delegated-to-a-model stage is embedding, delegated entirely to the
  existing `embedding.EmbeddingService`.
- **Opaque strings where no shared enum exists.** `CategoryCode` stays an
  opaque string type, mirroring `irac.RuleNode`'s own
  `JurisdictionCode`/`LegalFamily` fields and `packages/statute`'s own
  `CategoryCode` convention — no hard dependency on `packages/category`.
- **Court hierarchy dominates authority; recency adjusts it.**
  `AuthorityScore`'s weighted-sum blend (0.7 court weight / 0.3 recency)
  reflects how practitioners actually treat precedent: an old Supreme
  Court decision remains highly authoritative, while a recent trial-court
  decision remains only persuasive.
- **Partial extraction is a valid intermediate state.** A precedent whose
  `FullText` lacks a recognizable holding marker is not dropped from the
  corpus; it is still built (falling back to its full text) and flagged
  in `IngestResult.FailedHoldingIDs` for callers who need to know what
  could not be cleanly extracted.
