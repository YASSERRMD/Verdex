package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/integration"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "integration@example.test",
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

// adminUser is a convenience wrapper building a RoleAdmin user (holds
// both PermManageIntegration and PermViewIntegration) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a convenience wrapper building a RoleAuditor user
// (holds only PermViewIntegration) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// judgeUser is a convenience wrapper building a RoleJudge user (holds
// neither integration permission) scoped to tenantID, used to exercise
// ErrForbidden paths.
func judgeUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleJudge)
}

// newTestAuditSink builds an *integration.AuditSink backed by a fresh
// in-memory auditlog.Store, returning both so tests can inspect
// recorded events directly.
func newTestAuditSink(t *testing.T) (*integration.AuditSink, *auditlog.Store) {
	t.Helper()
	store, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := integration.NewAuditSink(store)
	if err != nil {
		t.Fatalf("integration.NewAuditSink: %v", err)
	}
	return sink, store
}

// newTestEngine builds an *integration.Engine backed by fresh
// in-memory repositories and an in-memory-backed AuditSink, returning
// the Engine and a fresh tenant ID so tests can exercise a full
// round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*integration.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to.
func newTestEngineWithAudit(t *testing.T) (*integration.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	configs := integration.NewInMemoryConfigRepository()
	credentials := integration.NewInMemoryCredentialsRepository()
	mappings := integration.NewInMemoryFieldMappingRepository()
	imports := integration.NewInMemoryImportRunRepository()
	deliveries := integration.NewInMemoryDeliveryRunRepository()
	reconciliations := integration.NewInMemoryReconciliationRepository()

	sink, auditStore := newTestAuditSink(t)

	registry := integration.NewRegistry()
	sandbox := integration.NewSandboxConnector("sandbox")
	if err := registry.Register(sandbox.ID(), sandbox); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}

	engine, err := integration.NewEngine(integration.EngineDeps{
		Configs:         configs,
		Credentials:     credentials,
		Mappings:        mappings,
		Imports:         imports,
		Deliveries:      deliveries,
		Reconciliations: reconciliations,
		Registry:        registry,
		Audit:           sink,
	})
	if err != nil {
		t.Fatalf("integration.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}
