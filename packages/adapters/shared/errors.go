package shared

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// anthropicErrorBody is the subset of the Anthropic error envelope we need.
type anthropicErrorBody struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// openaiErrorBody is the subset of the OpenAI error envelope we need.
type openaiErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"` // string or int
	} `json:"error"`
}

// geminiErrorBody is the subset of the Gemini REST error envelope we need.
type geminiErrorBody struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// MapHTTPStatus converts an HTTP status code and raw response body into a
// typed *provider.ProviderError. It attempts to parse provider-specific error
// envelopes for richer codes and messages; on failure it falls back to generic
// descriptions derived from the status code.
//
// Retryable conditions (429, 503) are wrapped in a *RetryableError so that
// WithRetry can distinguish them.
func MapHTTPStatus(statusCode int, body []byte, providerID string) error {
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return nil
	}

	code, message := parseProviderError(body, providerID, statusCode)

	underlying := &provider.ProviderError{
		ProviderID: providerID,
		Code:       code,
		Message:    message,
	}

	// Wrap the sentinel so callers can errors.Is against provider sentinels.
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		underlying.Underlying = provider.ErrInvalidRequest
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		underlying.Underlying = provider.ErrInvalidRequest
	case http.StatusRequestEntityTooLarge:
		underlying.Underlying = provider.ErrContextExceeded
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		underlying.Underlying = provider.ErrProviderUnavailable
		return &RetryableError{Err: underlying}
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout:
		underlying.Underlying = provider.ErrProviderUnavailable
		return &RetryableError{Err: underlying}
	default:
		underlying.Underlying = provider.ErrProviderUnavailable
	}

	return underlying
}

func parseProviderError(body []byte, providerID string, statusCode int) (code, message string) {
	switch providerID {
	case "anthropic":
		var env anthropicErrorBody
		if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
			return env.Error.Type, env.Error.Message
		}
	case "openai":
		var env openaiErrorBody
		if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
			code := fmt.Sprintf("%v", env.Error.Code)
			if code == "<nil>" || code == "" {
				code = env.Error.Type
			}
			return code, env.Error.Message
		}
	case "gemini":
		var env geminiErrorBody
		if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
			return env.Error.Status, env.Error.Message
		}
	}
	// Generic fallback.
	return fmt.Sprintf("http_%d", statusCode), http.StatusText(statusCode)
}
