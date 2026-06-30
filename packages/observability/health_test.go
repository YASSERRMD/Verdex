package observability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessHandler_AlwaysOK(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	LivenessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
}

func TestReadinessHandler_AllHealthy(t *testing.T) {
	checkers := []NamedChecker{
		{Name: "db", Checker: func(_ context.Context) error { return nil }},
		{Name: "cache", Checker: func(_ context.Context) error { return nil }},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	ReadinessHandler(checkers...).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadinessHandler_OneFailing(t *testing.T) {
	checkers := []NamedChecker{
		{Name: "db", Checker: func(_ context.Context) error { return nil }},
		{Name: "downstream", Checker: func(_ context.Context) error { return errors.New("timeout") }},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	ReadinessHandler(checkers...).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Status != "unavailable" {
		t.Errorf("status = %q, want unavailable", body.Status)
	}
	if msg, ok := body.Failures["downstream"]; !ok || msg != "timeout" {
		t.Errorf("expected failures[downstream]=timeout, got %v", body.Failures)
	}
	if _, ok := body.Failures["db"]; ok {
		t.Errorf("did not expect db in failures: %v", body.Failures)
	}
}

func TestReadinessHandler_NoCheckers(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	ReadinessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when no checkers are registered", rec.Code)
	}
}

func TestReadinessHandler_NilCheckerSkipped(t *testing.T) {
	checkers := []NamedChecker{
		{Name: "broken-registration", Checker: nil},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	ReadinessHandler(checkers...).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when checker func is nil", rec.Code)
	}
}
