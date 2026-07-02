package reasoningorchestration

import (
	"sync"
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
)

// StageTelemetry is a single stage's operational record: how long it
// took, whether it errored, and — for the three LLM-agent stages
// (StageIssueFraming, StageFirstPartyArguments, StageSecondPartyArguments,
// StageSynthesis) — the underlying agentframework.Stats snapshot from
// that stage's Runner.Run call. The three deterministic stages
// (StageEvidenceWeighing, StageLawApplication, StageUncertaintySurfacing)
// and StageGuardrailCheck leave AgentStats at its zero value: they never
// drive an agentframework.Runner, so there is no model-call/tool-call
// telemetry to aggregate for them, only a plain duration.
type StageTelemetry struct {
	// Stage identifies which stage this record describes.
	Stage Stage

	// Duration is the wall-clock time this stage took, from just before
	// its entrypoint was called to just after it returned (excluding any
	// checkpoint persistence, which is fire-and-forget — see doc/
	// reasoning-orchestration.md's concurrency section).
	Duration time.Duration

	// AgentStats is the agentframework.Stats snapshot from this stage's
	// underlying Runner.Run call, populated only for the three
	// LLM-agent-backed stages. Zero value for deterministic stages.
	AgentStats agentframework.Stats

	// Err is the error this stage returned, if any. Nil for a
	// successfully completed stage.
	Err error
}

// Stats is a point-in-time snapshot of a pipeline run's telemetry across
// every stage executed so far, mirroring treeindex/adaptiveretrieval's
// package-local stats convention (see agentframework.Stats's own doc
// comment for the precedent this follows).
type Stats struct {
	// PerStage is one StageTelemetry per stage attempted so far, in
	// execution order.
	PerStage []StageTelemetry

	// TotalWallClock is the sum of every PerStage entry's Duration.
	TotalWallClock time.Duration
}

// telemetryRecorder accumulates per-stage StageTelemetry under a mutex
// during a run, mirroring agentframework's own unexported
// telemetryRecorder convention: a fresh recorder per Run/Resume call so
// concurrent runs for different cases never share state, and a Stats
// snapshot returned to a caller can never be mutated out from under a
// live run.
type telemetryRecorder struct {
	mu     sync.Mutex
	stages []StageTelemetry
}

// newTelemetryRecorder constructs an empty telemetryRecorder.
func newTelemetryRecorder() *telemetryRecorder {
	return &telemetryRecorder{}
}

// record appends entry to the recorder's stage list. Safe for concurrent
// use, though a single Run/Resume call only ever calls this sequentially
// from its own goroutine (the pipeline's stages are sequential by
// dependency, see doc/reasoning-orchestration.md).
func (r *telemetryRecorder) record(entry StageTelemetry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stages = append(r.stages, entry)
}

// snapshot returns a Stats value summarizing every StageTelemetry
// recorded so far. The returned slice is a defensive copy.
func (r *telemetryRecorder) snapshot() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]StageTelemetry, len(r.stages))
	copy(out, r.stages)

	var total time.Duration
	for _, s := range out {
		total += s.Duration
	}

	return Stats{PerStage: out, TotalWallClock: total}
}
