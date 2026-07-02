// Package agentframework is the base framework every Part 5 reasoning
// agent (issue-analysis, first-party and second-party argument agents,
// evidence weighing, law application, synthesis, and the agents built in
// Phases 050-060) is expected to build on, instead of each phase
// reimplementing its own step loop, tool-calling plumbing, budget
// enforcement, and telemetry.
//
// # Scope
//
// This package defines the shape of a tool-using reasoning agent, not any
// specific agent's reasoning behavior:
//
//   - Agent / Config / Runner (agent.go): the lifecycle contract —
//     construct a Runner from a Config, call Run once per case-scoped
//     task, and receive a structured Result. An Agent supplies the
//     domain-specific parts (system prompt construction, deciding when to
//     stop, interpreting a model turn) while Runner owns the loop.
//   - Tool / ToolSet (tool.go): a name+description+JSON-schema-ish
//     parameter contract plus an Invoke function, so an Agent can declare
//     what it is allowed to call without hardcoding retrieval logic.
//   - KnowledgeAPI-backed tools (tools_knowledgeapi.go): concrete Tool
//     values wrapping knowledgeapi.KnowledgeAPI methods (tree read,
//     hybrid retrieval, citation resolution, validation status) — the
//     only sanctioned way an agent built on this framework touches case
//     knowledge.
//   - Scratchpad (scratchpad.go): a CaseID-scoped, append-only record of
//     every Step an agent took during one Run, so a caller (or a later
//     orchestration pipeline) can inspect what an agent did and learned
//     without re-deriving it from raw model output.
//   - Budget (budget.go): step count, wall-clock, and (best-effort) token
//     ceilings, enforced by Runner so a run terminates gracefully with a
//     distinct TerminationBudgetExhausted status instead of running away
//     or panicking.
//   - Stats / RunTelemetry (telemetry.go): package-local, mutex-guarded
//     counters (steps taken, tool calls made, tokens used, wall-clock,
//     termination reason) in the treeindex/adaptiveretrieval style — this
//     package does not import any shared observability interface.
//   - Seed (seed.go): a deterministic-mode flag threaded into every
//     provider.ChatRequest's Metadata for reproducible runs in tests and
//     eval harnesses. Not every provider honors it; see seed.go's doc
//     comment.
//
// # Model-agnostic by construction
//
// A Runner never talks to packages/provider or packages/adapters
// directly. Every model call is built as a provider.ChatRequest and sent
// through a caller-supplied *router.Router, which owns task-type
// routing, fallback chains, and circuit breakers. An Agent declares which
// provider.TaskType it wants (typically provider.TaskReason) via its
// Config; Runner never hardcodes one.
//
// # Non-binding guardrail compatibility
//
// This package does not itself enforce the non-binding guardrail — that
// is Phase 057's job, project-wide. Result is deliberately structured so
// wrapping its FinalText (or the per-Step Observations) in an
// irac.ConclusionNode via irac.NewConclusionNode, or attaching
// irac.DraftAnalysisLabel some other way, is a mechanical step for
// whatever wraps a Runner's output, not an awkward retrofit.
//
// See doc/agent-framework.md for the full model, worked examples of
// building a KnowledgeAPI-backed tool set, and guidance for how Part 5
// agents (Phases 050-056) are expected to depend on this package.
package agentframework
