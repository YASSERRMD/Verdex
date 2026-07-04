package alerting_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's/packages/backupdr's helpers_test.go newTestUser
// convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "alerting@example.test",
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

// newTestEngine builds an alerting.Engine backed by fresh in-memory
// repositories, returning the Engine and a fresh tenant ID so tests can
// exercise a full round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*alerting.Engine, uuid.UUID) {
	t.Helper()

	rules := alerting.NewInMemoryAlertRuleRepository()
	events := alerting.NewInMemoryAlertEventRepository()
	policies := alerting.NewInMemoryEscalationPolicyRepository()

	engine, err := alerting.NewEngine(rules, events, policies)
	if err != nil {
		t.Fatalf("alerting.NewEngine: %v", err)
	}
	return engine, uuid.New()
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManageAlerting and PermViewAlerting) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewAlerting) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// judgeUser is a small convenience wrapper building a RoleJudge user
// (holds neither alerting permission) scoped to tenantID, for
// negative-authorization tests.
func judgeUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleJudge)
}

// registerTestRule registers and returns a single threshold AlertRule,
// using adminUser(tenantID) as the actor, for tests that need a real
// catalogued rule without repeating the request shape.
func registerTestRule(t *testing.T, engine *alerting.Engine, tenantID uuid.UUID) alerting.AlertRule {
	t.Helper()
	admin := adminUser(tenantID)
	rule, err := engine.RegisterRule(ctxWithUser(admin), tenantID, alerting.AlertRule{
		Name:        "test-rule",
		Description: "A rule registered for test fixtures.",
		Condition: alerting.Condition{
			Kind:       alerting.ConditionThresholdAbove,
			MetricName: "test_metric",
			Threshold:  10,
		},
		Severity: alerting.SeverityWarning,
	})
	if err != nil {
		t.Fatalf("RegisterRule: %v", err)
	}
	return rule
}
