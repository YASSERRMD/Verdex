package analytics_test

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/analytics"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

func sampleMetrics() *analytics.Metrics {
	jurisdictionID := uuid.New()
	return &analytics.Metrics{
		TenantID:    uuid.New(),
		GeneratedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		TotalCases:  3,
		ByState: []analytics.StateCount{
			{State: caselifecycle.StateActive, Count: 2},
			{State: caselifecycle.StateClosed, Count: 1},
		},
		ByCategory: []analytics.CategoryCount{
			{CategoryID: "contract", Count: 3},
		},
		ByJurisdiction: []analytics.JurisdictionBreakdown{
			{
				JurisdictionID: jurisdictionID,
				Count:          3,
				ByState: []analytics.StateCount{
					{State: caselifecycle.StateActive, Count: 2},
					{State: caselifecycle.StateClosed, Count: 1},
				},
			},
		},
		CreatedTrend: []analytics.DailyCaseCount{
			{Date: "2026-05-30", Count: 2},
			{Date: "2026-05-31", Count: 1},
		},
	}
}

func TestExport_NilMetrics(t *testing.T) {
	_, err := analytics.Export(nil, analytics.FormatJSON)
	if !errors.Is(err, analytics.ErrNilMetrics) {
		t.Errorf("Export(nil) error = %v, want ErrNilMetrics", err)
	}
}

func TestExport_InvalidFormat(t *testing.T) {
	_, err := analytics.Export(sampleMetrics(), analytics.ExportFormat("xml"))
	if !errors.Is(err, analytics.ErrInvalidFormat) {
		t.Errorf("Export(xml) error = %v, want ErrInvalidFormat", err)
	}
}

func TestExport_JSON_RoundTrips(t *testing.T) {
	m := sampleMetrics()
	data, err := analytics.Export(m, analytics.FormatJSON)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	var got analytics.Metrics
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; data = %s", err, data)
	}
	if got.TotalCases != m.TotalCases {
		t.Errorf("round-tripped TotalCases = %d, want %d", got.TotalCases, m.TotalCases)
	}
	if got.TenantID != m.TenantID {
		t.Errorf("round-tripped TenantID = %v, want %v", got.TenantID, m.TenantID)
	}
	if len(got.ByState) != len(m.ByState) {
		t.Fatalf("round-tripped len(ByState) = %d, want %d", len(got.ByState), len(m.ByState))
	}
	if len(got.CreatedTrend) != len(m.CreatedTrend) {
		t.Fatalf("round-tripped len(CreatedTrend) = %d, want %d", len(got.CreatedTrend), len(m.CreatedTrend))
	}
}

func TestExport_CSV_IsValidAndParseable(t *testing.T) {
	m := sampleMetrics()
	data, err := analytics.Export(m, analytics.FormatCSV)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(data)))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll() error = %v; data = %s", err, data)
	}
	if len(rows) < 2 {
		t.Fatalf("len(rows) = %d, want at least 2 (header + data)", len(rows))
	}

	wantHeader := []string{"section", "key", "sub_key", "count"}
	for i, col := range wantHeader {
		if rows[0][i] != col {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], col)
		}
	}

	// Every row must have exactly 4 columns and a parseable count.
	sawTotal := false
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("row %v has %d columns, want 4", row, len(row))
		}
		if row[0] == "total" {
			sawTotal = true
			if row[3] != "3" {
				t.Errorf("total row count = %q, want %q", row[3], "3")
			}
		}
	}
	if !sawTotal {
		t.Error("no 'total' row found in CSV export")
	}
}

func TestExport_CSV_IncludesJurisdictionStateBreakdown(t *testing.T) {
	m := sampleMetrics()
	data, err := analytics.Export(m, analytics.FormatCSV)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if !strings.Contains(string(data), "jurisdiction_state") {
		t.Error("CSV export missing jurisdiction_state rows")
	}
	if !strings.Contains(string(data), "created_trend") {
		t.Error("CSV export missing created_trend rows")
	}
}
