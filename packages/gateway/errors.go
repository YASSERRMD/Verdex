package gateway

import (
	"encoding/json"
	"net/http"
)

// Error code constants used in ErrorResponse.Code and APIError.Code.
const (
	ErrCodeBadRequest      = "BAD_REQUEST"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeForbidden       = "FORBIDDEN"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeTooManyRequests = "TOO_MANY_REQUESTS"
	ErrCodeInternal        = "INTERNAL_ERROR"
)

// APIError is a structured error type that carries a machine-readable code and a
// human-readable message. It implements the error interface.
type APIError struct {
	// Code is one of the ErrCode* constants.
	Code string
	// Message is a human-readable description of the error.
	Message string
	// Err is an optional underlying error (not exposed to API consumers).
	Err error
}

// Error satisfies the error interface.
func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap allows errors.Is / errors.As to inspect the wrapped error.
func (e *APIError) Unwrap() error { return e.Err }

// StatusCodeFor maps an ErrCode constant to the corresponding HTTP status code.
// Unknown codes map to 500 Internal Server Error.
func StatusCodeFor(code string) int {
	switch code {
	case ErrCodeBadRequest:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeTooManyRequests:
		return http.StatusTooManyRequests
	case ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// WriteError writes an error response to w. If err is an *APIError the error
// code and message are used directly; otherwise an INTERNAL_ERROR response is
// written.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	switch e := err.(type) {
	case *APIError:
		apiErr = e
	default:
		apiErr = &APIError{
			Code:    ErrCodeInternal,
			Message: "an unexpected error occurred",
			Err:     err,
		}
	}

	resp := ErrorResponse{
		Version: CurrentVersion.String(),
		Status:  "error",
		Code:    apiErr.Code,
		Message: apiErr.Message,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(StatusCodeFor(apiErr.Code))
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(resp)
}

// WriteErrorWithRequest is like WriteError but enriches the envelope with version
// and request-ID from the request context.
func WriteErrorWithRequest(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *APIError
	switch e := err.(type) {
	case *APIError:
		apiErr = e
	default:
		apiErr = &APIError{
			Code:    ErrCodeInternal,
			Message: "an unexpected error occurred",
			Err:     err,
		}
	}

	Err(w, r, apiErr)
}
