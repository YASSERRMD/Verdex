package issueagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/router"
)

// AnalyzeConfig configures a single Analyze call: the routing dependency
// every model call is dispatched through, the Budget bounding the run,
// optional determinism settings, and the tenant the run is billed/routed
// under. Mirrors agentframework.Config's shape minus the parts (Agent,
// Tools) Analyze fixes itself.
type AnalyzeConfig struct {
	// Router is the *router.Router the agent's single model call is
	// dispatched through. Required.
	Router *router.Router

	// Budget bounds the run. Zero value uses agentframework's own
	// defaults (see agentframework.Budget.withDefaults) — ample for this
	// agent's single-step design.
	Budget agentframework.Budget

	// Seed configures deterministic-mode metadata for the run's model
	// call. Zero value (disabled) is valid.
	Seed agentframework.Seed

	// TenantID is passed through to router.Router.Chat's tenantID
	// argument.
	TenantID string
}

// Analyze runs the issue-framing Agent for caseID end to end: constructs
// an agentframework.Runner around agent and cfg, drives it to completion,
// and decodes the resulting IssueAnalysisResult from the run's
// FinalText. This is the sanctioned entrypoint for a caller that just
// wants a typed result — packages/issueagent's own package
// documentation and doc/issue-agent.md point Phase 051/052/055 consumers
// here rather than at agentframework.Runner directly.
//
// Returns the underlying agentframework error (e.g. ErrBudgetExhausted,
// ErrModelCall) unchanged when the run does not conclude naturally, and
// ErrMalformedModelOutput wrapped further if the run concluded but its
// FinalText could not be decoded (which should not happen for a run that
// reached TerminationConcluded, since Interpret only sets FinalText to
// its own successfully-encoded output).
func Analyze(ctx context.Context, agent *Agent, caseID string, cfg AnalyzeConfig) (IssueAnalysisResult, agentframework.Result, error) {
	if caseID == "" {
		return IssueAnalysisResult{}, agentframework.Result{}, ErrEmptyCaseID
	}

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router:   cfg.Router,
		Agent:    agent,
		Budget:   cfg.Budget,
		Seed:     cfg.Seed,
		TenantID: cfg.TenantID,
	})
	if err != nil {
		return IssueAnalysisResult{}, agentframework.Result{}, err
	}

	runResult, err := runner.Run(ctx, caseID)
	if err != nil {
		return IssueAnalysisResult{}, runResult, err
	}

	result, decodeErr := DecodeResult(runResult.FinalText)
	if decodeErr != nil {
		return IssueAnalysisResult{}, runResult, fmt.Errorf("%w: %v", ErrMalformedModelOutput, decodeErr)
	}
	return result, runResult, nil
}
