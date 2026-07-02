package agentframework

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/provider"
	"github.com/YASSERRMD/verdex/packages/router"
)

// Config holds everything a Runner needs to drive an Agent through
// repeated steps: the model-routing dependency, the tools it may call,
// the budget bounding the run, and optional determinism settings.
//
// Config never accepts a provider.LLMProvider or an adapters value
// directly — Router is the only sanctioned path to a model call, per
// Verdex's model-agnostic-by-construction rule.
type Config struct {
	// Router is the *router.Router every model call is dispatched
	// through. Required.
	Router *router.Router

	// Agent supplies the domain-specific BuildRequest/Interpret behavior.
	// Required.
	Agent Agent

	// Tools is the ToolSet available for the Agent's Decision.ToolCalls
	// to dispatch against. May be nil (or empty), in which case any
	// ToolCall requested by the Agent fails with ErrToolNotFound.
	Tools *ToolSet

	// Budget bounds step count, wall-clock, per-step timeout, and
	// (best-effort) tokens for a single Run. Zero value is valid — see
	// DefaultBudget's field-level documentation for what each zero field
	// falls back to.
	Budget Budget

	// Seed configures deterministic-mode metadata attached to every
	// model call this Runner makes. Zero value (disabled) is valid.
	Seed Seed

	// TenantID is passed through to router.Router.Chat/Embed's tenantID
	// argument for every call this Runner makes.
	TenantID string
}

// Runner drives an Agent through a bounded sequence of steps: build a
// request, call the model via Router, interpret the response, optionally
// dispatch tool calls, record everything on a Scratchpad, and repeat
// until the Agent concludes or a Budget limit is reached.
//
// Runner owns the loop, tool dispatch, budget enforcement, per-step
// timeouts, error classification, and telemetry so that every agent built
// on this framework (Phases 050-056) shares one implementation of all
// five instead of each reimplementing its own.
type Runner struct {
	cfg Config
}

// NewRunner validates cfg and returns a ready-to-use Runner. Returns
// ErrNilRouter if cfg.Router is nil, or ErrNilAgent if cfg.Agent is nil.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.Router == nil {
		return nil, ErrNilRouter
	}
	if cfg.Agent == nil {
		return nil, ErrNilAgent
	}
	if cfg.Tools == nil {
		cfg.Tools = NewToolSet()
	}
	return &Runner{cfg: cfg}, nil
}

// Run drives cfg.Agent through steps scoped to caseID until it concludes
// naturally, its Budget is exhausted, or an unrecoverable error occurs.
// It never panics on budget exhaustion or a step/tool failure; those are
// reported via the returned Result's Termination and Err fields. A
// non-nil error is also returned whenever Termination is not
// TerminationConcluded, so callers that only check the error get correct
// success/failure behavior without inspecting Result.
func (r *Runner) Run(ctx context.Context, caseID string) (Result, error) {
	pad, err := NewScratchpad(caseID, r.cfg.TenantID)
	if err != nil {
		return Result{}, err
	}

	telemetry := newTelemetryRecorder()
	tracker := newBudgetTracker(r.cfg.Budget)
	runStart := time.Now()

	result := Result{
		CaseID:     caseID,
		AgentName:  r.cfg.Agent.Name(),
		Scratchpad: pad,
	}

	for {
		if exhausted, reason := tracker.exhausted(); exhausted {
			result.Termination = TerminationBudgetExhausted
			result.Err = fmt.Errorf("%w: %s", ErrBudgetExhausted, reason)
			telemetry.setTermination(TerminationBudgetExhausted)
			break
		}

		step, decision, stepErr := r.runStep(ctx, pad, tracker, telemetry)
		pad.AppendStep(step)
		telemetry.recordStep()
		tracker.recordStep(tokensFromResponse(step.Response))

		if stepErr != nil {
			// A step that failed because the run's own overall wall-clock
			// deadline (not its per-step timeout) elapsed mid-call is a
			// graceful budget exhaustion, not a step error: re-check the
			// budget now that time has passed, and prefer that
			// classification when it applies.
			if errors.Is(stepErr, ErrStepTimeout) {
				if exhausted, reason := tracker.exhausted(); exhausted {
					result.Termination = TerminationBudgetExhausted
					result.Err = fmt.Errorf("%w: %s", ErrBudgetExhausted, reason)
					telemetry.setTermination(TerminationBudgetExhausted)
					break
				}
			}
			result.Termination = TerminationError
			result.Err = stepErr
			telemetry.setTermination(TerminationError)
			break
		}

		if decision.Conclude {
			result.Termination = TerminationConcluded
			result.FinalText = decision.FinalText
			telemetry.setTermination(TerminationConcluded)
			break
		}
		// Otherwise: tool calls (if any) have already been dispatched and
		// recorded on the step; loop again for another BuildRequest.
	}

	telemetry.setWallClock(time.Since(runStart))
	result.Telemetry = telemetry.snapshot()
	return result, result.Err
}

// runStep executes exactly one step: build the request, call the model,
// interpret the response, and dispatch any resulting tool calls. It
// returns the completed Step (always non-nil fields populated as far as
// execution progressed) plus the Decision reached (zero value on
// failure) and a classified error, if any.
func (r *Runner) runStep(
	ctx context.Context,
	pad *Scratchpad,
	tracker *budgetTracker,
	telemetry *telemetryRecorder,
) (Step, Decision, error) {
	stepCtx, cancel := tracker.stepContext(ctx)
	defer cancel()

	step := Step{
		Index:     pad.StepCount(),
		StartedAt: time.Now(),
	}

	req, err := r.cfg.Agent.BuildRequest(stepCtx, pad)
	if err != nil {
		step.EndedAt = time.Now()
		step.Err = fmt.Errorf("%w: %v", ErrMalformedOutput, err)
		return step, Decision{}, step.Err
	}
	req.Metadata = r.cfg.Seed.applyTo(cloneMetadata(req.Metadata))
	step.Request = req

	resp, err := r.cfg.Router.Chat(stepCtx, r.cfg.TenantID, req)
	telemetry.recordModelCall(err, tokensFromResponse(resp))
	if err != nil {
		step.EndedAt = time.Now()
		step.Err = classifyModelError(err, stepCtx)
		return step, Decision{}, step.Err
	}
	step.Response = resp

	decision, err := r.cfg.Agent.Interpret(stepCtx, pad, resp)
	if err != nil {
		step.EndedAt = time.Now()
		step.Err = fmt.Errorf("%w: %v", ErrMalformedOutput, err)
		return step, Decision{}, step.Err
	}
	step.Decision = decision

	if !decision.Conclude && len(decision.ToolCalls) > 0 {
		observations, toolErr := r.dispatchToolCalls(stepCtx, decision.ToolCalls, telemetry)
		step.Observations = observations
		if toolErr != nil {
			step.EndedAt = time.Now()
			step.Err = toolErr
			return step, decision, toolErr
		}
	}

	step.EndedAt = time.Now()
	return step, decision, nil
}

// dispatchToolCalls invokes each call in order against r.cfg.Tools,
// recording an Observation per call. It stops and returns an error on
// the first tool failure, still returning the Observations gathered so
// far (including the failed one) so the partial record is not lost.
func (r *Runner) dispatchToolCalls(ctx context.Context, calls []ToolCall, telemetry *telemetryRecorder) ([]Observation, error) {
	observations := make([]Observation, 0, len(calls))
	for _, call := range calls {
		result, err := r.cfg.Tools.Invoke(ctx, call.Name, call.Args)
		telemetry.recordToolCall(err)
		observations = append(observations, Observation{Call: call, Result: result, Err: err})
		if err != nil {
			return observations, err
		}
	}
	return observations, nil
}

// classifyModelError wraps err as ErrStepTimeout when stepCtx's deadline
// was the cause, or ErrModelCall otherwise, so callers can distinguish a
// per-step timeout from any other model-call failure.
func classifyModelError(err error, stepCtx context.Context) error {
	if errors.Is(stepCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrStepTimeout, err)
	}
	return fmt.Errorf("%w: %v", ErrModelCall, err)
}

// tokensFromResponse extracts TotalTokens from resp, returning 0 for a
// nil response.
func tokensFromResponse(resp *provider.ChatResponse) int {
	if resp == nil {
		return 0
	}
	return resp.Usage.TotalTokens
}

// cloneMetadata returns a shallow copy of md so Seed.applyTo never
// mutates a map the Agent's BuildRequest may have retained a reference
// to.
func cloneMetadata(md map[string]string) map[string]string {
	if md == nil {
		return nil
	}
	out := make(map[string]string, len(md))
	for k, v := range md {
		out[k] = v
	}
	return out
}
