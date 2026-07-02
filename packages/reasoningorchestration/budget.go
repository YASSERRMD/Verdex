package reasoningorchestration

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
)

// DefaultMaxTotalWallClock is the MaxTotalWallClock a zero-valued
// PipelineBudget falls back to: a generous ceiling across all eight
// stages, well above any single stage's own agentframework.Budget
// default (agentframework.DefaultMaxWallClock, 2 minutes per LLM-agent
// stage).
const DefaultMaxTotalWallClock = 15 * time.Minute

// PipelineBudget bounds the whole run, layered on top of (not replacing)
// each individual LLM-agent stage's own agentframework.Budget.
//
// The two budgets answer different questions: PerStageBudget bounds how
// long/how-many-steps a single agent call may take in isolation (the
// same bound issueagent.AnalyzeConfig.Budget,
// firstpartyagent.ArgueConfig.Budget, secondpartyagent.ArgueConfig.Budget,
// and synthesisagent.SynthesizeConfig.Budget already accept), while
// MaxTotalWallClock bounds the sum across every stage — LLM-backed and
// deterministic alike — so a case that keeps narrowly staying under each
// individual stage's budget cannot still run unboundedly long overall.
type PipelineBudget struct {
	// PerStageBudget is passed as the Budget field of every LLM-agent
	// stage's Config (AnalyzeConfig/ArgueConfig/SynthesizeConfig). Zero
	// value uses agentframework's own defaults (see
	// agentframework.Budget.withDefaults), identical to leaving Budget
	// unset when calling any Part-5 agent package directly.
	PerStageBudget agentframework.Budget

	// MaxTotalWallClock caps the pipeline's cumulative wall-clock spend
	// across every stage attempted so far (see Stats.TotalWallClock).
	// Checked before starting each stage — never mid-stage — so a run
	// halts gracefully between stages rather than cancelling a stage
	// partway through. Zero or negative falls back to
	// DefaultMaxTotalWallClock.
	MaxTotalWallClock time.Duration
}

// withDefaults returns a copy of b with a zero-or-negative
// MaxTotalWallClock replaced by DefaultMaxTotalWallClock.
func (b PipelineBudget) withDefaults() PipelineBudget {
	out := b
	if out.MaxTotalWallClock <= 0 {
		out.MaxTotalWallClock = DefaultMaxTotalWallClock
	}
	return out
}
