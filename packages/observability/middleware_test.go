package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCorrelationMiddleware_GeneratesWhenMissing proves that a request
// arriving with no X-Correlation-ID header is assigned a freshly
// generated one, which is then visible both on the response header and
// to the handler via the request context.
func TestCorrelationMiddleware_GeneratesWhenMissing(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	var sawID string
	handler := CorrelationMiddleware(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := CorrelationIDFromContext(r.Context())
		if !ok {
			t.Error("expected a correlation ID to be present on the request context")
		}
		sawID = id
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if sawID == "" {
		t.Fatal("expected a non-empty generated correlation ID")
	}

	headerID := rec.Header().Get(CorrelationIDHeader)
	if headerID != sawID {
		t.Errorf("response header correlation ID = %q, want %q (matching context value)", headerID, sawID)
	}
}

// TestCorrelationMiddleware_PropagatesExisting proves that an inbound
// X-Correlation-ID header is preserved unchanged end-to-end: on the
// request context and echoed back unmodified on the response header.
func TestCorrelationMiddleware_PropagatesExisting(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	const inboundID = "fixed-correlation-id-001"

	var sawID string
	handler := CorrelationMiddleware(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := CorrelationIDFromContext(r.Context())
		sawID = id
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set(CorrelationIDHeader, inboundID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if sawID != inboundID {
		t.Errorf("context correlation ID = %q, want unchanged %q", sawID, inboundID)
	}
	if got := rec.Header().Get(CorrelationIDHeader); got != inboundID {
		t.Errorf("response header correlation ID = %q, want unchanged %q", got, inboundID)
	}
}

// TestCorrelationMiddleware_LoggerCarriesCorrelationID proves that a
// Logger obtained from the request context (via FromContext) attaches
// the correlation ID to every emitted log record, by capturing the
// JSON log output into an in-memory buffer and asserting on the
// decoded field.
func TestCorrelationMiddleware_LoggerCarriesCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	base := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	const inboundID = "log-carrying-id-002"

	handler := CorrelationMiddleware(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := FromContext(r.Context(), base)
		logger.Info(r.Context(), "handling request", "route", "/widgets")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set(CorrelationIDHeader, inboundID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 log line, got %d: %v", len(lines), lines)
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("invalid JSON log line: %v\nline: %s", err, lines[0])
	}
	if record[correlationIDLogField] != inboundID {
		t.Errorf("log record %s field = %v, want %q", correlationIDLogField, record[correlationIDLogField], inboundID)
	}
	if record["route"] != "/widgets" {
		t.Errorf("log record route field = %v, want /widgets", record["route"])
	}
}

// TestCorrelationMiddleware_DistinctRequestsGetDistinctIDs proves the
// middleware does not leak or reuse a generated ID across independent
// requests that both omit the inbound header.
func TestCorrelationMiddleware_DistinctRequestsGetDistinctIDs(t *testing.T) {
	base := New(WithOutput(&bytes.Buffer{}))

	var ids []string
	handler := CorrelationMiddleware(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := CorrelationIDFromContext(r.Context())
		ids = append(ids, id)
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if len(ids) != 2 {
		t.Fatalf("expected 2 recorded IDs, got %d", len(ids))
	}
	if ids[0] == ids[1] {
		t.Errorf("expected distinct correlation IDs across requests, got the same value twice: %q", ids[0])
	}
}
