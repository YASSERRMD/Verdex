package auditlog_test

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestStore_Export_JSON_ProducesParseableOutput(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	mustAppend(t, store, newEvent(tenantID, "user-2", "case.signoff", auditlog.KindSignoff))

	auditor := newTestUser(tenantID, identity.RoleAuditor)
	out, err := store.Export(ctxWithUser(auditor), tenantID, auditlog.Filter{}, auditlog.ExportFormatJSON)
	if err != nil {
		t.Fatalf("Export JSON: %v", err)
	}

	var events []auditlog.Event
	if err := json.Unmarshal(out, &events); err != nil {
		t.Fatalf("Export JSON output did not parse: %v\noutput: %s", err, out)
	}
	if len(events) != 2 {
		t.Fatalf("Export JSON: got %d events, want 2", len(events))
	}

	valid, brokenAt, err := auditlog.VerifyGenesisChain(events)
	if err != nil || !valid || brokenAt != -1 {
		t.Fatalf("VerifyGenesisChain over exported JSON events: valid=%v brokenAt=%d err=%v", valid, brokenAt, err)
	}
}

func TestStore_Export_CSV_ProducesParseableOutput(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))
	mustAppend(t, store, newEvent(tenantID, "user-2", "case.signoff", auditlog.KindSignoff))

	auditor := newTestUser(tenantID, identity.RoleAuditor)
	out, err := store.Export(ctxWithUser(auditor), tenantID, auditlog.Filter{}, auditlog.ExportFormatCSV)
	if err != nil {
		t.Fatalf("Export CSV: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(out)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("Export CSV output did not parse: %v\noutput: %s", err, out)
	}
	// Header row + 2 data rows.
	if len(records) != 3 {
		t.Fatalf("Export CSV: got %d rows (incl. header), want 3", len(records))
	}
	wantHeader := []string{"id", "tenant_id", "time", "actor", "action", "target", "outcome", "kind", "case_id", "detail", "prev_hash", "chain_hash"}
	for i, col := range wantHeader {
		if records[0][i] != col {
			t.Fatalf("CSV header[%d] = %q, want %q", i, records[0][i], col)
		}
	}
}

func TestStore_Export_RejectsInvalidFormat(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	auditor := newTestUser(tenantID, identity.RoleAuditor)

	_, err := store.Export(ctxWithUser(auditor), tenantID, auditlog.Filter{}, auditlog.ExportFormat("xml"))
	if !errors.Is(err, auditlog.ErrInvalidExportFormat) {
		t.Fatalf("Export with bad format: err = %v, want ErrInvalidExportFormat", err)
	}
}

func TestStore_Export_RequiresAuditReadPermission(t *testing.T) {
	store := newTestStore(t)
	tenantID := uuid.New()
	mustAppend(t, store, newEvent(tenantID, "user-1", "case.viewed", auditlog.KindDataAccess))

	clerk := newTestUser(tenantID, identity.RoleClerk)
	_, err := store.Export(ctxWithUser(clerk), tenantID, auditlog.Filter{}, auditlog.ExportFormatJSON)
	if !errors.Is(err, auditlog.ErrForbidden) {
		t.Fatalf("Export as clerk: err = %v, want ErrForbidden", err)
	}
}
