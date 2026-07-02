package synthesisagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/router"
)

// SynthesizeConfig configures a single Synthesize call: the routing
// dependency every model call is dispatched through, the Budget bounding
// the run, optional determinism settings, and the tenant the run is
// billed/routed under. Mirrors firstpartyagent.ArgueConfig's shape
// exactly.
type SynthesizeConfig struct {
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

// Synthesize runs the synthesis Agent for caseID end to end: constructs
// an agentframework.Runner around agent and cfg, drives it to
// completion, and decodes the resulting Opinion from the run's
// FinalText. This is the sanctioned entrypoint for a caller that just
// wants a typed result — mirroring firstpartyagent.Argue's
// convenience-entry convention exactly.
//
// Returns the underlying agentframework error (e.g. ErrBudgetExhausted,
// ErrModelCall) unchanged when the run does not conclude naturally, and
// ErrMalformedModelOutput wrapped further if the run concluded but its
// FinalText could not be decoded (which should not happen for a run that
// reached TerminationConcluded, since Interpret only sets FinalText to
// its own successfully-encoded output).
func Synthesize(ctx context.Context, agent *Agent, caseID string, cfg SynthesizeConfig) (Opinion, agentframework.Result, error) {
	if caseID == "" {
		return Opinion{}, agentframework.Result{}, ErrEmptyCaseID
	}

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router:   cfg.Router,
		Agent:    agent,
		Budget:   cfg.Budget,
		Seed:     cfg.Seed,
		TenantID: cfg.TenantID,
	})
	if err != nil {
		return Opinion{}, agentframework.Result{}, err
	}

	runResult, err := runner.Run(ctx, caseID)
	if err != nil {
		return Opinion{}, runResult, err
	}

	result, decodeErr := DecodeResult(runResult.FinalText)
	if decodeErr != nil {
		return Opinion{}, runResult, fmt.Errorf("%w: %v", ErrMalformedModelOutput, decodeErr)
	}
	return result, runResult, nil
}
