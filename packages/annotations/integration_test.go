package annotations_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/casesearch/integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/casesearch/integration_test.go. Run with
// -short to skip entirely, per this repo's convention that
// Docker-backed integration verification is a separate pass.
func requirePostgresContainer(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping testcontainers-backed integration test in -short mode")
	}

	startCtx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	ctr, err := tcpostgres.Run(startCtx, "postgres:16-alpine",
		tcpostgres.WithDatabase("verdex_test"),
		tcpostgres.WithUsername("verdex"),
		tcpostgres.WithPassword("verdex-test-password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(containerStartTimeout),
		),
	)
	if err != nil {
		t.Skipf("skipping: could not start Postgres testcontainer (Docker unavailable?): %v", err)
	}

	t.Cleanup(func() {
		tearCtx, tearCancel := context.WithTimeout(context.Background(), containerStartTimeout)
		defer tearCancel()
		if err := ctr.Terminate(tearCtx); err != nil {
			t.Logf("warning: failed to terminate postgres container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(startCtx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}
	return dsn
}

// migratedAppPool starts a Postgres container, applies every embedded
// packages/persistence migration as the bootstrap superuser, then
// returns a pool authenticated as the non-superuser verdex_app role,
// mirroring packages/casesearch/integration_test.go's helper of the
// same name exactly.
func migratedAppPool(t *testing.T) *persistence.Postgres {
	t.Helper()
	dsn := requirePostgresContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	migrator, err := persistence.NewEmbeddedMigrator(dsn)
	if err != nil {
		t.Fatalf("NewEmbeddedMigrator: %v", err)
	}
	t.Cleanup(func() { _ = migrator.Close() })
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	cfg := config.Default()
	cfg.Database.DSN = dsn
	superuserPG, err := persistence.Open(ctx, &cfg)
	if err != nil {
		t.Fatalf("Open (superuser): %v", err)
	}
	defer superuserPG.Close()

	password, err := tenancy.GenerateAppRolePassword()
	if err != nil {
		t.Fatalf("GenerateAppRolePassword: %v", err)
	}
	if err := tenancy.BootstrapAppRolePassword(ctx, superuserPG.Pool(), password); err != nil {
		t.Fatalf("BootstrapAppRolePassword: %v", err)
	}
	appDSN, err := tenancy.BuildAppRoleDSN(dsn, password)
	if err != nil {
		t.Fatalf("BuildAppRoleDSN: %v", err)
	}

	appCfg := config.Default()
	appCfg.Database.DSN = appDSN
	appPG, err := persistence.Open(ctx, &appCfg)
	if err != nil {
		t.Fatalf("Open (app role): %v", err)
	}
	t.Cleanup(appPG.Close)

	if err := tenancy.VerifyRLSEnforceable(ctx, appPG.Pool()); err != nil {
		t.Fatalf("VerifyRLSEnforceable: %v", err)
	}
	return appPG
}

func seedTenant(t *testing.T, pg *persistence.Postgres, name, slug string) *persistence.Tenant {
	t.Helper()
	tenants := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: name, Slug: slug}
	if err := tenants.Create(context.Background(), pg.Pool(), tenant); err != nil {
		t.Fatalf("create tenant %s: %v", slug, err)
	}
	return tenant
}

func seedIntegrationCase(t *testing.T, caseRepo caselifecycle.Repository, tenantID, actorID uuid.UUID, title string) *caselifecycle.Case {
	t.Helper()
	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: uuid.New(),
		CategoryID:     "civil",
		Title:          title,
		CreatedBy:      actorID,
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := caseRepo.Create(context.Background(), tenantID, c); err != nil {
		t.Fatalf("caseRepo.Create: %v", err)
	}
	return c
}

// TestIntegration_Service_RealPostgresRepository proves Service
// composes with a real Postgres-backed
// annotations.TenantScopedRepository and caselifecycle.
// TenantScopedRepository (not just the in-memory fixtures used
// elsewhere in this package's test suite): create, thread, resolve,
// and mention persistence against real rows, with Row-Level Security
// enforcing the tenant boundary at the database layer.
func TestIntegration_Service_RealPostgresRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-annotations")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	// RoleAdmin, not RoleClerk: this actor also calls svc.AuditTrail
	// below, which requires identity.PermAuditRead in addition to the
	// view/edit permissions RoleClerk already holds. RoleClerk
	// deliberately lacks PermAuditRead (see
	// TestService_AuditTrail_RequiresAuditPermission in audit_test.go,
	// which proves RoleClerk must be forbidden from AuditTrail), so
	// RoleAdmin is the correct fixture here -- mirroring this package's
	// own audit_test.go and packages/keymanagement's
	// postgres_integration_test.go, both of which use RoleAdmin for the
	// actor exercising an audit-trail read.
	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	c := seedIntegrationCase(t, caseRepo, tenant.ID, actor.ID, "Civil Matter")

	annoRepo := annotations.NewTenantScopedRepository(pg.Pool())
	svc, err := annotations.NewService(annoRepo, caseRepo, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	mentioned := uuid.New()
	root, err := svc.Create(ctx, tenant.ID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "Flagging this for review @" + mentioned.String(),
		AnchorType: annotations.AnchorTreeNode,
		AnchorID:   "issue-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reply, err := svc.Create(ctx, tenant.ID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "Looking into it now.",
		AnchorType: annotations.AnchorTreeNode,
		AnchorID:   "issue-1",
		ParentID:   &root.ID,
	})
	if err != nil {
		t.Fatalf("Create reply: %v", err)
	}

	thread, err := svc.Thread(ctx, tenant.ID, root.ID)
	if err != nil {
		t.Fatalf("Thread: %v", err)
	}
	if len(thread) != 2 || thread[0].ID != root.ID || thread[1].ID != reply.ID {
		t.Fatalf("Thread = %+v, want [root, reply]", thread)
	}

	mentions, err := svc.MentionsFor(ctx, tenant.ID, mentioned)
	if err != nil {
		t.Fatalf("MentionsFor: %v", err)
	}
	if len(mentions) != 1 || mentions[0].AnnotationID != root.ID {
		t.Fatalf("MentionsFor = %+v, want one mention on %s", mentions, root.ID)
	}

	resolved, err := svc.Resolve(ctx, tenant.ID, root.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resolved.Resolved {
		t.Fatal("expected Resolved to be true")
	}

	trail, err := svc.AuditTrail(ctx, tenant.ID, root.ID)
	if err != nil {
		t.Fatalf("AuditTrail: %v", err)
	}
	if len(trail) != 2 {
		t.Fatalf("len(trail) = %d, want 2 (created, resolved)", len(trail))
	}

	// A second tenant must not see tenant A's annotation, proving RLS
	// (not just the application-level guard) enforces the boundary.
	otherTenant := seedTenant(t, pg, "Tenant B", "tenant-b-annotations")
	otherActor := &identity.User{ID: uuid.New(), TenantID: otherTenant.ID, Roles: []identity.Role{identity.RoleClerk}, Status: identity.UserStatusActive}
	otherCtx := identity.WithUser(context.Background(), otherActor)
	if _, err := svc.Get(otherCtx, otherTenant.ID, root.ID); err == nil {
		t.Fatal("expected an error reading tenant A's annotation from tenant B's scope")
	}
}
