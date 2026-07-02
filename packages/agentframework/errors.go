package agentframework

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilRouter is returned when a Runner is constructed with a nil
	// *router.Router. Every model call must go through a Router — this
	// package never constructs a provider.LLMProvider itself.
	ErrNilRouter = errors.New("agentframework: router must not be nil")

	// ErrNilAgent is returned when a Runner is constructed with a nil
	// Agent.
	ErrNilAgent = errors.New("agentframework: agent must not be nil")

	// ErrEmptyCaseID is returned when a Run is started, or a Scratchpad
	// constructed, with an empty CaseID.
	ErrEmptyCaseID = errors.New("agentframework: case id is required")

	// ErrInvalidBudget is returned when a Budget's fields cannot produce
	// a usable run (e.g. a negative step count).
	ErrInvalidBudget = errors.New("agentframework: budget is invalid")

	// ErrBudgetExhausted is returned when a run terminates because it
	// exceeded its Budget (steps, wall-clock, or tokens) before the Agent
	// concluded naturally. Callers can distinguish this from a natural
	// conclusion via Result.Termination or by testing the returned error
	// with errors.Is.
	ErrBudgetExhausted = errors.New("agentframework: step budget exhausted")

	// ErrToolNotFound is returned when a model turn requests a tool name
	// that is not registered on the Runner's ToolSet.
	ErrToolNotFound = errors.New("agentframework: tool not found")

	// ErrDuplicateTool is returned when a ToolSet is asked to register two
	// tools with the same Name.
	ErrDuplicateTool = errors.New("agentframework: duplicate tool name")

	// ErrToolInvocation wraps an error returned by a Tool's Invoke
	// function, distinguishing a tool-execution failure from a model-call
	// failure or malformed model output.
	ErrToolInvocation = errors.New("agentframework: tool invocation failed")

	// ErrModelCall wraps an error returned by the Router while attempting
	// a model call, distinguishing it from a tool failure.
	ErrModelCall = errors.New("agentframework: model call failed")

	// ErrMalformedOutput is returned when a model turn's output cannot be
	// interpreted by the Agent (e.g. a tool call the Agent cannot parse).
	ErrMalformedOutput = errors.New("agentframework: malformed model output")

	// ErrStepTimeout is returned when a single step exceeds its configured
	// per-step timeout.
	ErrStepTimeout = errors.New("agentframework: step timed out")

	// ErrNilTool is returned when a ToolSet is asked to register a nil
	// Tool, or a Tool with an empty Name.
	ErrNilTool = errors.New("agentframework: tool must not be nil or unnamed")

	// ErrNilKnowledgeAPI is returned when a KnowledgeAPI-backed tool
	// constructor is called with a nil *knowledgeapi.KnowledgeAPI.
	ErrNilKnowledgeAPI = errors.New("agentframework: knowledge api must not be nil")
)
