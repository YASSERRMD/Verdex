# Issue-analysis agent (`packages/issueagent`)

Phase 050 is the first concrete agent built on `packages/agentframework`
(Phase 049). Its job in Part 5 — Reasoning & Adversarial Synthesis — is to
take a case's already-assembled reasoning tree and produce a structured,
ranked, jurisdiction-aware framing of its legal issues, for Phase
051/052's first-party and second-party argument agents and Phase 055's
synthesis agent to consume as their own starting context.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/issue` (Phase 033) | Deterministic, rule-based **extraction** of candidate legal issues from raw case facts/segments into `irac.IssueNode`s, before a case's tree is assembled. | Upstream and different in kind. `packages/issueagent` never re-derives issue extraction and never runs before tree assembly — it reads `IssueNode`s that already exist in an assembled tree. If a case has no `IssueNode`s yet, that is `packages/issue`'s (and `packages/treeassembly`'s) job to fix, not this package's — see `ErrNoIssueNodes`. |
| `packages/agentframework` (Phase 049) | `Agent` interface, `Runner`/`Config`, `Scratchpad`, `Budget`, `Stats`, `Seed`, `NewKnowledgeAPIToolSet`. | `issueagent.Agent` implements `agentframework.Agent`. Every model call is driven by an `agentframework.Runner`, constructed either directly or via this package's `Analyze` convenience function. No step loop, budget enforcement, or telemetry logic is reimplemented here. |
| `packages/knowledgeapi` | `KnowledgeAPI` facade — `GetTree`, `LookupPaths`, `Retrieve`, `GetNode`, `ResolveCitation`, `ValidationStatus`. | The only way this package touches a case's tree. `fetch.go`'s `fetchIssueContexts` calls `GetTree` once per run and resolves governing-rule linkage from the returned edge list locally — it does not import `packages/graph`, `packages/treeindex`, or `packages/traversal` directly. |
| `packages/irac` | `IssueNode`, `RuleNode`, `NodeType`, `EdgeType` (in particular `EdgeGoverns`: `Rule --governs--> Issue`). | Read-only. This package never constructs an `IssueNode`, `RuleNode`, or any tree edge — its own output type, `FramedIssue`, references an existing `IssueNode`'s ID by string instead of embedding or duplicating the node. |
| `packages/prompts` | `PromptTemplate`, `Registry`, `Render`, `VariantSelector.SelectBest`, the mandatory non-binding disclaimer. | This package's own issue-framing template(s) live under `packages/issueagent/templates` (package-local, not added to the shared `packages/prompts/templates`) but register into the single shared `prompts.DefaultRegistry` — see "Prompt template" below for why. |
| `packages/router` / `packages/provider` | Model-agnostic routing, `ChatRequest`/`ChatResponse`, `TaskType`. | `issueagent.Agent.TaskType()` returns `provider.TaskReason`. Every model call is dispatched through a caller-supplied `*router.Router`; this package never imports `packages/adapters` or talks to an `LLMProvider` directly. |

## What this agent does

For a given case:

1. **Pulls every `IssueNode`** in the case's tree via one
   `knowledgeapi.KnowledgeAPI.GetTree` call, along with every governing
   `RuleNode` linked to each issue (`Rule --governs--> Issue` edges,
   resolved locally from the returned edge list — `LookupPaths` cannot be
   used for this because it only walks forward from a start node, and the
   `governs` edge points from the rule to the issue, not the other way).
2. **Renders one jurisdiction-aware framing prompt** covering every issue
   at once (see "Prompt template" below), and sends it through the
   `agentframework.Runner`'s single step.
3. **Parses the model's structured JSON response** (tolerating a
   markdown code fence or surrounding prose) into per-issue materiality
   scores, governing questions, ambiguity flags, and a confidence score.
4. **Blends the model's output with heuristic signals** computed purely
   from the tree (rule-linkage richness, upstream extraction confidence)
   so that an issue the model's response omits, or a genuinely
   malformed/partial response, still yields a sensible `FramedIssue`
   rather than dropping the issue or defaulting to zero.
5. **Ranks every issue by materiality** and assembles the case's
   `IssueAnalysisResult`.

This agent is a **single-step** agent by design: it does not iteratively
call tools mid-reasoning the way a first-party argument agent (Phase 051)
exploring a specific line of attack might. Framing needs the whole tree
up front, not incremental exploration, so `BuildRequest` gathers
everything once and `Interpret` always concludes on the first model turn.
A future revision that wants iterative exploration (e.g. following up on
a flagged ambiguity with a targeted `knowledgeapi`-backed tool call) is a
change to `Interpret`'s `Decision`, not a different framework — the
`agentframework.Runner` this agent runs on already supports multi-step
tool-calling loops.

## Why materiality ranking is heuristic-plus-model, not model-only

The model's `materiality_score` per issue is the dominant signal (weight
0.7 in `blendMateriality`), but a purely model-driven score has two
failure modes this design avoids:

- A malformed or partial model response (missing an issue entirely, or
  returning unparseable JSON for one entry) would otherwise leave that
  issue unranked or ranked at an arbitrary default.
- Ranking is more auditable when it is not a black box: the heuristic
  score (governing-rule linkage count, saturating at 3 rules, plus the
  issue's own upstream extraction confidence) gives every `FramedIssue` a
  deterministic floor and a transparent `RuleLinkageCount` field a
  downstream consumer can inspect independently of the model's stated
  reasoning.

The same shape (model-reported value blended with a rule-linkage
heuristic, weight 0.6 for confidence) is used for the `Confidence` field,
kept as a distinct axis from `MaterialityScore` — materiality is "how
important is this issue to the case", confidence is "how much should this
particular framing be trusted".

## Prompt template

`packages/issueagent/templates` registers a single template,
`issueagent.issue.framing`, with `NonBindingLabel: true` (so
`prompts.Render` always appends the mandatory non-binding disclaimer), via
its own `init()`:

```go
import _ "github.com/YASSERRMD/verdex/packages/issueagent/templates"
```

**Design choice — registration target.** `packages/prompts/templates` is
the home for templates shared across the whole platform (e.g.
`irac.issue.extraction`, used by `packages/issue` at ingestion time).
This agent's template is specific to its own reasoning task — framing
issues that already exist in the tree, not extracting new ones from raw
text — so it lives package-locally under `packages/issueagent/templates`
rather than being added to `packages/prompts/templates`. It still
registers into the single shared `prompts.DefaultRegistry` (rather than a
package-local `*prompts.Registry`) so that `prompts.VariantSelector{}.
SelectBest` and `prompts.DefaultRegistry.Latest` work uniformly across
every agent's templates, for any future prompt-management tooling that
lists "every registered template" from one place. `issueagent.New`
accepts a `WithRegistry` option for a caller that wants a fully isolated
registry instead (e.g. a test harness registering a competing template
version without mutating the shared default).

**Jurisdiction-aware selection.** `Agent.BuildRequest` resolves the
template via `prompts.VariantSelector{}.SelectBest(registry, "issueagent.
issue.framing", locale, legalFamily)`, using the agent's configured
`WithLocale`/`WithLegalFamily` options. `SelectBest`'s tiered fallback
(exact match, locale-only, family-only, universal) means a future
locale- or legal-family-specific template variant is picked up
automatically once registered at a higher version, with no change to
`agent.go`.

## Output shape

```go
type FramedIssue struct {
    SourceIssueNodeID  string   // the irac.IssueNode.ID this framing describes
    Question           string   // the issue's text, carried forward from the tree
    MaterialityRank    int      // 1-based, 1 = most material
    MaterialityScore   float64  // [0,1], the score MaterialityRank was derived from
    GoverningQuestions []string // the specific legal question(s) this issue raises
    Ambiguities        []string // thin/missing rule linkage, low confidence, etc.
    Confidence         float64  // [0,1], how much to trust this framing
    RuleLinkageCount   int      // transparency signal behind Confidence/Ambiguities
}

type IssueAnalysisResult struct {
    CaseID           string
    JurisdictionCode string        // from WithJurisdictionCode, if set
    LegalFamily      string        // from WithLegalFamily, if set
    Issues           []FramedIssue // sorted by MaterialityRank ascending
    GeneratedAt      time.Time
}
```

`FramedIssue` deliberately references `SourceIssueNodeID` by string
rather than embedding or wrapping the `irac.IssueNode` itself: this
package never constructs a new tree node, and its output is not written
back into the reasoning tree. A downstream agent that needs the full node
(e.g. its `Provenance` or `Spans`) fetches it via `knowledgeapi.
KnowledgeAPI.GetNode` using `SourceIssueNodeID`.

## Usage

```go
agent, err := issueagent.New(api,
    issueagent.WithJurisdictionCode("US-CA"),
    issueagent.WithJurisdictionName("California"),
    issueagent.WithLegalFamily("common_law"),
)

result, runResult, err := issueagent.Analyze(ctx, agent, caseID, issueagent.AnalyzeConfig{
    Router:   myRouter, // *router.Router — required
    Budget:   agentframework.DefaultBudget(),
    Seed:     agentframework.Seed{},
    TenantID: tenantID,
})
// result is a typed issueagent.IssueAnalysisResult.
// runResult is the underlying agentframework.Result, for telemetry/Scratchpad inspection.
```

A caller driving an `agentframework.Runner` directly (e.g. to compose
this agent alongside others in one orchestration loop, Phase 059) can
recover the same typed result from `Result.FinalText` via
`issueagent.DecodeResult`.

## What this package deliberately does not do

- It does not extract new issues from raw case facts, segments, claims,
  or timelines — that is `packages/issue`'s job, performed before the
  tree this package reads even exists.
- It does not construct, mutate, or persist any `irac` tree node or edge.
  Its output is consumed data for downstream agents, never written back
  into the case's reasoning tree.
- It does not perform citation resolution or tree-integrity validation
  itself — `knowledgeapi.KnowledgeAPI.ResolveCitation` and
  `ValidationStatus` exist for that, and a future revision of this agent
  (or a caller composing it with other `agentframework`-backed tools) can
  call them without this package reimplementing either.
- It does not talk to a model provider directly, retry across providers,
  or implement a fallback chain — that is `packages/router`'s job. This
  package only ever declares `provider.TaskReason` and lets a
  caller-supplied `*router.Router` decide the rest.
- It does not enforce the non-binding guardrail beyond the
  `NonBindingLabel` disclaimer `packages/prompts` appends to the rendered
  prompt text — Phase 057's project-wide guardrail-enforcement layer is
  responsible for anything stronger applied to this agent's output.
