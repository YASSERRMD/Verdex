# Provider Contract

This document describes the behavioural contract that every LLM provider adapter registered
in the Verdex `packages/provider` package must satisfy.

---

## Overview

All LLM calls in the Verdex judicial reasoning platform are routed through the `LLMProvider`
interface.  Concrete adapters (Anthropic, OpenAI, Azure OpenAI, etc.) are registered in the
`Registry` at application startup and resolved by ID at call sites.

---

## The LLMProvider Interface

```go
type LLMProvider interface {
    ID()           string
    Capabilities() Capability
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
    Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
    HealthCheck(ctx context.Context) error
}
```

### ID()

- MUST return a stable, non-empty string that uniquely identifies this adapter instance.
- MUST match the key used when calling `Registry.Register`.
- MUST equal `Capabilities().ProviderID`.

### Capabilities()

- MUST return a `Capability` value where `ProviderID == ID()`.
- `MaxContextTokens` MUST be positive.
- `SupportedTasks` SHOULD accurately list the task types the adapter handles.
- The returned value MUST NOT be mutated by callers.

### Chat()

- MUST return a non-nil `*ChatResponse` with non-empty `Content` on success.
- MUST honour `ctx.Done()` and return promptly when the context is cancelled.
- MUST wrap provider-specific errors in `ProviderError` so callers can inspect `Code`.
- SHOULD set `ChatResponse.Usage` with accurate token counts where the upstream API provides them.
- `FinishReason` SHOULD be set to a meaningful value (`"end_turn"`, `"max_tokens"`, `"stop_sequence"`, etc.).

### ChatStream()

- MUST return a readable channel that emits one or more `StreamChunk` values.
- The final chunk MUST have `Done == true`.  The channel MUST be closed after this chunk.
- If the stream is interrupted before completion, the last chunk SHOULD have `FinishReason == "error"` and `Done == true`.
- The goroutine driving the channel MUST stop and close the channel when `ctx` is cancelled.
- Callers MUST drain the channel to completion; failing to do so leaks the producer goroutine.

### Embed()

- MUST return a `*EmbedResponse` where `len(Embeddings) == len(req.Texts)`.
- `Dimensions` MUST equal `len(Embeddings[i])` for every i.
- Adapters that do not support embeddings MUST return `ErrProviderUnavailable`.
- Empty `req.Texts` MUST return `ErrInvalidRequest`.

### HealthCheck()

- MUST return `nil` when the upstream endpoint is healthy.
- MUST return a non-nil error (preferably wrapping `ErrProviderUnavailable`) when the upstream is unreachable.
- SHOULD complete quickly; callers typically impose their own deadline via context.

---

## Error Taxonomy

| Sentinel | When to return |
|---|---|
| `ErrProviderNotFound` | Registry lookup by ID fails |
| `ErrProviderUnavailable` | Upstream is unreachable; Embed not supported |
| `ErrContextExceeded` | Request exceeds the model's context window |
| `ErrInvalidRequest` | Missing or malformed fields in the request |
| `ErrStreamInterrupted` | Streaming response cut short before completion |

All errors from an adapter SHOULD wrap one of the above sentinels (via `ProviderError.Underlying`
or `fmt.Errorf("... %w", sentinel)`) so that callers can use `errors.Is`.

---

## Concurrency

- Every method MUST be safe to call from multiple goroutines simultaneously.
- Adapters that maintain internal state (token buckets, connection pools) MUST protect that
  state with appropriate synchronisation.

---

## Conformance Testing

Use `ProviderConformanceTest(t, p)` from `conformance_test.go` to verify that your adapter
meets this contract.  Add the following test to your adapter package:

```go
func TestMyAdapter_Conformance(t *testing.T) {
    p := NewMyAdapter( /* test config */ )
    provider.ProviderConformanceTest(t, p)
}
```

---

## Registration

Adapters are registered once at startup:

```go
reg := provider.DefaultRegistry
if err := reg.Register("anthropic", anthropicAdapter); err != nil {
    log.Fatal(err)
}

// Later, at call sites:
p, err := reg.Get("anthropic")
```

Use `WithTimeout` to enforce per-call deadlines and `HookedProvider` to attach token accounting:

```go
p = provider.WithTimeout(p, 30*time.Second, 10*time.Second)
p = provider.HookedProvider(p, myAccountingHook)
reg.Register("anthropic", p)
```

---

## Versioning

This contract is versioned alongside the `packages/provider` Go module.  Breaking changes to
the `LLMProvider` interface require a major version bump of the module.
