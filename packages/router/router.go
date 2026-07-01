package router

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// Router dispatches LLM calls to the appropriate provider(s) using an ordered
// fallback chain, respects circuit breakers, and emits telemetry for every
// attempt.
type Router struct {
	registry   *provider.Registry
	policy     RoutingPolicy
	selector   ProviderSelector
	cbRegistry *CircuitBreakerRegistry
	telemetry  TelemetrySink
}

// Chat selects a provider chain for task type TaskChat and tenantID, then
// tries each provider in order until one succeeds.  Circuit-breaker open
// providers are skipped.  Telemetry is emitted per attempt.
func (r *Router) Chat(ctx context.Context, tenantID string, req provider.ChatRequest) (*provider.ChatResponse, error) {
	ids, err := r.selector.Select(ctx, TaskChat, tenantID)
	if err != nil {
		return nil, err
	}

	var lastErr error
	tried := 0

	for attempt, id := range ids {
		cb := r.cbRegistry.Get(id)
		if !cb.Allow() {
			// Circuit is open; skip without counting as an attempt.
			continue
		}

		tried++
		p, err := r.registry.Get(id)
		if err != nil {
			cb.Trip()
			r.telemetry.Record(RouterEvent{
				TaskType:   TaskChat,
				ProviderID: id,
				Success:    false,
				Latency:    0,
				Attempt:    attempt + 1,
				TenantID:   tenantID,
			})
			lastErr = err
			continue
		}

		start := time.Now()
		resp, err := p.Chat(ctx, req)
		latency := time.Since(start)

		if err != nil {
			cb.Trip()
			r.telemetry.Record(RouterEvent{
				TaskType:   TaskChat,
				ProviderID: id,
				Success:    false,
				Latency:    latency,
				Attempt:    attempt + 1,
				TenantID:   tenantID,
			})
			lastErr = err
			continue
		}

		cb.Reset()
		r.telemetry.Record(RouterEvent{
			TaskType:   TaskChat,
			ProviderID: id,
			Success:    true,
			Latency:    latency,
			Attempt:    attempt + 1,
			TenantID:   tenantID,
		})
		return resp, nil
	}

	if tried == 0 {
		return nil, ErrNoProvidersAvailable
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrAllProvidersFailed
}

// ChatStream selects a provider chain for task type TaskChat and tenantID,
// then tries each provider in order until one returns a stream channel.
func (r *Router) ChatStream(ctx context.Context, tenantID string, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	ids, err := r.selector.Select(ctx, TaskChat, tenantID)
	if err != nil {
		return nil, err
	}

	var lastErr error
	tried := 0

	for attempt, id := range ids {
		cb := r.cbRegistry.Get(id)
		if !cb.Allow() {
			continue
		}

		tried++
		p, err := r.registry.Get(id)
		if err != nil {
			cb.Trip()
			r.telemetry.Record(RouterEvent{
				TaskType:   TaskChat,
				ProviderID: id,
				Success:    false,
				Latency:    0,
				Attempt:    attempt + 1,
				TenantID:   tenantID,
			})
			lastErr = err
			continue
		}

		start := time.Now()
		ch, err := p.ChatStream(ctx, req)
		if err != nil {
			latency := time.Since(start)
			cb.Trip()
			r.telemetry.Record(RouterEvent{
				TaskType:   TaskChat,
				ProviderID: id,
				Success:    false,
				Latency:    latency,
				Attempt:    attempt + 1,
				TenantID:   tenantID,
			})
			lastErr = err
			continue
		}

		// Wrap the channel so we can emit a telemetry event when the stream
		// completes (or errors) and update the circuit breaker.
		wrapped := r.wrapStream(ctx, ch, id, tenantID, attempt+1, start, cb)
		return wrapped, nil
	}

	if tried == 0 {
		return nil, ErrNoProvidersAvailable
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrAllProvidersFailed
}

// wrapStream drains src in a goroutine, forwarding chunks to the returned
// channel.  On completion it updates the circuit breaker and emits telemetry.
func (r *Router) wrapStream(
	ctx context.Context,
	src <-chan provider.StreamChunk,
	providerID, tenantID string,
	attempt int,
	start time.Time,
	cb *CircuitBreaker,
) <-chan provider.StreamChunk {
	out := make(chan provider.StreamChunk, 32)
	go func() {
		defer close(out)
		success := true
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-src:
				if !ok {
					latency := time.Since(start)
					if success {
						cb.Reset()
					} else {
						cb.Trip()
					}
					r.telemetry.Record(RouterEvent{
						TaskType:   TaskChat,
						ProviderID: providerID,
						Success:    success,
						Latency:    latency,
						Attempt:    attempt,
						TenantID:   tenantID,
					})
					return
				}
				if chunk.Done && chunk.FinishReason == "" {
					success = false
				}
				out <- chunk
			}
		}
	}()
	return out
}

// Embed selects a provider chain for task type TaskEmbed and tenantID, then
// tries each provider in order until one succeeds.
func (r *Router) Embed(ctx context.Context, tenantID string, req provider.EmbedRequest) (*provider.EmbedResponse, error) {
	ids, err := r.selector.Select(ctx, TaskEmbed, tenantID)
	if err != nil {
		return nil, err
	}

	var lastErr error
	tried := 0

	for attempt, id := range ids {
		cb := r.cbRegistry.Get(id)
		if !cb.Allow() {
			continue
		}

		tried++
		p, err := r.registry.Get(id)
		if err != nil {
			cb.Trip()
			r.telemetry.Record(RouterEvent{
				TaskType:   TaskEmbed,
				ProviderID: id,
				Success:    false,
				Latency:    0,
				Attempt:    attempt + 1,
				TenantID:   tenantID,
			})
			lastErr = err
			continue
		}

		start := time.Now()
		resp, err := p.Embed(ctx, req)
		latency := time.Since(start)

		if err != nil {
			cb.Trip()
			r.telemetry.Record(RouterEvent{
				TaskType:   TaskEmbed,
				ProviderID: id,
				Success:    false,
				Latency:    latency,
				Attempt:    attempt + 1,
				TenantID:   tenantID,
			})
			lastErr = err
			continue
		}

		cb.Reset()
		r.telemetry.Record(RouterEvent{
			TaskType:   TaskEmbed,
			ProviderID: id,
			Success:    true,
			Latency:    latency,
			Attempt:    attempt + 1,
			TenantID:   tenantID,
		})
		return resp, nil
	}

	if tried == 0 {
		return nil, ErrNoProvidersAvailable
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrAllProvidersFailed
}
