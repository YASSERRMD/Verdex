# Second-party argument agent (`packages/secondpartyagent`)

Phase 052 is the direct adversarial counterpart to `packages/firstpartyagent`
(Phase 051), built on `packages/agentframework` (Phase 049). Its job in
Part 5 — Reasoning & Adversarial Synthesis — is to construct the strongest
good-faith case for a second party across every issue already framed in a
case's tree, while explicitly targeting and rebutting the first party's
already-constructed arguments. The result is a structured `ArgumentSet`
that Phase 053's evidence-weighing module and Phase 055's synthesis agent
consume alongside `firstpartyagent`'s own output.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/issueagent` (Phase 050) | `FramedIssue` / `IssueAnalysisResult` — ranked, jurisdiction-aware issue framing. | Upstream input, exactly as for `firstpartyagent`. This package never re-derives issue framing or materiality ranking — a caller passes an `[]issueagent.FramedIssue` into `secondpartyagent.New`, and this package only ever reads `SourceIssueNodeID`, `Question`, and `GoverningQuestions` from it. |
| `packages/firstpartyagent` (Phase 051) | `ArgumentSet` / `Argument` / `CitationRef` — the first party's own good-faith case. | **New for this phase.** This package imports `firstpartyagent` for its exported types only — never its internal logic — to read the first party's `Claim`, `IssueNodeID`, and `Counterarguments` as the rebuttal target list rendered into this agent's own prompt. This package's own `Argument`/`ArgumentSet`/`CitationRef` types are structurally independent (not aliases) of `firstpartyagent`'s, so this package's rebuttal-linkage addition (`RebutsArgumentIDs`, `FabricatedRebuttalIDs`) never has to be retrofitted onto `firstpartyagent`'s shape. |
| `packages/agentframework` (Phase 049) | `Agent` interface, `Runner`/`Config`, `Scratchpad`, `Budget`, `Stats`, `Seed`. | `secondpartyagent.Agent` implements `agentframework.Agent`, mirroring `firstpartyagent.Agent`'s single-step design exactly. Every model call is driven by an `agentframework.Runner`, constructed either directly or via this package's `Argue` convenience function. No step loop, budget enforcement, or telemetry logic is reimplemented here. |
| `packages/knowledgeapi` | `KnowledgeAPI` facade — `GetTree`, `GetNode`, `ResolveCitation`. | The only way this package touches a case's tree, exactly as `firstpartyagent` uses it. `fetch.go`'s `fetchIssueEvidence` calls `GetTree` once per run and resolves both governing rules and Application-linked supporting facts locally from the returned edge list. `assemble.go` calls `ResolveCitation` once per surviving argument's supporting rule. |
| `packages/irac` | `NodeType`, `EdgeType` (`EdgeGoverns`, `EdgeAppliesTo`, `EdgeSupports`). | Read-only. This package never constructs a `FactNode`, `RuleNode`, `ApplicationNode`, or any tree edge — its own output type, `Argument`, references existing node IDs by string instead of embedding or duplicating them. |
| `packages/citation` (via `knowledgeapi.ResolveCitation`) | Citation resolution and the `Verify` anti-hallucination check. | This package never resolves or verifies a citation itself; it only records `knowledgeapi.CitationDTO`'s fields into its own `CitationRef` shape and folds `ConfidenceScore`/`Verified` into `strengthScore`. |
| `packages/prompts` | `PromptTemplate`, `Registry`, `Render`, `VariantSelector.SelectBest`, the mandatory non-binding disclaimer. | This package's own argument-rebuttal template lives under `packages/secondpartyagent/templates` (package-local, mirroring `packages/firstpartyagent/templates`'s own placement rationale) but registers into the single shared `prompts.DefaultRegistry` under a distinct ID, `secondpartyagent.argument.rebuttal`. |
| `packages/router` / `packages/provider` | Model-agnostic routing, `ChatRequest`/`ChatResponse`, `TaskType`. | `secondpartyagent.Agent.TaskType()` returns `provider.TaskReason`. Every model call is dispatched through a caller-supplied `*router.Router`; this package never imports `packages/adapters` or talks to an `LLMProvider` directly. |
| `packages/timeline` (and any party/role domain package) | Case participant / party modeling. | **Not imported.** `PartyID` is this package's own opaque `string` type, exactly mirroring `firstpartyagent.PartyID`'s convention — a caller maps a richer domain party concept onto `PartyID` at its own boundary. |

## What this agent does

For a given case, party, set of already-framed issues, and the first
party's `ArgumentSet`:

1. **Pulls the case's full tree** via one `knowledgeapi.GetTree` call and
   resolves, for every input `FramedIssue`, the same governing-rule and
   Application-linked-fact evidence `packages/firstpartyagent` resolves —
   see `fetch.go`. The second party argues from the same case tree as the
   first; "favoring the second party" happens in argument construction
   and framing, not by hiding evidence from one side.
2. **Renders one jurisdiction- and party-aware argument-construction-and-
   rebuttal prompt** covering every input issue at once, listing — per
   issue — the *exact, exhaustive* set of fact/rule IDs the model is
   permitted to cite, and — separately — every first-party argument
   available to rebut, each tagged with an `opposing_argument_id` (the
   first party's real `Argument.ID`) and its own `Claim` and
   `Counterarguments` (the first party's own anticipated rebuttals,
   fed in as a starting point for this agent's targeted rebuttal rather
   than a generic denial).
3. **Parses the model's structured JSON response** into one or more
   proposed arguments per issue: a claim, supporting fact/rule IDs,
   `rebuts_argument_ids` (the opposing arguments this claim targets),
   anticipated counterarguments, and a confidence value.
4. **Grounds every proposed argument against the tree and the opposing
   argument set** (see "Anti-fabrication grounding" below), stripping or
   dropping any argument that cites a node ID or opposing-argument ID the
   model invented.
5. **Resolves citations** for every surviving argument's supporting rule
   IDs via `knowledgeapi.ResolveCitation`.
6. **Scores each argument's strength** in `[0,1]`, using the identical
   rubric `firstpartyagent` uses (see "Strength scoring" below) so the
   two adversarial outputs are directly comparable.
7. **Assembles the case's `ArgumentSet`**, recording any issue for which
   every proposed argument was fully fabricated in
   `SkippedIssueNodeIDs` rather than silently dropping it.

Like `firstpartyagent.Agent`, this agent is a **single-step** agent by
design: `BuildRequest` gathers every issue's evidence (and the opposing
argument set) once, and `Interpret` always concludes on the first model
turn.

## Anti-fabrication grounding

`ground.go`'s `groundArgument` extends `firstpartyagent`'s node-ID
grounding with a second, independent grounding pass over rebuttal
linkage:

- Every ID in a proposed argument's `supporting_fact_ids` and
  `supporting_rule_ids` is checked against `issueEvidence.allowedNodeIDs`
  — the exact set this package resolved from the case's real tree for
  that issue. Any ID not in that set is stripped and recorded in
  `FabricatedNodeIDs`, with `Grounded` set to `false` — identical to
  `firstpartyagent`'s contract.
- **New in this phase:** every ID in a proposed argument's
  `rebuts_argument_ids` is checked against the exact set of
  `firstpartyagent.Argument.ID` values present in the `ArgumentSet`
  supplied to `New`. Any ID not in that set is a fabricated rebuttal
  target — the model claimed to be rebutting an opposing argument that
  does not actually exist. The fabricated ID is stripped from
  `RebutsArgumentIDs` and recorded in `FabricatedRebuttalIDs`.
- A fabricated rebuttal target **never** causes the containing `Argument`
  to be dropped, and never flips `Grounded` to `false` — `Grounded`
  continues to track only supporting fact/rule fabrication, exactly as in
  `firstpartyagent`. Rebuttal linkage is an enrichment of an argument
  that already stands on its own supporting evidence, not a precondition
  for the argument existing. This mirrors `groundArgument`'s existing
  "partial fabrication is kept, flagged" philosophy for supporting IDs,
  applied to a second, independent ID space.
- An argument left with **zero** real supporting fact/rule IDs after
  stripping is still dropped entirely, exactly as in `firstpartyagent` —
  this precondition is unaffected by whether its rebuttal targets were
  real or fabricated.
- A caller supplying an empty (zero-value) first-party `ArgumentSet` —
  e.g. because the first-party agent skipped every issue — is handled
  the same way: every proposed `rebuts_argument_ids` entry is fabricated
  by definition (nothing to reference), so every rebuttal claim is
  stripped, but the second party's own affirmative arguments are still
  constructed and returned normally.

This is the same anti-fabrication philosophy `packages/firstpartyagent`
established (never trust a model's own claim of good faith; verify every
reference against ground truth the package itself resolved), applied
independently to two separate reference spaces: tree node IDs and
opposing-argument IDs.

## Strength scoring

`score.go`'s `strengthScore` is identical to
`packages/firstpartyagent/score.go`'s: the same three `[0,1]` signals
(citation verification, weight 0.4; fact confidence, weight 0.35;
rule-linkage richness, weight 0.25, saturating at 3 rules) blended into a
single `Argument.Strength`. Keeping the scoring rubric party-agnostic and
identical across both adversarial agents means Phase 053's
evidence-weighing module and Phase 055's synthesis agent can compare a
first-party and second-party argument's `Strength` values directly,
without needing to normalize across two different formulas.

## Prompt template

`packages/secondpartyagent/templates` registers a single template,
`secondpartyagent.argument.rebuttal`, with `NonBindingLabel: true`, via
its own `init()`:

```go
import _ "github.com/YASSERRMD/verdex/packages/secondpartyagent/templates"
```

It follows `packages/firstpartyagent/templates`'s exact registration
convention (package-local template, shared `prompts.DefaultRegistry`,
`WithRegistry` escape hatch) and jurisdiction-aware selection
(`prompts.VariantSelector{}.SelectBest`, tiered fallback: exact ->
locale-only -> family-only -> universal). Its prompt body adds one new
section absent from `firstpartyagent`'s template — "OPPOSING PARTY'S
ARGUMENTS TO REBUT" — listing every first-party argument by
`opposing_argument_id`, `claim`, and anticipated rebuttals, and instructs
the model to populate `rebuts_argument_ids` in its JSON response using
only IDs from that list.

## Output shape

```go
type PartyID string // opaque, caller-defined — independent of firstpartyagent.PartyID

type CitationRef struct {
    NodeID             string
    Citation           string
    VerificationStatus string
    Verified           bool
    ConfidenceScore    float64
}

type Argument struct {
    ID                    string
    IssueNodeID           string   // the irac.IssueNode.ID this argument addresses
    PartyID               PartyID
    Claim                 string
    SupportingFactIDs     []string // guaranteed to exist in the case's tree
    SupportingRuleIDs     []string // guaranteed to exist in the case's tree
    Citations             []CitationRef
    Counterarguments      []string
    RebutsArgumentIDs     []string // firstpartyagent.Argument.ID values this argument targets — guaranteed real
    Strength              float64  // [0,1]
    Grounded              bool     // false if any cited fact/rule ID had to be stripped
    FabricatedNodeIDs     []string // fact/rule IDs stripped, for transparency
    FabricatedRebuttalIDs []string // rebuttal target IDs stripped, for transparency
}

type ArgumentSet struct {
    CaseID              string
    PartyID             PartyID
    Arguments           []Argument
    SkippedIssueNodeIDs []string // issues where every argument failed grounding
    GeneratedAt         time.Time
}
```

## Usage

```go
issueResult, _, err := issueagent.Analyze(ctx, issueAgent, caseID, issueagent.AnalyzeConfig{Router: r})

firstPartyResult, _, err := firstpartyagent.Argue(ctx, firstAgent, caseID, firstpartyagent.ArgueConfig{Router: r})

agent, err := secondpartyagent.New(api, secondpartyagent.PartyID("party-defendant"), issueResult.Issues, firstPartyResult,
    secondpartyagent.WithJurisdictionName("California"),
    secondpartyagent.WithLegalFamily("common_law"),
    secondpartyagent.WithPartyLabel("the defendant"),
)

result, runResult, err := secondpartyagent.Argue(ctx, agent, caseID, secondpartyagent.ArgueConfig{
    Router:   myRouter, // *router.Router — required
    Budget:   agentframework.DefaultBudget(),
    Seed:     agentframework.Seed{},
    TenantID: tenantID,
})
// result is a typed secondpartyagent.ArgumentSet, feeding Phase 053's
// evidence-weighing module and Phase 055's synthesis agent alongside
// firstPartyResult.
```

A caller driving an `agentframework.Runner` directly can recover the same
typed result from `Result.FinalText` via `secondpartyagent.DecodeResult`.

## What this package deliberately does not do

- It does not frame or rank issues — that is `packages/issueagent`'s job,
  performed before this package ever runs. This package treats its input
  `[]issueagent.FramedIssue` as already correct and complete.
- It does not construct the first party's arguments, re-score them, or
  mutate `firstpartyagent`'s output in any way — it only reads
  `ID`, `IssueNodeID`, `Claim`, and `Counterarguments` off each supplied
  `firstpartyagent.Argument` to render the rebuttal-target prompt block
  and to validate `RebutsArgumentIDs`.
- It does not construct, mutate, or persist any `irac` tree node or edge.
  Its output is consumed data for Phase 053's evidence-weighing module or
  the synthesis agent (Phase 055), never written back into the case's
  reasoning tree.
- It does not model parties, roles, or standing itself — `PartyID` is an
  intentionally opaque local string, never a hard dependency on
  `packages/timeline` or any other participant/role package, and never
  the same Go type as `firstpartyagent.PartyID` (a caller mapping both
  parties onto a shared domain concept does so at its own boundary).
- It does not weigh evidence reliability, reconcile the two parties'
  competing claims, or produce a final judgment — that is Phase 053's
  (evidence-weighing module) and Phase 055's (synthesis agent) job
  respectively. This package's output is one more adversarial input to
  those downstream stages, not a resolution of the dispute.
- It does not talk to a model provider directly, retry across providers,
  or implement a fallback chain — that is `packages/router`'s job.
- It does not enforce the non-binding guardrail beyond the
  `NonBindingLabel` disclaimer `packages/prompts` appends to the rendered
  prompt text — Phase 057's project-wide guardrail-enforcement layer is
  responsible for anything stronger applied to this agent's output.
