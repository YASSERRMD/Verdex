package bulkimport_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/bulkimport"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/privacy's and packages/compliance's helpers_test.go
// newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "bulkimport@example.test",
		Name:     "Test User",
		Roles:    roles,
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManageBulkImport and PermViewBulkImport) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewBulkImport) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// newTestEngine builds a bulkimport.Engine backed by fresh in-memory
// repositories and an in-memory-backed AuditSink, returning the
// Engine and a fresh tenant ID so tests can exercise a full
// round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*bulkimport.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly (task 8's
// "every job registration, batch run, and rollback gets recorded
// there").
func newTestEngineWithAudit(t *testing.T) (*bulkimport.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	jobs := bulkimport.NewInMemoryJobRepository()
	records := bulkimport.NewInMemoryRecordRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := bulkimport.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("bulkimport.NewAuditSink: %v", err)
	}

	engine, err := bulkimport.NewEngine(jobs, records, sink)
	if err != nil {
		t.Fatalf("bulkimport.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// sampleSourceRecords builds n well-formed SourceRecord values with
// distinct case numbers, so every record dedups distinctly by
// default.
func sampleSourceRecords(n int) []bulkimport.SourceRecord {
	out := make([]bulkimport.SourceRecord, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, bulkimport.SourceRecord{
			PayloadRef:   uuid.New().String(),
			CaseNumber:   uuid.New().String(),
			Jurisdiction: "dubai-courts",
			PartyNames:   []string{"Jane Doe", "Acme LLC"},
		})
	}
	return out
}
