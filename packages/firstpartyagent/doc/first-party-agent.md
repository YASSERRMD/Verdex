# First-party argument agent (`packages/firstpartyagent`)

Phase 051 is the second concrete agent built on `packages/agentframework`
(Phase 049), consuming `packages/issueagent`'s (Phase 050)
`IssueAnalysisResult` as its own input. Its job in Part 5 — Reasoning &
Adversarial Synthesis — is to construct the strongest good-faith case for
a single party across every issue already framed in a case's tree,
producing a structured `ArgumentSet` for Phase 052's second-party
(rebuttal) agent to argue against and Phase 055's synthesis agent to
reconcile alongside every other agent's output.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/issueagent` (Phase 050) | `FramedIssue` / `IssueAnalysisResult` — ranked, jurisdiction-aware issue framing. | Upstream input. This package never re-derives issue framing or materiality ranking — a caller passes an `[]issueagent.FramedIssue` (typically `IssueAnalysisResult.Issues`) into `firstpartyagent.New`, and this package only ever reads `SourceIssueNodeID`, `Question`, and `GoverningQuestions` from it. |
| `packages/agentframework` (Phase 049) | `Agent` interface, `Runner`/`Config`, `Scratchpad`, `Budget`, `Stats`, `Seed`. | `firstpartyagent.Agent` implements `agentframework.Agent`, mirroring `issueagent.Agent`'s single-step design exactly. Every model call is driven by an `agentframework.Runner`, constructed either directly or via this package's `Argue` convenience function. No step loop, budget enforcement, or telemetry logic is reimplemented here. |
| `packages/knowledgeapi` | `KnowledgeAPI` facade — `GetTree`, `GetNode`, `ResolveCitation`. | The only way this package touches a case's tree. `fetch.go`'s `fetchIssueEvidence` calls `GetTree` once per run and resolves both governing rules and Application-linked supporting facts locally from the returned edge list — it does not import `packages/graph`, `packages/treeindex`, or `packages/traversal` directly. `assemble.go` calls `ResolveCitation` once per surviving argument's supporting rule. |
| `packages/irac` | `NodeType`, `EdgeType` (`EdgeGoverns`, `EdgeAppliesTo`, `EdgeSupports`). | Read-only. This package never constructs a `FactNode`, `RuleNode`, `ApplicationNode`, or any tree edge — its own output type, `Argument`, references existing node IDs by string instead of embedding or duplicating them. |
| `packages/citation` (via `knowledgeapi.ResolveCitation`) | Citation resolution and the `Verify` anti-hallucination check. | This package never resolves or verifies a citation itself; it only records `knowledgeapi.CitationDTO`'s fields into its own `CitationRef` shape and folds `ConfidenceScore`/`Verified` into `strengthScore`. |
| `packages/prompts` | `PromptTemplate`, `Registry`, `Render`, `VariantSelector.SelectBest`, the mandatory non-binding disclaimer. | This package's own argument-construction template lives under `packages/firstpartyagent/templates` (package-local, mirroring `packages/issueagent/templates`'s own placement rationale) but registers into the single shared `prompts.DefaultRegistry`. |
| `packages/router` / `packages/provider` | Model-agnostic routing, `ChatRequest`/`ChatResponse`, `TaskType`. | `firstpartyagent.Agent.TaskType()` returns `provider.TaskReason`. Every model call is dispatched through a caller-supplied `*router.Router`; this package never imports `packages/adapters` or talks to an `LLMProvider` directly. |
| `packages/timeline` (and any party/role domain package) | Case participant / party modeling. | **Not imported.** `PartyID` is this package's own opaque `string` type, exactly mirroring `CaseID`'s plain-string convention — a caller maps a richer domain party concept onto `PartyID` at its own boundary. This keeps `firstpartyagent` decoupled from whichever package eventually owns a full party/role model. |

## What this agent does

For a given case, party, and set of already-framed issues:

1. **Pulls the case's full tree** via one `knowledgeapi.GetTree` call and
   resolves, for every input `FramedIssue`:
   - its governing `RuleNode`s (`Rule --governs--> Issue`, the same edge
     direction and resolution approach `packages/issueagent` uses), and
   - `FactNode`s reachable from those rules through the tree's
     `ApplicationNode` layer (`Application --applies_to--> Rule` and
     `Fact --supports--> Application`), ranked by confidence and capped
     (`maxFactsPerIssue` / `maxRulesPerIssue`) to keep the prompt
     bounded. An issue with no governing-rule linkage at all falls back
     to every `FactNode` in the tree, so a thin issue still gets some
     evidence to argue from.
2. **Renders one jurisdiction- and party-aware argument-construction
   prompt** covering every input issue at once, listing — per issue —
   the *exact, exhaustive* set of fact/rule IDs the model is permitted to
   cite, and instructing it not to invent IDs outside that list.
3. **Parses the model's structured JSON response** (tolerating a
   markdown code fence or surrounding prose, mirroring
   `packages/issueagent/parse.go`'s tolerance) into one or more proposed
   arguments per issue: a claim, supporting fact/rule IDs, anticipated
   counterarguments, and a confidence value.
4. **Grounds every proposed argument against the tree** (see "Anti-
   fabrication grounding" below), stripping or dropping any argument that
   cites a node ID the model invented.
5. **Resolves citations** for every surviving argument's supporting rule
   IDs via `knowledgeapi.ResolveCitation`.
6. **Scores each argument's strength** in `[0,1]` (see "Strength
   scoring" below).
7. **Assembles the case's `ArgumentSet`**, recording any issue for which
   every proposed argument was fully fabricated in
   `SkippedIssueNodeIDs` rather than silently dropping it.

Like `issueagent.Agent`, this agent is a **single-step** agent by design:
`BuildRequest` gathers every issue's evidence once and `Interpret` always
concludes on the first model turn. A future revision wanting iterative
exploration (e.g. following up a weak argument with a targeted
`knowledgeapi.Retrieve` call) is a change to `Interpret`'s `Decision`, not
a different framework.

## Anti-fabrication grounding

The prompt instructs the model to cite only fact/rule IDs from the
explicit evidence list it was given, but a model can still hallucinate an
ID — either one that exists nowhere in the tree, or one that exists but
was never actually offered as evidence for that issue. `ground.go`'s
`groundArgument` closes this gap deterministically, without trusting the
model's own claim of good faith:

- Every ID in a proposed argument's `supporting_fact_ids` and
  `supporting_rule_ids` is checked against `issueEvidence.allowedNodeIDs`
  — the exact set this package itself resolved from the case's real tree
  for that issue (not a separate, later `GetNode` round trip per ID,
  since the evidence set was already read from the same `GetTree` call
  the prompt was built from).
- Any ID not in that set is stripped from the `Argument` and recorded in
  `FabricatedNodeIDs`; the `Argument.Grounded` field is set to `false` so
  a downstream consumer or human reviewer can immediately see which
  arguments needed correction.
- An argument left with **zero** real supporting IDs after stripping is
  dropped entirely — a "claim" with no tree-backed evidence at all is
  indistinguishable from an unsupported assertion, so it is not
  meaningful to keep as an `Argument`. The issue it belonged to is
  recorded in `ArgumentSet.SkippedIssueNodeIDs` instead of silently
  vanishing from the result.
- A partially fabricated argument (some real IDs, some invented) is kept
  — `Grounded=false` plus a non-empty `FabricatedNodeIDs` is a strictly
  more useful signal to a downstream reviewer than discarding an
  otherwise-legitimate argument over one bad reference.

This mirrors `packages/citation`'s own anti-hallucination philosophy
(`citation.Verify` independently confirms a cited node exists before
trusting it) applied one layer earlier, at the fact/rule-linkage level
rather than the citation-text level.

## Strength scoring

`score.go`'s `strengthScore` combines three `[0,1]` signals into a single
`Argument.Strength`:

- **Citation verification** (weight 0.4): the mean `ConfidenceScore`
  across the argument's resolved `Citations` for which `Verified` is
  true. An argument with rules but zero verified citations scores zero on
  this axis alone.
- **Fact confidence** (weight 0.35): the mean `Confidence` of the
  argument's `SupportingFactIDs`' underlying `FactNode`s — facts the tree
  itself trusts more heavily count for more.
- **Rule-linkage richness** (weight 0.25): how many distinct
  `SupportingRuleIDs` back the argument, saturating at 3 (mirroring
  `packages/issueagent/rank.go`'s own small-saturating-count convention
  rather than an unbounded linear signal).

An argument with no supporting rules at all (so no citations were even
attempted) redistributes the citation weight proportionally across the
other two signals, so a fact-only argument still receives a meaningful,
non-zero-by-construction score rather than being penalized purely for
having nothing to cite a rule for.

## Prompt template

`packages/firstpartyagent/templates` registers a single template,
`firstpartyagent.argument.construction`, with `NonBindingLabel: true`, via
its own `init()`:

```go
import _ "github.com/YASSERRMD/verdex/packages/firstpartyagent/templates"
```

It follows `packages/issueagent/templates`'s exact registration
convention (package-local template, shared `prompts.DefaultRegistry`,
`WithRegistry` escape hatch) and jurisdiction-aware selection
(`prompts.VariantSelector{}.SelectBest`, tiered fallback: exact ->
locale-only -> family-only -> universal).

## Output shape

```go
type PartyID string // opaque, caller-defined — no dependency on packages/timeline

type CitationRef struct {
    NodeID             string
    Citation           string
    VerificationStatus string
    Verified           bool
    ConfidenceScore    float64
}

type Argument struct {
    ID                string
    IssueNodeID       string   // the irac.IssueNode.ID this argument addresses
    PartyID           PartyID
    Claim             string
    SupportingFactIDs []string // guaranteed to exist in the case's tree
    SupportingRuleIDs []string // guaranteed to exist in the case's tree
    Citations         []CitationRef
    Counterarguments  []string
    Strength          float64  // [0,1]
    Grounded          bool     // false if any cited ID had to be stripped
    FabricatedNodeIDs []string // what was stripped, for transparency
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

agent, err := firstpartyagent.New(api, firstpartyagent.PartyID("party-plaintiff"), issueResult.Issues,
    firstpartyagent.WithJurisdictionName("California"),
    firstpartyagent.WithLegalFamily("common_law"),
    firstpartyagent.WithPartyLabel("the plaintiff"),
)

result, runResult, err := firstpartyagent.Argue(ctx, agent, caseID, firstpartyagent.ArgueConfig{
    Router:   myRouter, // *router.Router — required
    Budget:   agentframework.DefaultBudget(),
    Seed:     agentframework.Seed{},
    TenantID: tenantID,
})
// result is a typed firstpartyagent.ArgumentSet, feeding Phase 052's
// second-party (rebuttal) agent and Phase 055's synthesis agent.
```

A caller driving an `agentframework.Runner` directly (e.g. to compose
this agent alongside `issueagent` and a future second-party agent in one
orchestration loop, Phase 059) can recover the same typed result from
`Result.FinalText` via `firstpartyagent.DecodeResult`.

## What this package deliberately does not do

- It does not frame or rank issues — that is `packages/issueagent`'s job,
  performed before this package ever runs. This package treats its input
  `[]issueagent.FramedIssue` as already correct and complete.
- It does not construct, mutate, or persist any `irac` tree node or edge.
  Its output is consumed data for a downstream agent (Phase 052) or the
  synthesis agent (Phase 055), never written back into the case's
  reasoning tree.
- It does not model parties, roles, or standing itself — `PartyID` is an
  intentionally opaque local string, never a hard dependency on
  `packages/timeline` or any other participant/role package.
- It does not argue the opposing side, generate a rebuttal to its own
  counterarguments, or reconcile multiple parties' arguments — that is
  Phase 052's (second-party/rebuttal agent) and Phase 055's (synthesis
  agent) job respectively. This package's `Counterarguments` field is
  deliberately a flat list of anticipated rebuttals, not a fully argued
  opposing case.
- It does not talk to a model provider directly, retry across providers,
  or implement a fallback chain — that is `packages/router`'s job.
- It does not enforce the non-binding guardrail beyond the
  `NonBindingLabel` disclaimer `packages/prompts` appends to the rendered
  prompt text — Phase 057's project-wide guardrail-enforcement layer is
  responsible for anything stronger applied to this agent's output.
