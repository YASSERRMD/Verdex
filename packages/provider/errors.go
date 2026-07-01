package provider

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrProviderNotFound is returned by Registry.Get when no provider is
	// registered under the requested ID.
	ErrProviderNotFound = errors.New("provider not found")

	// ErrProviderUnavailable is returned when the provider's upstream
	// endpoint is unreachable or returning errors.
	ErrProviderUnavailable = errors.New("provider unavailable")

	// ErrContextExceeded is returned when the request exceeds the
	// provider's maximum context window.
	ErrContextExceeded = errors.New("context window exceeded")

	// ErrInvalidRequest is returned when the ChatRequest or EmbedRequest
	// contains invalid or missing fields.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrStreamInterrupted is returned (or sent as a StreamChunk with
	// Done=true) when a streaming response is cut short before the model
	// signals completion.
	ErrStreamInterrupted = errors.New("stream interrupted")
)

// ProviderError is a rich error value that carries structured context from a
// provider call failure.  Callers can unwrap it to test against sentinels:
//
//	var pe *ProviderError
//	if errors.As(err, &pe) { ... }
type ProviderError struct {
	// ProviderID is the ID() of the provider that produced this error.
	ProviderID string
	// Code is a provider-specific error code string (e.g. "rate_limit_error").
	Code string
	// Message is a human-readable description of the error.
	Message string
	// Underlying is the root cause, if any.
	Underlying error
}

// Error satisfies the error interface.
func (e *ProviderError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("provider %s [%s]: %s: %v", e.ProviderID, e.Code, e.Message, e.Underlying)
	}
	return fmt.Sprintf("provider %s [%s]: %s", e.ProviderID, e.Code, e.Message)
}

// Unwrap returns the underlying error so errors.Is / errors.As work on the
// cause chain.
func (e *ProviderError) Unwrap() error { return e.Underlying }

// newProviderError is a convenience constructor used by adapters.
func newProviderError(providerID, code, message string, underlying error) *ProviderError {
	return &ProviderError{
		ProviderID: providerID,
		Code:       code,
		Message:    message,
		Underlying: underlying,
	}
}
