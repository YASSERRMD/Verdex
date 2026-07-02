package agentframework

import (
	"sync"
	"time"
)

// Stats is a point-in-time snapshot of a single Runner.Run's operational
// counters: how many steps ran, how many tool calls were made, how many
// tokens were consumed (where reported), how long the run took, and why
// it stopped. Mirrors the treeindex/adaptiveretrieval convention of a
// package-local telemetry struct rather than any shared observability
// interface.
type Stats struct {
	// StepsTaken is the number of steps the run completed (including a
	// final step that concluded the run or hit an error).
	StepsTaken int

	// ToolCallsMade is the total number of Tool.Invoke calls made across
	// every step.
	ToolCallsMade int

	// ToolCallErrors is the number of tool calls that returned an error.
	ToolCallErrors int

	// ModelCalls is the number of Router.Chat calls attempted.
	ModelCalls int

	// ModelCallErrors is the number of Router.Chat calls that returned an
	// error.
	ModelCallErrors int

	// TokensUsed is the cumulative provider.TokenUsage.TotalTokens
	// across every successful step, when the underlying provider reports
	// it (a zero value may mean either "no tokens used" or "not
	// reported" — providers are not required to populate TokenUsage).
	TokensUsed int

	// WallClock is the total duration of the run, from the first
	// BuildRequest call to termination.
	WallClock time.Duration

	// Termination classifies why the run stopped.
	Termination TerminationReason
}

// telemetryRecorder accumulates Stats counters under a mutex during a run.
// A Runner constructs one fresh telemetryRecorder per Run call so
// concurrent runs never share counters; it is not exported so a caller's
// Stats snapshot (via Result.Telemetry) can never be mutated out from
// under a completed run, mirroring adaptiveretrieval's
// telemetryRecorder/BuildTelemetry split.
type telemetryRecorder struct {
	mu    sync.Mutex
	stats Stats
}

func newTelemetryRecorder() *telemetryRecorder {
	return &telemetryRecorder{}
}

func (r *telemetryRecorder) recordStep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.StepsTaken++
}

func (r *telemetryRecorder) recordModelCall(err error, tokens int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.ModelCalls++
	if err != nil {
		r.stats.ModelCallErrors++
		return
	}
	r.stats.TokensUsed += tokens
}

func (r *telemetryRecorder) recordToolCall(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.ToolCallsMade++
	if err != nil {
		r.stats.ToolCallErrors++
	}
}

func (r *telemetryRecorder) setWallClock(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.WallClock = d
}

func (r *telemetryRecorder) setTermination(reason TerminationReason) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.Termination = reason
}

func (r *telemetryRecorder) snapshot() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}
