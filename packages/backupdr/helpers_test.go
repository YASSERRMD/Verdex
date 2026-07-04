package backupdr_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/backupdr"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// testFrequency and testRetention are stand-in BackupPolicy durations
// shared across this package's tests -- values themselves are
// arbitrary, only their relative use (frequency << retention) matters
// for the tests that read them.
const (
	testFrequency = time.Hour
	testRetention = 30 * 24 * time.Hour
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "backupdr@example.test",
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

// newTestEngine builds a backupdr.Engine backed by fresh in-memory
// repositories and an in-memory-backed AuditSink, returning the Engine
// and a fresh tenant ID so tests can exercise a full round-trip
// without repeating this wiring.
func newTestEngine(t *testing.T) (*backupdr.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*backupdr.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	policies := backupdr.NewInMemoryPolicyRepository()
	records := backupdr.NewInMemoryRecordRepository()
	drills := backupdr.NewInMemoryDrillRepository()
	targets := backupdr.NewInMemoryTargetRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := backupdr.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("backupdr.NewAuditSink: %v", err)
	}

	engine, err := backupdr.NewEngine(policies, records, drills, targets, sink)
	if err != nil {
		t.Fatalf("backupdr.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManageBackupDR and PermViewBackupDR) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewBackupDR) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// judgeUser is a small convenience wrapper building a RoleJudge user
// (holds neither backupdr permission) scoped to tenantID, for
// negative-authorization tests.
func judgeUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleJudge)
}

// setTestPolicy sets and returns a BackupPolicy for class, using
// adminUser(tenantID) as the actor, for tests that need a real
// registered policy without repeating the request shape.
func setTestPolicy(t *testing.T, engine *backupdr.Engine, tenantID uuid.UUID, class backupdr.DataClass) backupdr.BackupPolicy {
	t.Helper()
	admin := adminUser(tenantID)
	p, err := engine.SetPolicy(ctxWithUser(admin), tenantID, backupdr.BackupPolicy{
		Class:               class,
		Frequency:           testFrequency,
		RetentionWindow:     testRetention,
		EncryptionRequired:  true,
		CrossRegionRequired: false,
	})
	if err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}
	return p
}

// testIntegrityHash is a fixed, valid-looking integrity hash used by
// tests that need a succeeded BackupRecord but don't care about a
// specific hash value beyond it being present and consistent.
const testIntegrityHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"

// recordTestBackup records and returns a BackupStatusSucceeded
// BackupRecord for class taken at takenAt, using adminUser(tenantID)
// as the actor, for tests that need a real restorable backup on file
// without repeating the request shape.
func recordTestBackup(t *testing.T, engine *backupdr.Engine, tenantID uuid.UUID, class backupdr.DataClass, takenAt time.Time) backupdr.BackupRecord {
	t.Helper()
	admin := adminUser(tenantID)
	rec, err := engine.RecordBackup(ctxWithUser(admin), tenantID, backupdr.BackupRecord{
		Class:         class,
		TakenAt:       takenAt,
		Location:      backupdr.LocationPrimaryRegion,
		Reference:     "test-backup-" + uuid.NewString(),
		IntegrityHash: testIntegrityHash,
		SizeBytes:     1024,
		Encrypted:     true,
		Status:        backupdr.BackupStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("RecordBackup: %v", err)
	}
	return rec
}
