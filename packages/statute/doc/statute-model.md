# Verdex Statute Model

## Overview

`packages/statute` loads jurisdiction statutes as structured
`irac.RuleNode`s (see `packages/irac/node.go`), enriching each rule with a
formatted citation, category/jurisdiction tags, amendment/effective-date
history, resolved cross-references, and retrieval embeddings, before
persisting it via `graph.GraphStore`.

Like `packages/fact`'s construction pipeline and `packages/evidence`'s
classifier, every heuristic in this package (hierarchy parsing,
cross-reference detection, citation formatting) is deterministic and
lexical/pattern-based — no ML model at runtime, except for the embedding
step, which is delegated entirely to the existing
`embedding.EmbeddingService`.

---

## The Pipeline

```
load -> parse hierarchy -> build rule nodes with citations -> tag
  -> track amendments -> resolve cross-refs -> embed -> persist
  -> return []irac.RuleNode
```

`StatuteIngestionService.Ingest` runs the full pipeline over a raw corpus
source and a target jurisdiction, and returns the resulting
`irac.RuleNode`s, already persisted via a `graph.GraphStore`.
`IngestDetailed` returns the same rules bundled with every intermediate
signal the pipeline derived (`IngestResult`).

```go
svc := statute.NewStatuteIngestionService()
svc.Embedding = myEmbeddingService // optional; nil skips embedding

nodes, err := svc.Ingest(ctx, statute.IngestRequest{
    Source:           corpusReader,
    JurisdictionCode: "AE",
    LegalFamily:      jurisdiction.LegalFamilyCivilLaw,
    CategoryCode:     "civil",
})
```

This mirrors `packages/fact`'s `FactConstructionService` orchestration
pattern: a single entry point wiring together this package's otherwise
independent, individually testable building blocks.

---

## 1. Load

```go
type Loader interface {
    Load(ctx context.Context, source io.Reader) ([]RawStatute, error)
}
```

`DefaultLoader` is a pure parser — no network fetch. It auto-detects two
input shapes:

- **JSON**: a top-level array of `RawStatute` objects, or an object with a
  top-level `"statutes"` array field.
- **Structured text**: a line-oriented format where each act begins with
  `ACT <number>: <title>` and every following line up to the next `ACT`
  header (or end of input) is that act's `Body`.

`RawStatute.Body` is left unparsed — hierarchy parsing is a separate stage.

---

## 2. Parse Hierarchy

```go
func ParseHierarchy(raw RawStatute) (*StatuteNode, error)
```

`StatuteNode` is a recursive `Act -> Section -> Clause` tree: the same
shape is reused at every level (`Level` distinguishes the tier), each node
carrying `Number`, `Title`, `Text`, and `Children`. `ParseHierarchy` scans
`raw.Body` line by line for `Section N` and `(x)` markers, appending any
other line to whichever node is currently open. `Walk` and `Leaves`
support depth-first traversal and leaf extraction.

---

## 3. Build Rule Nodes With Citations

```go
type Citation struct {
    Act, Section, Clause string
}

func BuildRuleNodes(act *StatuteNode, opts RuleBuildOptions) ([]BuiltRule, error)
```

`BuildRuleNodes` converts a parsed `StatuteNode` tree into one
`irac.RuleNode` per node at a configurable granularity — clause-level
(the default) or section-level — via `irac.NewRuleNode`. Each produced
rule carries a `Citation`, formatted via `Citation.String()` as
`"Act 12, s.5(a)"` (or `"Act 12, s.5"` / `"Act 12"` when clause/section are
absent). `BuiltRule` bundles the node with its `Citation` and source
`StatuteNode`.

---

## 4. Tag By Category and Jurisdiction

```go
func TagRules(rules []BuiltRule, opts TagOptions) []TaggedRule
```

`TagRules` attaches a `CategoryCode` (an opaque string mirroring
`packages/category`'s own convention, with no hard module dependency) plus
optional `JurisdictionCode`/`LegalFamily` overrides to every rule.
`LegalFamily` is typed as `jurisdiction.LegalFamily` — this package
imports `packages/jurisdiction` directly (see `go.mod`) since a shared
enum already exists there, unlike `CategoryCode`/`JurisdictionCode`, for
which no equivalent shared enum exists.

---

## 5. Effective Dates and Amendments

```go
type Amendment struct {
    PriorText     string
    EffectiveDate time.Time
    Description   string
}

type AmendmentRecord struct {
    RuleID         string
    EffectiveDate  *time.Time
    History        []Amendment
    SupersededBy   *string
}
```

`AmendmentRecord` is a sibling struct — not embedded on `irac.RuleNode`,
since this phase must not modify `packages/irac` — tracking a rule's
current effective date, chronological amendment `History`, and an
optional `SupersededBy` link to the `irac.RuleNode.ID` that replaced it.
`SupersessionChain` walks that link (with cycle detection) to find the
full lineage of a superseded rule. `ApplyAmendments` zips `TaggedRule`s
with their `AmendmentRecord` into `AmendedRule`.

---

## 6. Cross-Reference Resolution

```go
func DetectCrossReferences(sourceRuleID, text string) []CrossReference
func ResolveCrossReferences(refs []CrossReference, rules []BuiltRule) []CrossReference
```

`DetectCrossReferences` scans rule text for citation-shaped references
(e.g. `"see Section 12"`, `"Section 5(a)"`) via regex.
`ResolveCrossReferences` matches each detected reference's section/clause
against every rule's `Citation` within the same loaded corpus, filling in
`ResolvedRuleID` when a match is found. Unresolved references keep an
empty `ResolvedRuleID` rather than erroring — a partially-resolved corpus
is a valid intermediate state during ingestion; callers that need strict
resolution inspect `UnresolvedCrossReferences` or return
`ErrUnresolvedCrossReference` themselves.

---

## 7. Embed Rule Text

```go
func EmbedRules(ctx context.Context, svc embedding.EmbeddingService, rules []AmendedRule, opts EmbedOptions) ([]EmbeddedRule, error)
```

`EmbedRules` computes embeddings for each rule's text via the existing
`embedding.EmbeddingService.EmbedChunked` — this package never
reimplements embedding logic or references a specific provider. Rules
with empty text, or when no `EmbeddingService` is configured, are skipped
(no `EmbedChunked` call) and returned with an empty `Embeddings` field
rather than failing the whole batch.

---

## 8. Persist Per Jurisdiction

```go
func PersistRules(ctx context.Context, store graph.GraphStore, jurisdictionCode string, rules []EmbeddedRule) ([]irac.RuleNode, error)
func LoadRulesForJurisdiction(ctx context.Context, store graph.GraphStore, ruleIDs []string) ([]irac.RuleNode, error)
```

`PersistRules` persists every rule's `irac.RuleNode` via
`graph.GraphStore.CreateNode`. `GraphStore` has no native
jurisdiction-scoped query, so `LoadRulesForJurisdiction` fetches
previously persisted nodes back by the IDs the caller tracked (typically
the IDs `PersistRules` returned). Errors are wrapped with
`ErrPersistFailed`/`ErrRuleNotFound` respectively, `errors.Is`-compatible
regardless of the underlying `GraphStore` implementation.

---

## Design Principles

- **No network fetch, no ML models.** `Loader` is a pure parser over
  caller-supplied input. Hierarchy parsing and cross-reference detection
  are deterministic regex/text heuristics — the only delegated-to-a-model
  stage is embedding, and even that is delegated entirely to the existing
  `embedding.EmbeddingService` rather than reimplemented here.
- **Opaque strings where no shared enum exists, real imports where one
  does.** `CategoryCode` and `irac.RuleNode`'s own
  `JurisdictionCode`/`LegalFamily` fields stay opaque strings — no hard
  dependency on `packages/category`. `LegalFamily` in `TagOptions`,
  however, is typed as `jurisdiction.LegalFamily` directly, since
  `packages/jurisdiction` already owns that enum.
- **Amendment history lives beside, not inside, `irac.RuleNode`.** This
  phase must not modify `packages/irac`'s schema, so `AmendmentRecord` is
  carried alongside the node it describes throughout the pipeline.
- **Partial resolution is a valid intermediate state.** A single
  malformed act, or an unresolved cross-reference, does not hard-fail an
  entire corpus ingestion; callers inspect `IngestResult` for what
  succeeded and what did not.
