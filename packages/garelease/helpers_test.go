package garelease_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/garelease"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/observability"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to a fresh tenant, mirroring
// packages/compliance/helpers_test.go's newTestUser convention.
func newTestUser(roles ...identity.Role) *identity.User {
	return newTestUserForTenant(uuid.New(), roles...)
}

// newTestUserForTenant is newTestUser's fuller form: it scopes the
// built user to a caller-supplied tenantID rather than a fresh random
// one, for tests that need the authenticated actor's TenantID to match
// a specific representativeTenantID (e.g. VerifyAuditTrail's tenant-
// scoped Query/VerifyTenantChain calls, which require an exact match
// per packages/auditlog.requireMatchingUserTenant).
func newTestUserForTenant(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "garelease@example.test",
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
// both PermManageRelease and PermViewRelease).
func adminUser() *identity.User {
	return newTestUser(identity.RoleAdmin)
}

// auditorUser is a convenience wrapper building a RoleAuditor user
// (holds only PermViewRelease).
func auditorUser() *identity.User {
	return newTestUser(identity.RoleAuditor)
}

// noPermUser is a convenience wrapper building a user with a role that
// holds neither PermViewRelease nor PermManageRelease.
func noPermUser() *identity.User {
	return newTestUser(identity.RoleAdvocate)
}

// newTestEngine builds a garelease.Engine backed by fresh in-memory
// repositories and an in-memory-backed AuditSink.
func newTestEngine(t *testing.T) *garelease.Engine {
	t.Helper()
	engine, _ := newTestEngineWithAudit(t)
	return engine
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*garelease.Engine, *auditlog.Store) {
	t.Helper()

	candidates := garelease.NewInMemoryReleaseCandidateRepository()
	releases := garelease.NewInMemoryReleaseRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := garelease.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("garelease.NewAuditSink: %v", err)
	}

	engine, err := garelease.NewEngine(candidates, releases, sink)
	if err != nil {
		t.Fatalf("garelease.NewEngine: %v", err)
	}
	return engine, auditStore
}

// readyReadiness builds a ReleaseReadiness snapshot with every named
// dimension reporting CheckPassed, for tests that need a ready-to-freeze
// starting point.
func readyReadiness() garelease.ReleaseReadiness {
	now := time.Now().UTC()
	checks := []garelease.ReadinessCheck{
		{Dimension: garelease.DimensionCriticalFindings, Status: garelease.CheckPassed, Detail: "none open", EvaluatedAt: now},
		{Dimension: garelease.DimensionComplianceGaps, Status: garelease.CheckPassed, Detail: "no gaps", EvaluatedAt: now},
		{Dimension: garelease.DimensionPerfBudget, Status: garelease.CheckPassed, Detail: "all budgets met", EvaluatedAt: now},
		{Dimension: garelease.DimensionE2ERegression, Status: garelease.CheckPassed, Detail: "all scenarios passed", EvaluatedAt: now},
		{Dimension: garelease.DimensionGuardrailIntegrity, Status: garelease.CheckPassed, Detail: "guardrail intact", EvaluatedAt: now},
		{Dimension: garelease.DimensionAuditCompleteness, Status: garelease.CheckPassed, Detail: "audit trail intact", EvaluatedAt: now},
	}
	return garelease.ReleaseReadiness{Checks: checks, Ready: true, EvaluatedAt: now}
}

// notReadyReadiness builds a ReleaseReadiness snapshot with one failing
// dimension, for tests that need to prove freezing is blocked.
func notReadyReadiness() garelease.ReleaseReadiness {
	r := readyReadiness()
	for i := range r.Checks {
		if r.Checks[i].Dimension == garelease.DimensionCriticalFindings {
			r.Checks[i].Status = garelease.CheckFailed
			r.Checks[i].Detail = "2 critical findings still open"
		}
	}
	r.Ready = false
	return r
}

// newGreenEngine builds a garelease.Engine whose audit-completeness
// dimension will genuinely pass: a real *auditlog.Store is configured
// via WithAuditTrailStore, seeded with one event for tenantID, and the
// returned context carries a RoleAdmin user scoped to that exact
// tenantID (so VerifyAuditTrail's Query/VerifyTenantChain calls, which
// require an exact tenant match, succeed rather than failing on a
// mismatched actor tenant this fixture would otherwise introduce by
// accident). Tests that want to exercise "every dimension genuinely
// passes" (as opposed to "every dimension EXCEPT audit completeness,
// because no store was wired") should use this helper rather than
// newTestEngine.
func newGreenEngine(t *testing.T) (*garelease.Engine, context.Context) {
	t.Helper()
	engine, auditStore := newTestEngineWithAudit(t)
	tenantID := uuid.New()

	if _, err := auditStore.Append(context.Background(), auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindSystem,
		AuditEvent: observability.AuditEvent{
			Actor:   "system:test",
			Action:  "test.seed",
			Target:  tenantID.String(),
			Outcome: "seeded",
		},
	}); err != nil {
		t.Fatalf("seeding audit event: %v", err)
	}

	engine.WithAuditTrailStore(auditStore, tenantID)
	ctx := ctxWithUser(newTestUserForTenant(tenantID, identity.RoleAdmin))
	return engine, ctx
}

// mustUUID returns a fresh, non-nil uuid.UUID, a small readability
// helper for test fixtures that need a well-formed foreign-key-shaped
// ID but do not care about its specific value.
func mustUUID() uuid.UUID {
	return uuid.New()
}

// emptyUUID returns uuid.Nil, a small readability helper naming the
// "deliberately empty" case explicitly at test call sites.
func emptyUUID() uuid.UUID {
	return uuid.Nil
}
