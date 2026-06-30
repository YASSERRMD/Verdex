package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistry_Counter(t *testing.T) {
	reg := NewRegistry()
	counter := reg.Counter("verdex_test_requests_total", "test counter", "method")

	counter.Inc("GET")
	counter.Inc("GET")
	counter.Add(3, "POST")

	body := scrapeMetrics(t, reg)
	if !strings.Contains(body, `verdex_test_requests_total{method="GET"} 2`) {
		t.Errorf("expected GET count of 2 in output:\n%s", body)
	}
	if !strings.Contains(body, `verdex_test_requests_total{method="POST"} 3`) {
		t.Errorf("expected POST count of 3 in output:\n%s", body)
	}
}

func TestRegistry_Gauge(t *testing.T) {
	reg := NewRegistry()
	gauge := reg.Gauge("verdex_test_inflight", "test gauge", "queue")

	gauge.Set(5, "default")
	gauge.Inc("default")
	gauge.Dec("default")
	gauge.Dec("default")

	body := scrapeMetrics(t, reg)
	if !strings.Contains(body, `verdex_test_inflight{queue="default"} 4`) {
		t.Errorf("expected gauge value of 4 in output:\n%s", body)
	}
}

func TestRegistry_Histogram(t *testing.T) {
	reg := NewRegistry()
	hist := reg.Histogram("verdex_test_duration_seconds", "test histogram", []float64{0.1, 0.5, 1}, "route")

	hist.Observe(0.05, "/healthz")
	hist.Observe(0.2, "/healthz")

	body := scrapeMetrics(t, reg)
	if !strings.Contains(body, "verdex_test_duration_seconds_bucket") {
		t.Errorf("expected histogram buckets in output:\n%s", body)
	}
	if !strings.Contains(body, `verdex_test_duration_seconds_count{route="/healthz"} 2`) {
		t.Errorf("expected histogram count of 2 in output:\n%s", body)
	}
}

func TestRegistry_HandlerServesPrometheusFormat(t *testing.T) {
	reg := NewRegistry()
	reg.Counter("verdex_test_handler_total", "test counter").Inc()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("unexpected content type: %s", contentType)
	}
}

func scrapeMetrics(t *testing.T, reg Registry) string {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape failed with status %d", rec.Code)
	}
	return rec.Body.String()
}
