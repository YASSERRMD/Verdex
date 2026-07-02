package agentframework

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// Agent supplies the domain-specific parts of a tool-using reasoning loop:
// how to build the next model request from the run's history so far, and
// how to decide whether a model turn concludes the run or requires another
// step. Runner owns everything else — the loop itself, tool dispatch,
// budget enforcement, timeouts, and telemetry.
//
// Implementations are expected to be stateless (or hold only immutable
// configuration such as a prompts.Registry reference); all per-run state
// belongs on the Scratchpad that Runner threads through every call.
type Agent interface {
	// Name identifies the agent for telemetry and error messages (e.g.
	// "issue-agent", "first-party-argument-agent").
	Name() string

	// TaskType declares which provider.TaskType this agent's model calls
	// should be routed as (typically provider.TaskReason for multi-step
	// reasoning agents). Runner passes this straight to
	// router.Router.Chat's routing decision; it never hardcodes a task
	// type itself.
	TaskType() provider.TaskType

	// BuildRequest constructs the next provider.ChatRequest given the
	// run's Scratchpad so far. Called once per step, before any tool
	// dispatch for that step. Implementations typically render a
	// prompts.Template (via prompts.Latest/prompts.Render) using the
	// Scratchpad's accumulated Steps as context.
	BuildRequest(ctx context.Context, pad *Scratchpad) (provider.ChatRequest, error)

	// Interpret inspects a model's ChatResponse for the current step and
	// decides what happens next: conclude the run, invoke one or more
	// tools, or continue to another model turn. Returning a non-nil
	// []ToolCall causes Runner to invoke each named tool (in order) via
	// the Runner's ToolSet and record the results as Observations before
	// the next BuildRequest call.
	//
	// Interpret returning ErrMalformedOutput (or an error wrapping it)
	// causes Runner to classify the step as a malformed-output failure
	// rather than a model-call failure.
	Interpret(ctx context.Context, pad *Scratchpad, resp *provider.ChatResponse) (Decision, error)
}

// Decision is Agent.Interpret's verdict on a single model turn.
type Decision struct {
	// Conclude, when true, ends the run naturally after this step.
	// FinalText is used as Result.FinalText. Mutually exclusive with a
	// non-empty ToolCalls (Runner treats Conclude as taking precedence if
	// both are set).
	Conclude bool

	// FinalText is the agent's concluding output, meaningful only when
	// Conclude is true.
	FinalText string

	// ToolCalls, when non-empty, are dispatched in order via the
	// Runner's ToolSet before the next step. Each result is recorded as
	// an Observation on the Scratchpad.
	ToolCalls []ToolCall
}

// ToolCall names a single tool invocation an Agent's Interpret step
// requested.
type ToolCall struct {
	// Name must match a Tool.Name registered on the Runner's ToolSet.
	Name string

	// Args are passed verbatim to the Tool's Invoke function.
	Args map[string]any
}

// TerminationReason classifies why a Run stopped, distinct from any error
// value returned alongside it.
type TerminationReason string

const (
	// TerminationConcluded means the Agent's Interpret step returned
	// Decision.Conclude naturally, within budget.
	TerminationConcluded TerminationReason = "concluded"

	// TerminationBudgetExhausted means the run stopped because a Budget
	// limit (steps, wall-clock, or tokens) was reached before the Agent
	// concluded naturally. This is a graceful termination, not an error
	// condition the caller must treat as a failure — Result.Termination
	// combined with a partial Scratchpad is often still useful.
	TerminationBudgetExhausted TerminationReason = "budget_exhausted"

	// TerminationError means the run stopped because of an unrecoverable
	// error (model-call failure, tool failure, malformed output, or
	// step timeout) that the Agent could not route around.
	TerminationError TerminationReason = "error"
)

// Result is the structured outcome of a single Runner.Run call.
//
// Result is deliberately plain data (no behavior) so that a later
// guardrail-enforcement layer (Phase 057) or an orchestration pipeline
// (Phase 059) can wrap FinalText — or any Observation on the Scratchpad —
// in an irac.ConclusionNode without this package needing to know about
// irac at all.
type Result struct {
	// CaseID is the case this run was scoped to.
	CaseID string

	// AgentName is Agent.Name(), captured at run start.
	AgentName string

	// Termination classifies why the run stopped.
	Termination TerminationReason

	// FinalText is the agent's concluding output. Populated when
	// Termination is TerminationConcluded; may be empty (with a partial
	// Scratchpad still available) for the other termination reasons.
	FinalText string

	// Scratchpad is the full step-by-step record of the run, always
	// populated regardless of Termination.
	Scratchpad *Scratchpad

	// Telemetry is a snapshot of this run's Stats.
	Telemetry Stats

	// Err holds the underlying error when Termination is
	// TerminationError or TerminationBudgetExhausted (wrapping
	// ErrBudgetExhausted). Nil when Termination is TerminationConcluded.
	Err error
}
