// Package provider defines the model-agnostic LLM provider abstraction used
// throughout the Verdex judicial reasoning platform.
//
// All LLM calls inside Verdex are routed through the LLMProvider interface
// defined in this package so that concrete provider adapters (Anthropic, OpenAI,
// Azure, etc.) can be swapped without touching business logic.
//
// Core concepts:
//
//   - LLMProvider: the interface every adapter must implement.
//   - Registry: a process-level map from provider IDs to LLMProvider instances.
//   - ChatRequest / ChatResponse: normalised request/response for chat completions.
//   - EmbedRequest / EmbedResponse: normalised request/response for text embeddings.
//   - TokenAccountingHook: optional side-channel for recording token usage.
//   - HookedProvider: wraps any LLMProvider to call a TokenAccountingHook.
//   - WithTimeout: wraps any LLMProvider to enforce per-call deadlines.
//   - NoOpProvider: a deterministic stub useful in tests and CI.
package provider
