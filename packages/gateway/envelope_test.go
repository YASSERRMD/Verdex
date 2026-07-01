package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YASSERRMD/verdex/packages/gateway"
)

type samplePayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestOK_statusAndEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	rec := httptest.NewRecorder()

	// Simulate VersionMiddleware having run.
	gateway.VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := samplePayload{ID: "abc", Name: "Test Case"}
		gateway.OK(w, r, payload)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var env gateway.Response[samplePayload]
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if env.Status != "success" {
		t.Errorf("expected status=success, got %q", env.Status)
	}
	if env.Version != "v1" {
		t.Errorf("expected version=v1, got %q", env.Version)
	}
	if env.Data.ID != "abc" {
		t.Errorf("expected data.id=abc, got %q", env.Data.ID)
	}
	if env.Data.Name != "Test Case" {
		t.Errorf("expected data.name='Test Case', got %q", env.Data.Name)
	}
}

func TestOK_contentTypeJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	rec := httptest.NewRecorder()

	gateway.VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gateway.OK(w, r, struct{}{})
	})).ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
}

func TestErr_statusAndEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cases/missing", nil)
	rec := httptest.NewRecorder()

	gateway.VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gateway.Err(w, r, &gateway.APIError{
			Code:    gateway.ErrCodeNotFound,
			Message: "case not found",
		})
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	var env gateway.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if env.Status != "error" {
		t.Errorf("expected status=error, got %q", env.Status)
	}
	if env.Code != gateway.ErrCodeNotFound {
		t.Errorf("expected code=%s, got %q", gateway.ErrCodeNotFound, env.Code)
	}
	if env.Message != "case not found" {
		t.Errorf("unexpected message: %q", env.Message)
	}
}

func TestErrWithDetails_includesDetails(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/cases", nil)
	rec := httptest.NewRecorder()

	details := []string{"name: is required", "tenant_id: is required"}

	gateway.VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gateway.ErrWithDetails(w, r, &gateway.APIError{
			Code:    gateway.ErrCodeBadRequest,
			Message: "request validation failed",
		}, details)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var env gateway.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(env.Details) != 2 {
		t.Errorf("expected 2 details, got %d", len(env.Details))
	}
}

func TestOKWithMeta_paginationMeta(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	rec := httptest.NewRecorder()

	meta := &gateway.PaginationMeta{Page: 2, PerPage: 10, Total: 55, TotalPages: 6}

	gateway.VersionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gateway.OKWithMeta(w, r, []samplePayload{}, meta)
	})).ServeHTTP(rec, req)

	var env struct {
		Meta *gateway.PaginationMeta `json:"meta"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if env.Meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if env.Meta.Page != 2 || env.Meta.TotalPages != 6 {
		t.Errorf("unexpected meta: %+v", env.Meta)
	}
}

func TestRequestIDMiddleware_propagatesID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	req.Header.Set("X-Request-ID", "test-req-id-123")
	rec := httptest.NewRecorder()

	var capturedID string
	gateway.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = gateway.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if capturedID != "test-req-id-123" {
		t.Errorf("expected request ID 'test-req-id-123', got %q", capturedID)
	}
	if rec.Header().Get("X-Request-ID") != "test-req-id-123" {
		t.Errorf("expected X-Request-ID response header to be 'test-req-id-123'")
	}
}

func TestRequestIDMiddleware_generatesID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	rec := httptest.NewRecorder()

	var capturedID string
	gateway.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = gateway.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("expected a generated request ID, got empty string")
	}
	if rec.Header().Get("X-Request-ID") != capturedID {
		t.Error("X-Request-ID response header should match generated ID")
	}
}
