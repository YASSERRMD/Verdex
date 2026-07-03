package auditlog

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// exportRow is the flattened, stable-column-order shape Export writes
// to CSV. JSON export instead serializes Event directly (its json
// tags already give a stable, self-describing shape), but CSV needs an
// explicit column order and string coercion, so it gets its own type
// rather than reflecting over Event.
type exportRow struct {
	ID        string
	TenantID  string
	Time      string
	Actor     string
	Action    string
	Target    string
	Outcome   string
	Kind      string
	CaseID    string
	Detail    string
	PrevHash  string
	ChainHash string
}

func toExportRow(e Event) exportRow {
	caseID := ""
	if e.CaseID != uuid.Nil {
		caseID = e.CaseID.String()
	}
	return exportRow{
		ID:        e.ID.String(),
		TenantID:  e.TenantID.String(),
		Time:      e.Time.UTC().Format(time.RFC3339Nano),
		Actor:     e.Actor,
		Action:    e.Action,
		Target:    e.Target,
		Outcome:   e.Outcome,
		Kind:      string(e.Kind),
		CaseID:    caseID,
		Detail:    e.Detail,
		PrevHash:  e.PrevHash,
		ChainHash: e.ChainHash,
	}
}

var csvHeader = []string{
	"id", "tenant_id", "time", "actor", "action", "target", "outcome",
	"kind", "case_id", "detail", "prev_hash", "chain_hash",
}

func (r exportRow) csvRecord() []string {
	return []string{
		r.ID, r.TenantID, r.Time, r.Actor, r.Action, r.Target, r.Outcome,
		r.Kind, r.CaseID, r.Detail, r.PrevHash, r.ChainHash,
	}
}

// Export runs Query(ctx, tenantID, filter) — inheriting its
// identity.PermAuditRead + tenant-match access control (task 8) — and
// renders the matching events in format for a regulator/compliance
// handoff (task 7). Events are exported in the same chain order Query
// returns them in, so a recipient can independently run VerifyChain
// (or VerifyGenesisChain, if the export covers a tenant's complete
// history) over the parsed result to confirm no event was altered
// between generation and receipt.
func (s *Store) Export(ctx context.Context, tenantID uuid.UUID, filter Filter, format ExportFormat) ([]byte, error) {
	if !format.IsValid() {
		return nil, wrapf("Export", ErrInvalidExportFormat)
	}

	events, err := s.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("Export", err)
	}

	switch format {
	case ExportFormatJSON:
		return exportJSON(events)
	case ExportFormatCSV:
		return exportCSV(events)
	default:
		return nil, wrapf("Export", ErrInvalidExportFormat)
	}
}

func exportJSON(events []Event) ([]byte, error) {
	if events == nil {
		events = []Event{}
	}
	out, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return nil, wrapf("exportJSON", err)
	}
	return out, nil
}

func exportCSV(events []Event) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write(csvHeader); err != nil {
		return nil, wrapf("exportCSV", err)
	}
	for _, e := range events {
		if err := w.Write(toExportRow(e).csvRecord()); err != nil {
			return nil, wrapf("exportCSV", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, wrapf("exportCSV", err)
	}
	return buf.Bytes(), nil
}
