package gateway

import (
	"encoding/json"
	"net/http"
)

// PaginationMeta carries pagination information in a response envelope.
type PaginationMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Response is the standard success envelope returned by all Verdex API handlers.
// T is the type of the data payload.
type Response[T any] struct {
	Version   string          `json:"version"`
	Status    string          `json:"status"`
	Data      T               `json:"data"`
	Meta      *PaginationMeta `json:"meta,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

// ErrorResponse is the standard error envelope returned by all Verdex API handlers
// when a request cannot be fulfilled successfully.
type ErrorResponse struct {
	Version   string   `json:"version"`
	Status    string   `json:"status"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Details   []string `json:"details,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
}

// OK builds a 200 success response envelope and writes it to w.
func OK[T any](w http.ResponseWriter, r *http.Request, data T) {
	OKWithMeta(w, r, data, nil)
}

// OKWithMeta builds a 200 success response envelope with pagination metadata and
// writes it to w.
func OKWithMeta[T any](w http.ResponseWriter, r *http.Request, data T, meta *PaginationMeta) {
	env := Response[T]{
		Version:   VersionFromContext(r.Context()).String(),
		Status:    "success",
		Data:      data,
		Meta:      meta,
		RequestID: RequestIDFromContext(r.Context()),
	}
	writeJSON(w, http.StatusOK, env)
}

// Created builds a 201 response envelope and writes it to w.
func Created[T any](w http.ResponseWriter, r *http.Request, data T) {
	env := Response[T]{
		Version:   VersionFromContext(r.Context()).String(),
		Status:    "success",
		Data:      data,
		RequestID: RequestIDFromContext(r.Context()),
	}
	writeJSON(w, http.StatusCreated, env)
}

// Err builds an error response envelope from an APIError and writes it to w.
// The HTTP status code is derived from the error code via StatusCodeFor.
func Err(w http.ResponseWriter, r *http.Request, apiErr *APIError) {
	ErrWithDetails(w, r, apiErr, nil)
}

// ErrWithDetails builds an error response envelope with additional detail strings.
func ErrWithDetails(w http.ResponseWriter, r *http.Request, apiErr *APIError, details []string) {
	env := ErrorResponse{
		Version:   VersionFromContext(r.Context()).String(),
		Status:    "error",
		Code:      apiErr.Code,
		Message:   apiErr.Message,
		Details:   details,
		RequestID: RequestIDFromContext(r.Context()),
	}
	writeJSON(w, StatusCodeFor(apiErr.Code), env)
}

// writeJSON serialises v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
