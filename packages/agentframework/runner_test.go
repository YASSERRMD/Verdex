package agentframework_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/provider"
)

func TestNewRunner_NilRouter_ReturnsError(t *testing.T) {
	agent := newScriptedAgent("a")
	_, err := agentframework.NewRunner(agentframework.Config{Agent: agent})
	if !errors.Is(err, agentframework.ErrNilRouter) {
		t.Fatalf("NewRunner() error = %v, want ErrNilRouter", err)
	}
}

func TestNewRunner_NilAgent_ReturnsError(t *testing.T) {
	r := newTestRouter(t, nil)
	_, err := agentframework.NewRunner(agentframework.Config{Router: r})
	if !errors.Is(err, agentframework.ErrNilAgent) {
		t.Fatalf("NewRunner() error = %v, want ErrNilAgent", err)
	}
}

func TestRun_EmptyCaseID_ReturnsError(t *testing.T) {
	r := newTestRouter(t, nil)
	agent := newScriptedAgent("a", agentframework.Decision{Conclude: true, FinalText: "ok"})
	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	_, err = runner.Run(context.Background(), "")
	if !errors.Is(err, agentframework.ErrEmptyCaseID) {
		t.Fatalf("Run() error = %v, want ErrEmptyCaseID", err)
	}
}

func TestRun_ConcludesNaturally_SingleStep(t *testing.T) {
	r := newTestRouter(t, nil)
	agent := newScriptedAgent("issue-agent", agentframework.Decision{Conclude: true, FinalText: "the issue is X"})
	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationConcluded)
	}
	if result.FinalText != "the issue is X" {
		t.Fatalf("FinalText = %q, want %q", result.FinalText, "the issue is X")
	}
	if result.Scratchpad.StepCount() != 1 {
		t.Fatalf("StepCount() = %d, want 1", result.Scratchpad.StepCount())
	}
	if result.Telemetry.StepsTaken != 1 {
		t.Fatalf("Telemetry.StepsTaken = %d, want 1", result.Telemetry.StepsTaken)
	}
	if result.Telemetry.ModelCalls != 1 {
		t.Fatalf("Telemetry.ModelCalls = %d, want 1", result.Telemetry.ModelCalls)
	}
	if result.Telemetry.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Telemetry.Termination = %q, want %q", result.Telemetry.Termination, agentframework.TerminationConcluded)
	}
}

func TestRun_MultiStepToolCalling(t *testing.T) {
	api := newTestKnowledgeAPI(t)
	tools, err := agentframework.NewKnowledgeAPIToolSet(api)
	if err != nil {
		t.Fatalf("NewKnowledgeAPIToolSet: %v", err)
	}

	r := newTestRouter(t, nil)
	agent := newScriptedAgent("first-party-agent",
		agentframework.Decision{ToolCalls: []agentframework.ToolCall{
			{Name: agentframework.ToolGetNode, Args: map[string]any{"node_id": "issue-1"}},
		}},
		agentframework.Decision{ToolCalls: []agentframework.ToolCall{
			{Name: agentframework.ToolValidationStatus, Args: map[string]any{}},
		}},
		agentframework.Decision{Conclude: true, FinalText: "strongest argument assembled"},
	)

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router: r,
		Agent:  agent,
		Tools:  tools,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(authedContext(), testCaseID)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Termination != agentframework.TerminationConcluded {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationConcluded)
	}
	if result.Scratchpad.StepCount() != 3 {
		t.Fatalf("StepCount() = %d, want 3", result.Scratchpad.StepCount())
	}
	obs := result.Scratchpad.Observations()
	if len(obs) != 2 {
		t.Fatalf("Observations() count = %d, want 2", len(obs))
	}
	if obs[0].Err != nil {
		t.Fatalf("Observations()[0].Err = %v, want nil", obs[0].Err)
	}
	if obs[0].Result.Data == nil {
		t.Fatalf("Observations()[0].Result.Data is nil, want a GetNodeResponse")
	}
	if result.Telemetry.ToolCallsMade != 2 {
		t.Fatalf("Telemetry.ToolCallsMade = %d, want 2", result.Telemetry.ToolCallsMade)
	}
	if result.Telemetry.ToolCallErrors != 0 {
		t.Fatalf("Telemetry.ToolCallErrors = %d, want 0", result.Telemetry.ToolCallErrors)
	}
	if agent.buildCalls != 3 || agent.interpretCalls != 3 {
		t.Fatalf("buildCalls=%d interpretCalls=%d, want 3/3", agent.buildCalls, agent.interpretCalls)
	}
}

func TestRun_ToolNotFound_TerminatesWithError(t *testing.T) {
	r := newTestRouter(t, nil)
	agent := newScriptedAgent("a", agentframework.Decision{
		ToolCalls: []agentframework.ToolCall{{Name: "does_not_exist"}},
	})
	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !errors.Is(err, agentframework.ErrToolNotFound) {
		t.Fatalf("Run() error = %v, want wrapping ErrToolNotFound", err)
	}
	if result.Termination != agentframework.TerminationError {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationError)
	}
}

func TestRun_BudgetExhausted_MaxSteps(t *testing.T) {
	r := newTestRouter(t, nil)
	// Agent never concludes; script only has "continue" decisions that
	// run out, at which point the default (Conclude) would kick in, so
	// give it far more non-concluding decisions than the budget allows.
	decisions := make([]agentframework.Decision, 0, 10)
	for i := 0; i < 10; i++ {
		decisions = append(decisions, agentframework.Decision{})
	}
	agent := newScriptedAgent("looping-agent", decisions...)

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router: r,
		Agent:  agent,
		Budget: agentframework.Budget{MaxSteps: 3},
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if !errors.Is(err, agentframework.ErrBudgetExhausted) {
		t.Fatalf("Run() error = %v, want ErrBudgetExhausted", err)
	}
	if result.Termination != agentframework.TerminationBudgetExhausted {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationBudgetExhausted)
	}
	if result.Scratchpad.StepCount() != 3 {
		t.Fatalf("StepCount() = %d, want 3", result.Scratchpad.StepCount())
	}
	if result.Telemetry.Termination != agentframework.TerminationBudgetExhausted {
		t.Fatalf("Telemetry.Termination = %q, want %q", result.Telemetry.Termination, agentframework.TerminationBudgetExhausted)
	}
}

func TestRun_BudgetExhausted_WallClock(t *testing.T) {
	slow := &provider.NoOpProvider{SimulatedLatency: 50 * time.Millisecond}
	r := newTestRouter(t, slow)

	decisions := make([]agentframework.Decision, 0, 10)
	for i := 0; i < 10; i++ {
		decisions = append(decisions, agentframework.Decision{})
	}
	agent := newScriptedAgent("slow-agent", decisions...)

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router: r,
		Agent:  agent,
		Budget: agentframework.Budget{MaxSteps: 100, MaxWallClock: 120 * time.Millisecond, StepTimeout: time.Second},
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if !errors.Is(err, agentframework.ErrBudgetExhausted) {
		t.Fatalf("Run() error = %v, want ErrBudgetExhausted", err)
	}
	if result.Termination != agentframework.TerminationBudgetExhausted {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationBudgetExhausted)
	}
	if result.Scratchpad.StepCount() >= 10 {
		t.Fatalf("StepCount() = %d, want fewer than the full 10 scripted steps", result.Scratchpad.StepCount())
	}
}

func TestRun_StepTimeout_ClassifiedAsTimeout(t *testing.T) {
	slow := &provider.NoOpProvider{SimulatedLatency: 200 * time.Millisecond}
	r := newTestRouter(t, slow)
	agent := newScriptedAgent("timeout-agent", agentframework.Decision{Conclude: true, FinalText: "done"})

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router: r,
		Agent:  agent,
		Budget: agentframework.Budget{StepTimeout: 20 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if !errors.Is(err, agentframework.ErrStepTimeout) {
		t.Fatalf("Run() error = %v, want ErrStepTimeout", err)
	}
	if result.Termination != agentframework.TerminationError {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationError)
	}
}

func TestRun_MalformedOutput_FromInterpret(t *testing.T) {
	r := newTestRouter(t, nil)
	agent := &scriptedAgent{name: "bad-agent", taskType: provider.TaskReason, interpretErr: errors.New("cannot parse tool call")}

	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if !errors.Is(err, agentframework.ErrMalformedOutput) {
		t.Fatalf("Run() error = %v, want ErrMalformedOutput", err)
	}
	if result.Termination != agentframework.TerminationError {
		t.Fatalf("Termination = %q, want %q", result.Termination, agentframework.TerminationError)
	}
}

func TestRun_MalformedOutput_FromBuildRequest(t *testing.T) {
	r := newTestRouter(t, nil)
	agent := &scriptedAgent{name: "bad-build-agent", taskType: provider.TaskReason, buildErr: errors.New("cannot render prompt")}

	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	_, err = runner.Run(context.Background(), testCaseID)
	if !errors.Is(err, agentframework.ErrMalformedOutput) {
		t.Fatalf("Run() error = %v, want ErrMalformedOutput", err)
	}
}

// capturingProvider records every ChatRequest it receives so tests can
// assert on Metadata threading (deterministic seed) and TaskType-derived
// routing behavior.
type capturingProvider struct {
	*provider.NoOpProvider
	requests []provider.ChatRequest
}

func newCapturingProvider() *capturingProvider {
	return &capturingProvider{NoOpProvider: provider.DefaultNoOpProvider()}
}

func (c *capturingProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	c.requests = append(c.requests, req)
	return c.NoOpProvider.Chat(ctx, req)
}

func TestRun_DeterministicSeed_ThreadsIntoMetadata(t *testing.T) {
	recorder := newCapturingProvider()
	r := newTestRouter(t, recorder)
	agent := newScriptedAgent("seeded-agent", agentframework.Decision{Conclude: true, FinalText: "ok"})

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router: r,
		Agent:  agent,
		Seed:   agentframework.NewSeed(42),
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	if _, err := runner.Run(context.Background(), testCaseID); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(recorder.requests) != 1 {
		t.Fatalf("captured %d requests, want 1", len(recorder.requests))
	}
	md := recorder.requests[0].Metadata
	if md[agentframework.SeedMetadataKey] != "42" {
		t.Fatalf("Metadata[%q] = %q, want %q", agentframework.SeedMetadataKey, md[agentframework.SeedMetadataKey], "42")
	}
	if md[agentframework.DeterministicMetadataKey] != "true" {
		t.Fatalf("Metadata[%q] = %q, want %q", agentframework.DeterministicMetadataKey, md[agentframework.DeterministicMetadataKey], "true")
	}
}

func TestRun_NoSeed_MetadataUnset(t *testing.T) {
	recorder := newCapturingProvider()
	r := newTestRouter(t, recorder)
	agent := newScriptedAgent("unseeded-agent", agentframework.Decision{Conclude: true, FinalText: "ok"})

	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if _, err := runner.Run(context.Background(), testCaseID); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(recorder.requests) != 1 {
		t.Fatalf("captured %d requests, want 1", len(recorder.requests))
	}
	if _, ok := recorder.requests[0].Metadata[agentframework.SeedMetadataKey]; ok {
		t.Fatalf("Metadata contains %q, want absent", agentframework.SeedMetadataKey)
	}
}

func TestRun_DeterministicOnly_SetsFlagWithoutSeedValue(t *testing.T) {
	recorder := newCapturingProvider()
	r := newTestRouter(t, recorder)
	agent := newScriptedAgent("deterministic-only-agent", agentframework.Decision{Conclude: true, FinalText: "ok"})

	runner, err := agentframework.NewRunner(agentframework.Config{
		Router: r,
		Agent:  agent,
		Seed:   agentframework.DeterministicOnly(),
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if _, err := runner.Run(context.Background(), testCaseID); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	md := recorder.requests[0].Metadata
	if md[agentframework.DeterministicMetadataKey] != "true" {
		t.Fatalf("Metadata[%q] = %q, want %q", agentframework.DeterministicMetadataKey, md[agentframework.DeterministicMetadataKey], "true")
	}
	if _, ok := md[agentframework.SeedMetadataKey]; ok {
		t.Fatalf("Metadata contains %q, want absent when no explicit seed value given", agentframework.SeedMetadataKey)
	}
}

func TestRun_TelemetryCountsModelCallErrors(t *testing.T) {
	failing := &failingProvider{}
	r := newTestRouter(t, failing)
	agent := newScriptedAgent("failing-agent", agentframework.Decision{Conclude: true, FinalText: "unreached"})

	runner, err := agentframework.NewRunner(agentframework.Config{Router: r, Agent: agent})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	result, err := runner.Run(context.Background(), testCaseID)
	if !errors.Is(err, agentframework.ErrModelCall) {
		t.Fatalf("Run() error = %v, want ErrModelCall", err)
	}
	if result.Telemetry.ModelCalls != 1 || result.Telemetry.ModelCallErrors != 1 {
		t.Fatalf("Telemetry ModelCalls=%d ModelCallErrors=%d, want 1/1", result.Telemetry.ModelCalls, result.Telemetry.ModelCallErrors)
	}
}

// failingProvider always fails Chat, to exercise Runner's model-call
// error classification path.
type failingProvider struct{ provider.NoOpProvider }

func (f *failingProvider) ID() string { return "failing" }
func (f *failingProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	return nil, errors.New("upstream unavailable")
}
