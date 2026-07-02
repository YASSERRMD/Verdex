# Verdex Application Model

## Overview

`packages/application` connects `irac.IssueNode`s to the rules that
govern them and builds `irac.ApplicationNode`s — the IRAC reasoning step
that applies a rule to a set of facts. The fixed IRAC schema
(Issue/Rule/Fact/Application/Conclusion) was established in Phase 031 and
is never modified afterward: this package never introduces a new node
type. Instead, every origin-specific concept this phase adds — precedent
linkage, distinguishing facts, multi-hop rule chains, legal-family
weighting — is a local wrapper struct that references the existing
`irac.RuleNode`, `irac.FactNode`, `irac.IssueNode`, and
`irac.ApplicationNode` types.

This package deliberately does **not** import `packages/statute` or
`packages/precedent`. Both packages already represent their output as
plain `irac.RuleNode` (see `packages/irac/node.go`'s `RuleNode` doc
comment: "a legal rule, statute, or precedent invoked to resolve an
Issue" — there is no separate node type per origin). Instead, this
package defines a local `Origin` enum and `OriginatedRule` wrapper, so a
future orchestration phase can hand it rules sourced from either package
without creating an import cycle.

---

## The Pipeline

```
match issue to rules -> build application nodes -> link
  precedents/distinguishing facts -> resolve rule chains -> weight by
  legal family -> score confidence -> persist subgraph
  -> return []irac.ApplicationNode
```

`ApplicationService.ApplyRules` runs the full pipeline over an issue, a
candidate rule set, and the case's facts, and returns the resulting
`irac.ApplicationNode`s, already persisted via a `graph.GraphStore`.
`ApplyRulesDetailed` returns the same nodes bundled with every
intermediate signal the pipeline derived (`ApplyResult`: `Matches`,
`PrecedentLinks`, `DistinguishingFacts`).

```go
svc := application.NewApplicationService()

result, err := svc.ApplyRulesDetailed(ctx, application.ApplyRequest{
    Issue:          issue,
    Rules:          []application.OriginatedRule{statuteRule, precedentRule},
    Facts:          facts,
    DominantFamily: "common_law",
    PrecedentRationales: map[string]string{
        precedentRule.Rule.ID: "same contested element: reasonableness of notice",
    },
})
```

This mirrors `packages/issue`'s `IssueExtractionService` and
`packages/fact`'s `FactConstructionService` orchestration pattern: a
single entry point wiring together this package's otherwise independent,
individually testable building blocks.

---

## 1. Origin and OriginatedRule

```go
type Origin string

const (
    OriginStatute   Origin = "statute"
    OriginPrecedent Origin = "precedent"
)

type OriginatedRule struct {
    Rule   irac.RuleNode
    Origin Origin
}
```

`OriginatedRule` is the seam between this package and whichever upstream
package produced a rule. A future orchestration phase constructs
`OriginatedRule{Rule: statuteRule, Origin: OriginStatute}` from
`packages/statute`'s output, or `OriginatedRule{Rule: precedentRule,
Origin: OriginPrecedent}` from `packages/precedent`'s output — this
package's own code never needs to know which package produced the rule
beyond that single tag.

---

## 2. Match Issue to Rules

```go
func MatchIssueToRules(issue irac.IssueNode, rules []OriginatedRule) []RuleMatch

type RuleMatch struct {
    Rule  OriginatedRule
    Score float64
}
```

`MatchIssueToRules` scores every candidate rule against the issue's text
using a symmetric keyword/token-overlap heuristic — no ML model,
deterministic lexical scoring, mirroring `packages/fact` and
`packages/issue`'s existing overlap conventions. The result is sorted by
descending `Score`; a rule that shares no tokens with the issue still
appears in the result, scored 0.

---

## 3. Build Application Nodes

```go
func BuildApplicationNode(issue irac.IssueNode, rule OriginatedRule, facts []irac.FactNode) (irac.ApplicationNode, error)
```

`BuildApplicationNode` converts a matched `(issue, rule, facts)` triple
into an `irac.ApplicationNode` via `irac.NewApplicationNode`, with a
short deterministic text summary of which rule was applied to which
facts. The node's ID is derived deterministically from the issue and
rule IDs, so re-running the pipeline over the same pair is idempotent
(`graph.GraphStore.CreateNode` is an idempotent upsert).

---

## 4. Precedent Linkage

```go
type PrecedentIssueLink struct {
    IssueID   string
    Rule      OriginatedRule
    Rationale string
    LinkedAt  time.Time
}

func NewPrecedentIssueLink(issueID string, rule OriginatedRule, rationale string, linkedAt time.Time) (PrecedentIssueLink, error)
```

A precedent's relevance to an issue is a distinct claim from a statute's:
a statute simply governs the issue by its text; a precedent is relevant
because a *previous case* decided a materially similar issue.
`PrecedentIssueLink` records that claim explicitly, with a rationale.
`NewPrecedentIssueLink` rejects statute-origin rules — this linkage type
is only meaningful for `OriginPrecedent`.

---

## 5. Distinguishing Facts

```go
type DistinguishingFact struct {
    Fact      irac.FactNode
    Rule      OriginatedRule
    Rationale string
    NotedAt   time.Time
}

func NewDistinguishingFact(fact irac.FactNode, rule OriginatedRule, rationale string, notedAt time.Time) (DistinguishingFact, error)
```

The classic common-law "distinguishing" move: a party argues a cited
precedent should not control because the present case's facts diverge
from it in some legally significant way. `DistinguishingFact` records
which current-case fact diverges, from which precedent, and why. Like
`PrecedentIssueLink`, this is only meaningful when `Origin ==
OriginPrecedent` — a statute has no "typical fact pattern" to diverge
from.

---

## 6. Multi-Hop Rule Chains

```go
type RuleChain struct {
    Rules []OriginatedRule
}

func (c RuleChain) Validate() error
```

Some rules only make sense applied together — a statute section that
cross-references another section, for example. `RuleChain` is an ordered
sequence of `OriginatedRule`s representing such a group. `Validate`
rejects an empty chain (`ErrEmptyInput`) and rejects a cycle
(`ErrCyclicChain`) — the same underlying `irac.RuleNode.ID` appearing more
than once in the chain, which would mean a rule circularly depends on
itself.

---

## 7. Legal-Family Weighting

```go
func WeightByLegalFamily(rule OriginatedRule, dominantFamily string) float64
```

Legal traditions treat statute and precedent as authority differently:

| `dominantFamily` | `OriginStatute` | `OriginPrecedent` |
|------------------|-----------------|--------------------|
| `"common_law"`   | 0.8             | 1.0                |
| `"civil_law"`    | 1.0             | 0.8                |
| anything else    | 1.0             | 1.0                |

Under `common_law` (England & Wales, most US states), judicial precedent
(*stare decisis*) is itself a primary, binding source of law, so a
precedent-origin rule is weighted higher. Under `civil_law` (France,
Germany, most of continental Europe), codified statute is the primary
source and judicial decisions are persuasive but not formally binding, so
the weighting reverses. Any other/unrecognized family is treated as
neutral — this package has no basis to prefer one origin over the other
without a recognized legal-family signal.

---

## 8. Confidence

```go
func ComputeConfidence(match RuleMatch, dominantFamily string) float64
func ApplyConfidence(node irac.ApplicationNode, match RuleMatch, dominantFamily string) irac.ApplicationNode
```

`ComputeConfidence` combines a `RuleMatch.Score` (the dominant signal,
weight 0.7 — a rule that does not textually relate to the issue at all
should not gain much confidence just from favorable legal-family
weighting) with `WeightByLegalFamily`'s output (a secondary adjustment,
weight 0.3) into a single `[0, 1]` confidence value, mirroring
`packages/issue/confidence.go`'s weighted-aggregate convention.
`ApplyConfidence` sets the result on the built `ApplicationNode`.

---

## 9. Persist Application Subgraph

```go
func PersistApplicationSubgraph(ctx context.Context, store graph.GraphStore, node irac.ApplicationNode, rule OriginatedRule, issueID string, facts []irac.FactNode, ruleGovernsIssueExists bool) error
```

`PersistApplicationSubgraph` persists the `ApplicationNode` via
`graph.GraphStore.CreateNode`, then creates every legal edge this package
owns, per `packages/irac/edge.go`'s `legalEdgeTriples` constraint table:

- `Application --applies_to--> Rule`
- `Application --applies_to--> Fact` (one per fact)
- `Rule --governs--> Issue` (only if `ruleGovernsIssueExists` is false)

Every edge is checked against `irac.IsLegalEdgeTriple` before being sent
to the store; an illegal triple returns `ErrIllegalEdge` without ever
reaching `graph.GraphStore.CreateEdge`.

`Fact --supports--> Application` edges are **not** created here — that
triple's source is a `FactNode`, and `packages/fact`'s own `PersistFacts`
already owns creating those edges once `ApplicationNode` IDs are known.
Duplicating that responsibility here would risk the two packages
disagreeing about which facts support which applications.

---

## 10. ApplicationService

`ApplicationService.ApplyRules` (and its detailed variant,
`ApplyRulesDetailed`) orchestrates the full pipeline described above:
match, build, link precedents/distinguishing facts (for any rule with a
supplied rationale), resolve a rule chain if one was supplied, weight by
legal family, score confidence, and persist. `ApplyRequest.TopN` caps how
many of the highest-scoring matches are built into application nodes;
zero means every positively-scoring match.

---

## Design Principles

- **No new node types.** Every origin-specific concept in this package —
  `PrecedentIssueLink`, `DistinguishingFact`, `RuleChain` — is a local
  wrapper struct referencing existing `irac` node types, never a new one.
- **No hard dependency on `packages/statute` or `packages/precedent`.**
  The `Origin`/`OriginatedRule` abstraction is the only seam a future
  orchestration phase needs.
- **Every edge is checked against `irac.IsLegalEdgeTriple` before
  persistence.** This package never asks a `graph.GraphStore` to persist
  an edge outside `packages/irac/edge.go`'s constraint table.
- **No ML models.** Matching, weighting, and confidence scoring are all
  deterministic, lexical/table-driven heuristics, consistent with this
  project's shared "no ML models, rule based" design principle.
