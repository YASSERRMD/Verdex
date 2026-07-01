package eval

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// EvalRunner orchestrates evaluation runs across one or more providers.
//
// It is safe for concurrent use; multiple goroutines may call RunAll or Run
// simultaneously as long as the underlying providers are thread-safe.
type EvalRunner struct {
	providers []provider.LLMProvider
	golden    *GoldenSet
}

// NewEvalRunner creates an EvalRunner that will evaluate providers against the
// tasks in golden.
//
// providers must contain at least one entry; golden must not be nil.
func NewEvalRunner(providers []provider.LLMProvider, golden *GoldenSet) *EvalRunner {
	return &EvalRunner{
		providers: providers,
		golden:    golden,
	}
}

// RunAll executes every task in the GoldenSet against every registered
// provider and returns a flat slice of EvalResults.
//
// Execution is sequential (task-major, provider-minor) for reproducibility.
// If ctx is cancelled the function returns immediately with whatever results
// have been collected so far and the context error.
func (r *EvalRunner) RunAll(ctx context.Context) ([]EvalResult, error) {
	if r.golden == nil || len(r.golden.Tasks) == 0 {
		return nil, ErrNoGoldenSet
	}
	if len(r.providers) == 0 {
		return nil, fmt.Errorf("%w: no providers registered", ErrEvalFailed)
	}

	var results []EvalResult

	for _, task := range r.golden.Tasks {
		for _, p := range r.providers {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			default:
			}

			res, err := r.Run(ctx, p.ID(), task)
			if err != nil {
				// Record a zero-scored result so the report is complete.
				results = append(results, EvalResult{
					TaskID:     task.ID,
					ProviderID: p.ID(),
					ModelID:    p.Capabilities().ModelID,
					Score:      0,
					Rubric:     map[string]float64{"error": 0},
					Output:     fmt.Sprintf("ERROR: %v", err),
					EvalAt:     time.Now().UTC(),
				})
				continue
			}
			results = append(results, *res)
		}
	}
	return results, nil
}

// Run executes a single task against the provider identified by providerID and
// returns an EvalResult.
//
// Temperature is always set to 0 for determinism regardless of the task Seed.
// If providerID is not found among the registered providers, Run returns an
// error wrapping ErrEvalFailed.
func (r *EvalRunner) Run(ctx context.Context, providerID string, task EvalTask) (*EvalResult, error) {
	p := r.findProvider(providerID)
	if p == nil {
		return nil, fmt.Errorf("%w: provider %q not registered", ErrEvalFailed, providerID)
	}

	req := provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: task.Prompt},
		},
		Temperature: 0, // deterministic; seed has no effect on temperature-0 calls
		MaxTokens:   2048,
		Metadata: map[string]string{
			"eval_task_id": task.ID,
			"eval_seed":    fmt.Sprintf("%d", task.Seed),
		},
	}

	start := time.Now()
	resp, err := p.Chat(ctx, req)
	latency := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("%w: provider %q task %q: %v", ErrEvalFailed, providerID, task.ID, err)
	}

	score, rubric := applyRubric(resp.Content, task.GoldenAnswer, task.ScoringRubric)

	return &EvalResult{
		TaskID:       task.ID,
		ProviderID:   providerID,
		ModelID:      p.Capabilities().ModelID,
		Score:        score,
		Rubric:       rubric,
		Output:       resp.Content,
		Latency:      latency,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		EvalAt:       time.Now().UTC(),
	}, nil
}

// findProvider returns the provider with the given ID, or nil if not found.
func (r *EvalRunner) findProvider(id string) provider.LLMProvider {
	for _, p := range r.providers {
		if p.ID() == id {
			return p
		}
	}
	return nil
}
