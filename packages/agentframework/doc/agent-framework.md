# Reasoning agent framework (`packages/agentframework`)

Phase 049 opens Part 5 — Reasoning & Adversarial Synthesis — with a single
base framework every reasoning agent in Phases 050-060 (issue-analysis,
first-party and second-party argument agents, evidence weighing, law
application, synthesis, uncertainty surfacing, the non-binding guardrail,
jurisdiction-parameterized reasoning, orchestration, and the reasoning
trace) is expected to build on. Its job is narrow but load-bearing: own
the step loop, the tool-calling plumbing, the case-scoped memory, the
budget/timeout enforcement, and the telemetry, so that no downstream
phase reimplements any of the five.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/router` | `Router.Chat`/`Router.Embed`, task-type routing, fallback chains, circuit breakers. | The only path a `Runner` uses to reach a model. `agentframework` never imports `packages/provider`'s `LLMProvider` implementations or `packages/adapters` directly — every call is a `provider.ChatRequest` sent through a caller-supplied `*router.Router`. |
| `packages/provider` | `TaskType`, `ChatRequest`/`ChatResponse`, `TokenUsage`. | Consumed only for its request/response types and `TaskType` enum. `Agent.TaskType()` declares which `provider.TaskType` (typically `provider.TaskReason`) a `Runner` should route as; `Runner` never hardcodes one. |
| `packages/knowledgeapi` | `KnowledgeAPI` facade over tree reads, hybrid retrieval, citation resolution, and validation status. | `NewKnowledgeAPIToolSet` wraps every read method behind a `Tool` — no retrieval, citation, or validation logic is re-derived here. |
| `packages/prompts` | `PromptTemplate`, `DefaultRegistry`, `Render`, jurisdiction/locale variant selection, mandatory non-binding disclaimer injection. | Not imported by this package directly (an `Agent` implementation is expected to use `prompts.DefaultRegistry.Latest`/`prompts.Render` inside its own `BuildRequest`), but `Config`/`Agent` are shaped so a `prompts`-driven `BuildRequest` is the obvious, unforced way to build a request — see the worked example below. |
| `packages/irac` | `Node`, `NewConclusionNode`, `DraftAnalysisLabel`, `ContainsVerdictLanguage`. | Not imported. `Result.FinalText` is deliberately plain data so Phase 057's guardrail-enforcement layer (or any earlier caller) can wrap it in an `irac.ConclusionNode` without this package needing to know `irac` exists. |
| `packages/knowledgeisolation` | `CaseScopedStore`/`CaseScopedVectorStore`, cross-case isolation. | Not imported directly; isolation is already enforced one layer down, inside whatever `KnowledgeAPI` a caller constructs. `Scratchpad` mirrors the same CaseID-keying convention purely so a caller inspecting several runs' Scratchpads at once cannot confuse one case's intermediate reasoning with another's. |

This package holds no case data of its own beyond what a single `Run`
accumulates on its `Scratchpad`. It does not persist anything.

## The `Agent` / `Runner` split

An `Agent` supplies only the domain-specific parts of a reasoning loop:

```go
type Agent interface {
    Name() string
    TaskType() provider.TaskType
    BuildRequest(ctx context.Context, pad *Scratchpad) (provider.ChatRequest, error)
    Interpret(ctx context.Context, pad *Scratchpad, resp *provider.ChatResponse) (Decision, error)
}
```

`Runner` owns everything else: the loop itself, tool dispatch, budget
enforcement, per-step timeouts, error classification, and telemetry.
This split exists so that Phase 050's issue-agent, Phase 051/052's
argument agents, and Phase 055's synthesis agent differ from each other
only in `BuildRequest`/`Interpret` — never in how a step budget is
enforced or how a tool call is dispatched.

```go
runner, err := agentframework.NewRunner(agentframework.Config{
    Router:   myRouter,        // *router.Router — required
    Agent:    myIssueAgent,    // implements Agent — required
    Tools:    myToolSet,       // *ToolSet — optional, defaults to empty
    Budget:   agentframework.DefaultBudget(),
    Seed:     agentframework.NewSeed(42), // optional, for reproducible eval runs
    TenantID: tenantID,
})
result, err := runner.Run(ctx, caseID)
```

`Run` never panics on a bad step, a tool failure, or a blown budget. It
always returns a `Result` with a populated `Scratchpad` (even on
failure) and a `Termination` classifying why the run stopped:

- `TerminationConcluded` — the `Agent`'s `Interpret` returned
  `Decision{Conclude: true}` naturally.
- `TerminationBudgetExhausted` — a `Budget` limit was hit first. This is
  a graceful outcome, not necessarily a failure: a caller may still want
  `Result.Scratchpad`'s partial reasoning.
- `TerminationError` — a model call, tool call, or malformed output
  could not be recovered from. `Result.Err` distinguishes which via
  `errors.Is` against `ErrModelCall`, `ErrToolInvocation`,
  `ErrMalformedOutput`, `ErrStepTimeout`, or `ErrToolNotFound`.

## One step, end to end

Each iteration of `Runner.Run`'s loop is one `Step`:

1. `Agent.BuildRequest(ctx, pad)` builds a `provider.ChatRequest` from
   the `Scratchpad`'s history so far.
2. `Runner` attaches `Config.Seed`'s deterministic-mode metadata (see
   below) and calls `Config.Router.Chat(ctx, tenantID, req)`.
3. `Agent.Interpret(ctx, pad, resp)` decides: conclude, or dispatch one
   or more `ToolCall`s from `Decision.ToolCalls`.
4. Each `ToolCall` is dispatched against `Config.Tools`, and every
   result (or error) is recorded as an `Observation`.
5. The completed `Step` — request, response, decision, observations,
   any error, and start/end timestamps — is appended to the
   `Scratchpad`, and the loop checks the `Budget` before continuing.

## Tool-calling over `KnowledgeAPI`

`NewKnowledgeAPIToolSet` is the sanctioned way an agent built on this
framework touches case knowledge:

```go
tools, err := agentframework.NewKnowledgeAPIToolSet(myKnowledgeAPI)
```

It registers five tools, each a thin translation layer over one
`knowledgeapi.KnowledgeAPI` method — no retrieval, citation, or
validation logic is reimplemented:

| Tool name (constant) | Wraps |
|---|---|
| `ToolSearchCaseKnowledge` | `KnowledgeAPI.Retrieve` (fused semantic + structural search) |
| `ToolGetNode` | `KnowledgeAPI.GetNode` |
| `ToolLookupPaths` | `KnowledgeAPI.LookupPaths` |
| `ToolResolveCitation` | `KnowledgeAPI.ResolveCitation` |
| `ToolValidationStatus` | `KnowledgeAPI.ValidationStatus` |

A downstream `Agent`'s `Interpret` requests these by name via
`Decision.ToolCalls`:

```go
return agentframework.Decision{
    ToolCalls: []agentframework.ToolCall{
        {Name: agentframework.ToolGetNode, Args: map[string]any{"node_id": "issue-1"}},
    },
}, nil
```

A downstream package that needs a *different* capability (e.g. a future
evidence-weighing tool in Phase 053) should define its own `Tool` value
and register it on the same `ToolSet` passed into `Config.Tools`, rather
than extending this package — `Tool` is deliberately a plain struct with
an `Invoke` closure so any package can produce one without importing
`agentframework`'s internals.

## Scratchpad: case-scoped memory across steps

A `Scratchpad` is constructed fresh per `Run`, keyed by the same `CaseID`
string every case-isolation-aware package in Verdex uses:

```go
pad, err := agentframework.NewScratchpad(caseID, tenantID)
```

It accumulates two things as a run progresses:

- **`Steps()`** — the full ordered history of every model call, decision,
  and tool observation. A later orchestration pipeline (Phase 059) can
  inspect this to reconstruct exactly what an agent did.
- **`Notes()`** — a free-form `map[string]string` an `Agent`'s
  `BuildRequest` can use to carry forward a running summary between
  steps, instead of re-serializing the entire `Steps()` history into
  every prompt (useful once a run has taken several steps and the full
  history would blow a context window).

`Observations()` flattens every tool result across every step, in call
order, which is usually what a synthesis-style agent (Phase 055) wants
to inspect rather than walking `Steps()` itself.

## Budget: graceful termination, not a panic

```go
budget := agentframework.Budget{
    MaxSteps:     10,
    MaxWallClock: 90 * time.Second,
    StepTimeout:  20 * time.Second,
    MaxTokens:    50_000, // best-effort; see field doc for why
}
```

Every field is independently enforced; the zero value of any field falls
back to a documented default (`DefaultMaxSteps`, `DefaultMaxWallClock`,
`DefaultStepTimeout`) via `DefaultBudget()`. `MaxTokens` is deliberately
best-effort: token usage is only known after a step's `ChatResponse`
arrives, so a single step can still overshoot the ceiling before the next
iteration's check catches it — this mirrors how `provider.TokenUsage` is
reported (after the fact, not as a pre-flight estimate).

A step that times out because the *run's own* overall `MaxWallClock`
deadline elapsed (not because `StepTimeout` alone was too short) is
reported as `TerminationBudgetExhausted`, not `TerminationError` — the
distinction that matters to a caller is "did the agent run out of room"
versus "did something actually break."

## Telemetry

`Result.Telemetry` is a `Stats` snapshot: `StepsTaken`, `ToolCallsMade`,
`ToolCallErrors`, `ModelCalls`, `ModelCallErrors`, `TokensUsed`,
`WallClock`, and `Termination`. This follows the same package-local
`Stats`/`telemetryRecorder` split used by `packages/treeindex` and
`packages/adaptiveretrieval` — a mutex-guarded struct with an unexported
recorder, no shared observability interface. A caller that wants to
export these into a metrics system does so itself; this package has no
opinion on where telemetry goes, only that it is captured accurately.

## Deterministic-seed support

```go
Seed: agentframework.NewSeed(42),       // seed value + deterministic flag
Seed: agentframework.DeterministicOnly(), // deterministic flag only
```

`Runner` attaches `agentframework.seed` (the seed's base-10 string) and
`agentframework.deterministic` (`"true"`) to every `provider.ChatRequest`'s
`Metadata` map — the same `map[string]string` field every other
Metadata-consuming convention in Verdex uses. **Not every provider
adapter honors these keys.** A provider that ignores unrecognized
`Metadata` entries (the documented contract for that field) will simply
produce non-deterministic output regardless of `Seed`. Tests and eval
harnesses that need a hard reproducibility guarantee should pair `Seed`
with a fixed/mock provider (e.g. `provider.NoOpProvider`) rather than
relying on a real adapter's best-effort support.

## Worked example: wiring a `prompts`-driven `Agent`

A downstream agent's `BuildRequest` is expected to look roughly like
this (illustrative — Phase 050 owns the real issue-agent template):

```go
type issueAgent struct{}

func (issueAgent) Name() string                   { return "issue-agent" }
func (issueAgent) TaskType() provider.TaskType     { return provider.TaskReason }

func (issueAgent) BuildRequest(ctx context.Context, pad *agentframework.Scratchpad) (provider.ChatRequest, error) {
    tmpl, err := prompts.DefaultRegistry.Latest("irac.issue.framing", "en", "common_law")
    if err != nil {
        return provider.ChatRequest{}, err
    }
    body, err := prompts.Render(tmpl, map[string]string{
        "case_id": pad.CaseID(),
    })
    if err != nil {
        return provider.ChatRequest{}, err
    }
    return provider.ChatRequest{
        Messages: []provider.Message{{Role: "user", Content: body}},
    }, nil
}
```

Because `prompts.Render` already appends the mandatory non-binding
disclaimer for any `PromptTemplate` with `NonBindingLabel: true`,
downstream agents inherit that guardrail behavior for free simply by
routing their prompt construction through `packages/prompts` as shown
above — `agentframework` does not need to know this happened.

## Non-binding guardrail compatibility

This package does **not** enforce the non-binding guardrail itself —
Phase 057 does that project-wide. What it guarantees is that enforcing it
later is mechanical, not a retrofit:

- `Result.FinalText` is a plain string. Wrapping it in an
  `irac.ConclusionNode` via `irac.NewConclusionNode` (which
  unconditionally attaches `irac.DraftAnalysisLabel`) requires no
  restructuring.
- `Observation.Result.Content`/`.Data` on every `Scratchpad` entry are
  similarly plain, so a guardrail-wrapping layer can walk the full step
  history and label anything it finds, not just the final output.
- Nothing in this package emits verdict or directive language on its
  own; any such language would come from whatever `Agent` and prompt a
  downstream phase supplies, which is exactly what Phase 057's
  `irac.ContainsVerdictLanguage` check is for.

## What this package deliberately does not do

- It does not call a `provider.LLMProvider` or `packages/adapters`
  directly — every model call goes through a caller-supplied
  `*router.Router`.
- It does not implement retrieval, citation resolution, or tree
  validation logic — `tools_knowledgeapi.go` only translates tool-call
  args into `knowledgeapi` request DTOs and back.
- It does not render prompts itself — an `Agent`'s `BuildRequest` is
  expected to use `packages/prompts` directly when jurisdiction/locale
  variance or the non-binding disclaimer matters.
- It does not enforce the non-binding guardrail — that is Phase 057's
  job, project-wide; this package only avoids making that job awkward.
- It does not persist a `Scratchpad`, `Result`, or `Stats` anywhere —
  everything lives in memory for the duration of one `Run` and is handed
  back to the caller, who decides whether/how to persist it (e.g. into a
  future reasoning-trace store, Phase 060).
- It does not decide when an agent should stop reasoning in domain
  terms (e.g. "have enough issues been identified?") — that is
  `Agent.Interpret`'s job; `Runner` only enforces the mechanical
  `Budget` ceiling.
