# IRAC Reasoning Tree Schema

`packages/irac` defines the formal schema for the Issue-Rule-Application-
Conclusion (IRAC) reasoning tree — the structural backbone every later
reasoning phase (033-062) builds on. This phase (031) defines **types and
validation only**. There is no storage backend here; Phase 032 (graph store
integration) builds the persistence layer on top of this schema.

## Why IRAC

IRAC is the standard structure for legal reasoning:

- **Issue** — the legal or factual question to be resolved.
- **Rule** — the legal rule, statute, or precedent that governs the issue.
- **Application** (sometimes called "Analysis") — applying the rule to the
  facts of the case.
- **Conclusion** — the outcome reasoned from that application.

Modeling this as a tree (rather than a flat document) lets downstream
phases trace *why* a conclusion was reached: which facts and rules fed
into which application, and which application a conclusion was drawn
from.

## Node model

Every node in the tree shares a common shape (`Node`, in `node.go`):

| Field        | Type          | Meaning                                             |
|--------------|---------------|------------------------------------------------------|
| `ID`         | `string`      | Unique within the case's tree                        |
| `Type`       | `NodeType`    | Which IRAC position this node occupies                |
| `CaseID`     | `string`      | The case this node belongs to                          |
| `Text`       | `string`      | Human-readable content                                 |
| `CreatedAt`  | `time.Time`   | When this node was created                              |
| `Confidence` | `float64`     | Extraction/reasoning confidence, in `[0, 1]`             |
| `Provenance` | `Provenance`  | Who/what generated this node, and from what              |

Five concrete typed wrappers embed `Node` and add type-specific fields:

- **`IssueNode`** — plus `Spans []SourceSpan`.
- **`RuleNode`** — plus `JurisdictionCode`, `LegalFamily`, and
  `Spans []SourceSpan`. Rule nodes are jurisdiction-aware because a rule's
  authority is inherently scoped to the jurisdiction (and legal
  tradition) it derives from.
- **`FactNode`** — plus `Spans []SourceSpan`.
- **`ApplicationNode`** — plus `Spans []SourceSpan`.
- **`ConclusionNode`** — plus `Label string` (always `draft_analysis`,
  see "Non-binding guardrail" below) and `Spans []SourceSpan`.

All five implement `NodeLike` (`GetID() string`, `GetType() NodeType`), so
callers can hold a heterogeneous `[]NodeLike` slice representing a mixed
tree without needing a type switch at every call site.

### Source-span linkage

Every concrete node type carries `Spans []SourceSpan`. `SourceSpan`
(`span.go`) mirrors `packages/segmentation`'s `SourceSpan` shape — rune
offset range plus optional OCR `Page` or STT `StartMS`/`EndMS` fields —
but is defined locally to avoid a hard module dependency. This guarantees
every claim in the reasoning tree traces back to its exact place in the
ingested source text, image, or audio.

### Confidence and provenance

Every node carries a `Confidence float64` in `[0, 1]` (mirroring
`packages/timeline`'s `Event.Confidence` convention) and a `Provenance`
struct (`provenance.go`):

```go
type Provenance struct {
    GeneratedBy     string    // e.g. "irac-issue-extractor-v1", "human-reviewer"
    GeneratedAt     time.Time
    UpstreamNodeIDs []string  // nodes this node was derived from, if any
}
```

### Jurisdiction tagging on Rule nodes

`RuleNode.JurisdictionCode` and `RuleNode.LegalFamily` are opaque strings
(`jurisdiction.go`) — no hard dependency on `packages/jurisdiction`. A
later integration phase can validate these tags against
`packages/jurisdiction`'s seeded records without any change to this
package's exported shape.

## Edge model

`EdgeType` (`edge.go`) has four values:

- `EdgeGoverns` — `Rule -> Issue`
- `EdgeAppliesTo` — `Application -> Fact`, `Application -> Rule`
- `EdgeSupports` — `Fact -> Application`
- `EdgeConcludesFrom` — `Conclusion -> Application`

### Constraint table

Only these five `(FromNodeType, EdgeType, ToNodeType)` triples are legal:

| From          | Edge             | To            |
|---------------|------------------|---------------|
| Rule          | governs          | Issue         |
| Application   | applies_to       | Fact          |
| Application   | applies_to       | Rule          |
| Fact          | supports         | Application   |
| Conclusion    | concludes_from   | Application   |

Diagrammatically, one issue's reasoning subtree looks like:

```
   Rule ──governs──▶ Issue

   Fact ──supports──▶ Application ──applies_to──▶ Rule
                            │                       ▲
                            └──applies_to──▶ Fact ───┘
                            ▲
   Conclusion ──concludes_from──┘
```

Any edge whose triple is not in this table (including every reversed
direction, e.g. `Issue -> governs -> Rule`) is illegal and is rejected by
`ValidateTree`.

## Tree revisions (versioning)

A tree is a sequence of immutable `TreeRevision`s (`version.go`), never
mutated in place:

```go
type TreeRevision struct {
    RevisionNumber int
    CaseID         string
    CreatedAt      time.Time
    ParentRevision *int // nil for the first revision in a case
}
```

`NewInitialRevision` builds revision 1; `NextRevision` builds the
successor to a given revision; `IsValidSuccessorOf` checks a revision is a
well-formed direct successor (same case, sequential number, correct
parent link).

## Validation

`ValidateTree(nodes []NodeLike, edges []Edge) []ValidationIssue`
(`validate.go`) checks:

1. Every edge's `FromID`/`ToID` resolves to a node in `nodes`
   (`ErrDanglingEdge` otherwise).
2. Every edge's `(FromNodeType, EdgeType, ToNodeType)` triple is in the
   constraint table above (`ErrIllegalEdgeTriple` otherwise).
3. No edge is a self-loop (`FromID == ToID`) (`ErrSelfLoop` otherwise).
4. Every node's `Type` is a recognized `NodeType`
   (`ErrUnknownNodeType` otherwise).
5. Every edge's `Type` is a recognized `EdgeType`
   (`ErrUnknownEdgeType` otherwise).
6. Every `ConclusionNode` carries the mandatory `draft_analysis` guardrail
   label (`ErrMissingGuardrailLabel` otherwise).

`ValidateTree` does **not** fail fast: it collects every issue found
across the whole tree and returns them all, so a caller can see
everything wrong with a candidate tree in one pass rather than fixing
problems one error at a time.

## Serialization

`MarshalTree`/`UnmarshalTree` (`serialize.go`) encode a tree's nodes,
edges, and revision metadata into a stable JSON envelope:

```json
{
  "version": 1,
  "revision": { "revision_number": 1, "case_id": "...", "created_at": "..." },
  "nodes": [
    { "kind": "issue", "node": { ... IssueNode fields ... } },
    { "kind": "rule", "node": { ... RuleNode fields ... } }
  ],
  "edges": [ { "from_id": "...", "to_id": "...", "type": "governs" } ]
}
```

Each node is tagged with a `kind` discriminator so `UnmarshalTree` can
reconstruct the correct concrete Go type. The round trip is lossless:
every field on every concrete node type survives marshal → unmarshal
unchanged. Both directions reject a `ConclusionNode` missing its
`draft_analysis` label with `ErrMissingGuardrailLabel`.

## Non-binding guardrail enforcement

CONTRIBUTING.md states:

> Every module that produces reasoning output must attach the
> `draft_analysis` label. Verdict or directive language is rejected by
> the output pipeline.

A `ConclusionNode` is reasoning output, so this package enforces the
guardrail **by construction**, not by convention:

- `DraftAnalysisLabel = "draft_analysis"` (`guardrail.go`) is the only
  value `ConclusionNode.Label` is ever set to.
- `NewConclusionNode(...)` is the **only exported constructor** for
  `ConclusionNode`. It unconditionally sets `Label` to
  `DraftAnalysisLabel` — there is no parameter, option, or code path that
  omits it. A bare `ConclusionNode{}` struct literal (bypassing the
  constructor) has an empty `Label` and is detectably non-conformant via
  `HasGuardrailLabel()`.
- `ValidateTree`, `MarshalTree`, and `UnmarshalTree` all additionally
  check `HasGuardrailLabel()` defensively, so a `ConclusionNode` that
  reaches this package's boundary some other way (e.g. hand-crafted JSON
  with the field stripped) is still caught rather than silently accepted.
- `ContainsVerdictLanguage(s string) bool` scans a string against a fixed
  word list of verdict/directive-sounding language ("guilty", "liable",
  "shall pay", "is ordered", "is hereby ordered", "judgment for",
  "convicted", "acquitted", "sentenced"), case-insensitively. Tests prove
  `DraftAnalysisLabel` itself never contains any of these — the guardrail
  label stays purely descriptive and non-binding, never drifting into
  directive language.

This is a **non-binding** guardrail: it labels reasoning output as draft
analysis rather than censoring the content of `ConclusionNode.Text`
itself (a conclusion's text may legitimately discuss verdict-adjacent
concepts when summarizing case posture — it is the *label*, attached
unconditionally, that signals "this is draft analysis, not a directive").

## What this phase deliberately does not do

- No persistence / storage backend (Phase 032).
- No HTTP handlers or service orchestration.
- No LLM or `packages/provider` integration — nodes are constructed by
  whatever caller (extractor, human reviewer, later reasoning phase)
  already has the data; this package only defines the shape and the
  integrity rules.
- No hard dependency on `packages/segmentation` or `packages/jurisdiction`
  — `SourceSpan`, `JurisdictionCode`, and `LegalFamily` are all local/
  opaque so this package can be built and tested in isolation.
