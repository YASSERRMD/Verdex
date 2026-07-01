// Package local provides a [provider.LLMProvider] adapter for local and
// self-hosted LLM servers that expose an OpenAI-compatible REST API (Ollama,
// LM Studio, vLLM, LocalAI, etc.).
package local

import "errors"

// ErrLocalEndpointDown is returned when the local endpoint is unreachable and
// OfflineMode is false. In offline-mode deployments where the endpoint is
// expected to be available, this signals a misconfiguration or service failure.
var ErrLocalEndpointDown = errors.New("local: endpoint is down or unreachable")

// ErrOfflineEndpointUnreachable is returned by DiscoverModels when the local
// /v1/models endpoint cannot be contacted, usually because the server has not
// started yet or the BaseURL is wrong.
var ErrOfflineEndpointUnreachable = errors.New("local: offline endpoint unreachable")

// ErrModelNotFound is returned when the requested model ID is not loaded in
// the local server. Typical cause: the GGUF file was not pulled before the
// request was made.
var ErrModelNotFound = errors.New("local: model not found on local server")

// ErrConcurrencyLimitExceeded is returned by AcquireSlot when the context is
// cancelled while waiting for a semaphore slot, indicating the server is
// saturated and no slot freed in time.
var ErrConcurrencyLimitExceeded = errors.New("local: concurrency limit exceeded")

// IsEndpointError reports whether err is one of the local-endpoint sentinel
// errors (ErrLocalEndpointDown or ErrOfflineEndpointUnreachable). This is a
// convenience helper for callers that want to distinguish connectivity problems
// from model-level errors without importing the errors package directly.
func IsEndpointError(err error) bool {
	return errors.Is(err, ErrLocalEndpointDown) || errors.Is(err, ErrOfflineEndpointUnreachable)
}
