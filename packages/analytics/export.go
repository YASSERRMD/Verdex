package analytics

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strconv"
)

// ExportFormat selects Export's output encoding.
type ExportFormat string

const (
	// FormatCSV renders Metrics as CSV: one row per (breakdown kind,
	// key, count) tuple, plus a leading summary row. See Export's doc
	// comment for the exact column layout.
	FormatCSV ExportFormat = "csv"

	// FormatJSON renders Metrics as its canonical JSON encoding
	// (encoding/json applied to the Metrics struct directly), so the
	// exported bytes round-trip through json.Unmarshal into an
	// equivalent Metrics value.
	FormatJSON ExportFormat = "json"
)

// IsValid reports whether f is a recognized ExportFormat.
func (f ExportFormat) IsValid() bool {
	switch f {
	case FormatCSV, FormatJSON:
		return true
	default:
		return false
	}
}

// Export renders m as CSV or JSON bytes, real and parseable in both
// cases (verified by round-trip tests — see export_test.go).
//
// This package writes directly with encoding/csv and encoding/json
// rather than reusing packages/reportexport's rendering pipeline:
// reportexport assembles a case narrative document (facts, issues,
// analysis, citations) into PDF/DOCX/Markdown/plain-text, which is the
// wrong shape for exporting tabular aggregate metrics — a CSV/JSON
// writer is the simpler, more direct fit for this data, per the phase
// task's explicit "your call" on export mechanism.
//
// Returns ErrNilMetrics if m is nil, or ErrInvalidFormat if format is
// not FormatCSV or FormatJSON.
func Export(m *Metrics, format ExportFormat) ([]byte, error) {
	if m == nil {
		return nil, ErrNilMetrics
	}
	switch format {
	case FormatJSON:
		return exportJSON(m)
	case FormatCSV:
		return exportCSV(m)
	default:
		return nil, ErrInvalidFormat
	}
}

func exportJSON(m *Metrics) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// exportCSV renders m as CSV with a fixed four-column layout:
// section, key, sub_key, count. "section" is one of "total", "state",
// "category", "jurisdiction", "jurisdiction_state", or "created_trend";
// "key" and "sub_key" hold the relevant identifiers for that section
// (sub_key is empty except for jurisdiction_state rows, which also
// carry the per-jurisdiction state breakdown); "count" is always the
// integer count. A single flat table (rather than one CSV per
// breakdown) keeps the export to one file while remaining fully
// parseable: every row is self-describing via its section column.
func exportCSV(m *Metrics) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{"section", "key", "sub_key", "count"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	rows := [][]string{
		{"total", "", "", strconv.Itoa(m.TotalCases)},
	}
	for _, sc := range m.ByState {
		rows = append(rows, []string{"state", string(sc.State), "", strconv.Itoa(sc.Count)})
	}
	for _, cc := range m.ByCategory {
		rows = append(rows, []string{"category", cc.CategoryID, "", strconv.Itoa(cc.Count)})
	}
	for _, jb := range m.ByJurisdiction {
		rows = append(rows, []string{"jurisdiction", jb.JurisdictionID.String(), "", strconv.Itoa(jb.Count)})
		for _, sc := range jb.ByState {
			rows = append(rows, []string{"jurisdiction_state", jb.JurisdictionID.String(), string(sc.State), strconv.Itoa(sc.Count)})
		}
	}
	for _, dc := range m.CreatedTrend {
		rows = append(rows, []string{"created_trend", dc.Date, "", strconv.Itoa(dc.Count)})
	}

	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
