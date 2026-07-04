package alerting_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/alerting"
	"github.com/YASSERRMD/verdex/packages/observability"
)

func TestRegisterBusinessMetrics_NilRegistry(t *testing.T) {
	t.Parallel()
	_, err := alerting.RegisterBusinessMetrics(nil)
	if !errors.Is(err, alerting.ErrNilStore) {
		t.Fatalf("RegisterBusinessMetrics(nil) error = %v, want ErrNilStore", err)
	}
}

func TestRegisterBusinessMetrics_RegistersRealHandles(t *testing.T) {
	t.Parallel()
	registry := observability.NewRegistry()
	catalogue, err := alerting.RegisterBusinessMetrics(registry)
	if err != nil {
		t.Fatalf("RegisterBusinessMetrics: %v", err)
	}
	if catalogue.CasesIngestedTotal == nil {
		t.Error("catalogue.CasesIngestedTotal is nil")
	}
	if catalogue.OpinionsSignedOffTotal == nil {
		t.Error("catalogue.OpinionsSignedOffTotal is nil")
	}
	if catalogue.SARRequestsPending == nil {
		t.Error("catalogue.SARRequestsPending is nil")
	}
	if catalogue.ReasoningRunsTotal == nil {
		t.Error("catalogue.ReasoningRunsTotal is nil")
	}
	if catalogue.AlertsFiredTotal == nil {
		t.Error("catalogue.AlertsFiredTotal is nil")
	}
	if catalogue.SyntheticCheckLatencySeconds == nil {
		t.Error("catalogue.SyntheticCheckLatencySeconds is nil")
	}

	// Exercise every handle so a real Prometheus exposition-format
	// scrape actually reflects them -- proving these are real,
	// registered metrics through observability.Registry, not inert
	// struct fields.
	catalogue.CasesIngestedTotal.Inc("success")
	catalogue.OpinionsSignedOffTotal.Inc("approved")
	catalogue.SARRequestsPending.Set(3, "tenant-a")
	catalogue.ReasoningRunsTotal.Add(2, "success")
	catalogue.AlertsFiredTotal.Inc("critical")
	catalogue.SyntheticCheckLatencySeconds.Observe(0.05, "health-endpoint", "pass")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	registry.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, wantMetric := range []string{
		alerting.MetricCasesIngestedTotal,
		alerting.MetricOpinionsSignedOffTotal,
		alerting.MetricSARRequestsPending,
		alerting.MetricReasoningRunsTotal,
		alerting.MetricAlertsFiredTotal,
		alerting.MetricSyntheticCheckLatencySeconds,
	} {
		if !strings.Contains(body, wantMetric) {
			t.Errorf("scraped /metrics output missing %q; got:\n%s", wantMetric, body)
		}
	}
}

func TestCatalogue_RecordAlertFired(t *testing.T) {
	t.Parallel()
	registry := observability.NewRegistry()
	catalogue, err := alerting.RegisterBusinessMetrics(registry)
	if err != nil {
		t.Fatalf("RegisterBusinessMetrics: %v", err)
	}

	// Must not panic, and should actually increment the underlying
	// counter.
	catalogue.RecordAlertFired(alerting.SeverityCritical)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	registry.Handler().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `severity="critical"`) {
		t.Errorf("scraped output missing severity=critical label; got:\n%s", rec.Body.String())
	}
}

func TestCatalogue_RecordAlertFired_NilCatalogueIsNoOp(t *testing.T) {
	t.Parallel()
	var catalogue *alerting.Catalogue
	// Must not panic.
	catalogue.RecordAlertFired(alerting.SeverityWarning)
}
