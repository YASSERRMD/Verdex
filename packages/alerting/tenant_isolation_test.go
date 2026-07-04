package alerting_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

// TestInMemoryAlertRuleRepository_TenantIsolated proves the repository
// layer itself never leaks a tenant's rules into another tenant's
// List/Get results, independent of the Engine authorization layer
// above it, mirroring
// packages/compliance.TestInMemoryEvidenceRepository_TenantIsolated
// exactly.
func TestInMemoryAlertRuleRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := alerting.NewInMemoryAlertRuleRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	ruleA := &alerting.AlertRule{
		ID: uuid.New(), TenantID: tenantA, Name: "rule-a",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	}
	ruleB := &alerting.AlertRule{
		ID: uuid.New(), TenantID: tenantB, Name: "rule-b",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	}
	if err := repo.Create(t.Context(), tenantA, ruleA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, ruleB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.List(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("List (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != ruleA.ID {
		t.Fatalf("List(tenantA) = %v, want exactly ruleA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, ruleB.ID); !errors.Is(err, alerting.ErrRuleNotFound) {
		t.Fatalf("Get(tenantA, ruleB.ID) error = %v, want ErrRuleNotFound", err)
	}

	// Duplicate rule names are only rejected within the same tenant --
	// tenant A and tenant B may both register a rule literally named
	// "rule-a" without colliding.
	dup := &alerting.AlertRule{
		ID: uuid.New(), TenantID: tenantB, Name: "rule-a",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	}
	if err := repo.Create(t.Context(), tenantB, dup); err != nil {
		t.Fatalf("Create (B, same name as A): %v, want success (names are tenant-scoped, not global)", err)
	}
}

// TestInMemoryAlertEventRepository_TenantIsolated mirrors the same
// guarantee for fired AlertEvent history.
func TestInMemoryAlertEventRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := alerting.NewInMemoryAlertEventRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	evA := &alerting.AlertEvent{ID: uuid.New(), TenantID: tenantA, RuleName: "r", Severity: alerting.SeverityCritical}
	evB := &alerting.AlertEvent{ID: uuid.New(), TenantID: tenantB, RuleName: "r", Severity: alerting.SeverityCritical}
	if err := repo.Create(t.Context(), tenantA, evA); err != nil {
		t.Fatalf("Create (A): %v", err)
	}
	if err := repo.Create(t.Context(), tenantB, evB); err != nil {
		t.Fatalf("Create (B): %v", err)
	}

	listA, err := repo.ListAll(t.Context(), tenantA)
	if err != nil {
		t.Fatalf("ListAll (A): %v", err)
	}
	if len(listA) != 1 || listA[0].ID != evA.ID {
		t.Fatalf("ListAll(tenantA) = %v, want exactly evA", listA)
	}

	if _, err := repo.Get(t.Context(), tenantA, evB.ID); !errors.Is(err, alerting.ErrEventNotFound) {
		t.Fatalf("Get(tenantA, evB.ID) error = %v, want ErrEventNotFound", err)
	}
}

// TestInMemoryEscalationPolicyRepository_TenantIsolated mirrors the
// same guarantee for EscalationPolicy configuration.
func TestInMemoryEscalationPolicyRepository_TenantIsolated(t *testing.T) {
	t.Parallel()
	repo := alerting.NewInMemoryEscalationPolicyRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()

	policyA := alerting.DefaultEscalationPolicy(tenantA)
	policyB := alerting.DefaultEscalationPolicy(tenantB)
	policyB.Tiers[0].Responder.Name = "tenant-b-primary"

	if err := repo.Set(t.Context(), tenantA, &policyA); err != nil {
		t.Fatalf("Set (A): %v", err)
	}
	if err := repo.Set(t.Context(), tenantB, &policyB); err != nil {
		t.Fatalf("Set (B): %v", err)
	}

	gotA, err := repo.Get(t.Context(), tenantA, "default")
	if err != nil {
		t.Fatalf("Get (A): %v", err)
	}
	if gotA.Tiers[0].Responder.Name != "oncall-primary" {
		t.Fatalf("Get(tenantA).Tiers[0].Responder.Name = %q, want oncall-primary (tenant A's own policy, not B's)", gotA.Tiers[0].Responder.Name)
	}

	if _, err := repo.Get(t.Context(), tenantA, "nonexistent"); !errors.Is(err, alerting.ErrPolicyNotFound) {
		t.Fatalf("Get(tenantA, nonexistent) error = %v, want ErrPolicyNotFound", err)
	}
}

// TestEngine_ListRules_CrossTenantRejected proves an admin
// authenticated against tenant A can never list tenant B's alert
// rules through the Engine authorization layer, mirroring
// packages/compliance.TestEngine_GetProfile_CrossTenantRejected.
func TestEngine_ListRules_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.ListRules(ctxWithUser(adminA), tenantB)
	if !errors.Is(err, alerting.ErrCrossTenantAccess) {
		t.Fatalf("ListRules() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_GetPolicy_CrossTenantRejected mirrors the same guarantee
// for GetPolicy.
func TestEngine_GetPolicy_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.GetPolicy(ctxWithUser(adminA), tenantB, "default")
	if !errors.Is(err, alerting.ErrCrossTenantAccess) {
		t.Fatalf("GetPolicy() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestEngine_RulesDoNotLeakAcrossTenants proves that tenant B's
// ListRules never reflects tenant A's registered rules, even when both
// tenants register a rule with the same literal Name.
func TestEngine_RulesDoNotLeakAcrossTenants(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	registerTestRule(t, engine, tenantA)

	tenantB := uuid.New()
	adminB := adminUser(tenantB)

	rulesB, err := engine.ListRules(ctxWithUser(adminB), tenantB)
	if err != nil {
		t.Fatalf("ListRules (B): %v", err)
	}
	if len(rulesB) != 0 {
		t.Fatalf("ListRules(tenantB) = %v, want empty (tenant A's rule must not leak)", rulesB)
	}

	// Tenant B can register a rule with the identical Name tenant A
	// used, without colliding.
	rule := registerTestRule(t, engine, tenantB)
	if rule.TenantID != tenantB {
		t.Fatalf("registered rule TenantID = %v, want %v", rule.TenantID, tenantB)
	}
}
