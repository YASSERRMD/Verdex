package alerting_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

func TestEngine_RegisterRule(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rule := registerTestRule(t, engine, tenantID)

	if rule.ID == uuid.Nil {
		t.Error("RegisterRule did not assign an ID")
	}
	if rule.TenantID != tenantID {
		t.Errorf("rule.TenantID = %v, want %v", rule.TenantID, tenantID)
	}
	if rule.CreatedAt.IsZero() {
		t.Error("RegisterRule did not set CreatedAt")
	}
}

func TestEngine_RegisterRule_DuplicateNameRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	registerTestRule(t, engine, tenantID)

	admin := adminUser(tenantID)
	_, err := engine.RegisterRule(ctxWithUser(admin), tenantID, alerting.AlertRule{
		Name:      "test-rule",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	})
	if !errors.Is(err, alerting.ErrDuplicateRule) {
		t.Fatalf("RegisterRule duplicate name error = %v, want ErrDuplicateRule", err)
	}
}

func TestEngine_RegisterRule_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	// A viewer-only (auditor) actor cannot register a rule.
	auditor := auditorUser(tenantID)
	_, err := engine.RegisterRule(ctxWithUser(auditor), tenantID, alerting.AlertRule{
		Name:      "r1",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	})
	if !errors.Is(err, alerting.ErrForbidden) {
		t.Fatalf("RegisterRule as auditor error = %v, want ErrForbidden", err)
	}

	// An actor holding neither alerting permission is also rejected.
	judge := judgeUser(tenantID)
	_, err = engine.RegisterRule(ctxWithUser(judge), tenantID, alerting.AlertRule{
		Name:      "r2",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	})
	if !errors.Is(err, alerting.ErrForbidden) {
		t.Fatalf("RegisterRule as judge error = %v, want ErrForbidden", err)
	}

	// No authenticated actor at all.
	_, err = engine.RegisterRule(t.Context(), tenantID, alerting.AlertRule{
		Name:      "r3",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	})
	if !errors.Is(err, alerting.ErrUnauthenticated) {
		t.Fatalf("RegisterRule with no actor error = %v, want ErrUnauthenticated", err)
	}
}

func TestEngine_RegisterRule_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()
	adminA := adminUser(tenantA)

	_, err := engine.RegisterRule(ctxWithUser(adminA), tenantB, alerting.AlertRule{
		Name:      "r1",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	})
	if !errors.Is(err, alerting.ErrCrossTenantAccess) {
		t.Fatalf("RegisterRule cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_ListRules_AuditorCanView(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	registerTestRule(t, engine, tenantID)

	auditor := auditorUser(tenantID)
	rules, err := engine.ListRules(ctxWithUser(auditor), tenantID)
	if err != nil {
		t.Fatalf("ListRules as auditor: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
}

func TestEngine_Evaluate_ThresholdAboveFires(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rule := alerting.AlertRule{
		ID:   uuid.New(),
		Name: "high-latency",
		Condition: alerting.Condition{
			Kind:       alerting.ConditionThresholdAbove,
			MetricName: "latency_ms",
			Threshold:  100,
		},
		Severity: alerting.SeverityCritical,
	}

	event, fired, err := engine.Evaluate(t.Context(), tenantID, rule, 150)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !fired {
		t.Fatal("Evaluate did not fire for a value exceeding the threshold")
	}
	if event.Severity != alerting.SeverityCritical {
		t.Errorf("event.Severity = %v, want SeverityCritical", event.Severity)
	}
	if event.TriggerValue != 150 {
		t.Errorf("event.TriggerValue = %v, want 150", event.TriggerValue)
	}
	if event.Detail == "" {
		t.Error("event.Detail is empty, want a human-readable explanation")
	}
}

func TestEngine_Evaluate_ThresholdAboveDoesNotFire(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rule := alerting.AlertRule{
		ID:   uuid.New(),
		Name: "high-latency",
		Condition: alerting.Condition{
			Kind:       alerting.ConditionThresholdAbove,
			MetricName: "latency_ms",
			Threshold:  100,
		},
		Severity: alerting.SeverityCritical,
	}

	_, fired, err := engine.Evaluate(t.Context(), tenantID, rule, 50)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if fired {
		t.Fatal("Evaluate fired for a value below the threshold")
	}
}

func TestEngine_Evaluate_ThresholdBelowFires(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rule := alerting.AlertRule{
		ID:   uuid.New(),
		Name: "low-success-rate",
		Condition: alerting.Condition{
			Kind:       alerting.ConditionThresholdBelow,
			MetricName: "success_rate",
			Threshold:  0.99,
		},
		Severity: alerting.SeverityWarning,
	}

	_, fired, err := engine.Evaluate(t.Context(), tenantID, rule, 0.95)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !fired {
		t.Fatal("Evaluate did not fire for a value below the threshold")
	}
}

func TestEngine_Evaluate_ThresholdBelowDoesNotFireAtBoundary(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rule := alerting.AlertRule{
		ID:   uuid.New(),
		Name: "low-success-rate",
		Condition: alerting.Condition{
			Kind:       alerting.ConditionThresholdBelow,
			MetricName: "success_rate",
			Threshold:  0.99,
		},
		Severity: alerting.SeverityWarning,
	}

	// Exactly at the threshold should not fire for ConditionThresholdBelow
	// (strictly less-than).
	_, fired, err := engine.Evaluate(t.Context(), tenantID, rule, 0.99)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if fired {
		t.Fatal("Evaluate fired for a value exactly at the threshold (want strict inequality)")
	}
}

func TestEngine_Evaluate_RejectsExternallyEvaluatedConditions(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	for _, kind := range []alerting.ConditionKind{
		alerting.ConditionSLOBreached,
		alerting.ConditionQualityRegression,
		alerting.ConditionCostThreshold,
	} {
		rule := alerting.AlertRule{
			ID:        uuid.New(),
			Name:      "externally-evaluated",
			Condition: alerting.Condition{Kind: kind, MetricName: "whatever"},
			Severity:  alerting.SeverityWarning,
		}
		_, fired, err := engine.Evaluate(t.Context(), tenantID, rule, 1)
		if !errors.Is(err, alerting.ErrNilCondition) {
			t.Errorf("Evaluate(kind=%s) error = %v, want ErrNilCondition", kind, err)
		}
		if fired {
			t.Errorf("Evaluate(kind=%s) fired = true, want false", kind)
		}
	}
}

func TestEngine_Evaluate_PersistsFiredEvent(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	rule := registerTestRule(t, engine, tenantID)

	_, fired, err := engine.Evaluate(t.Context(), tenantID, rule, 20)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !fired {
		t.Fatal("Evaluate did not fire")
	}

	admin := adminUser(tenantID)
	events, err := engine.ListEvents(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].RuleName != rule.Name {
		t.Errorf("events[0].RuleName = %q, want %q", events[0].RuleName, rule.Name)
	}
}

func TestEngine_SetPolicy_And_GetPolicy(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	policy := alerting.DefaultEscalationPolicy(tenantID)
	set, err := engine.SetPolicy(ctxWithUser(admin), tenantID, policy)
	if err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}
	if set.Name != "default" {
		t.Errorf("set.Name = %q, want %q", set.Name, "default")
	}

	got, err := engine.GetPolicy(ctxWithUser(admin), tenantID, "default")
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if len(got.Tiers) != len(policy.Tiers) {
		t.Errorf("len(got.Tiers) = %d, want %d", len(got.Tiers), len(policy.Tiers))
	}
}

func TestEngine_SetPolicy_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.SetPolicy(ctxWithUser(auditor), tenantID, alerting.DefaultEscalationPolicy(tenantID))
	if !errors.Is(err, alerting.ErrForbidden) {
		t.Fatalf("SetPolicy as auditor error = %v, want ErrForbidden", err)
	}
}

func TestNewEngine_RequiresNonNilStores(t *testing.T) {
	t.Parallel()
	rules := alerting.NewInMemoryAlertRuleRepository()
	events := alerting.NewInMemoryAlertEventRepository()
	policies := alerting.NewInMemoryEscalationPolicyRepository()

	if _, err := alerting.NewEngine(nil, events, policies); !errors.Is(err, alerting.ErrNilStore) {
		t.Errorf("NewEngine(nil rules, ...) error = %v, want ErrNilStore", err)
	}
	if _, err := alerting.NewEngine(rules, nil, policies); !errors.Is(err, alerting.ErrNilStore) {
		t.Errorf("NewEngine(..., nil events, ...) error = %v, want ErrNilStore", err)
	}
	if _, err := alerting.NewEngine(rules, events, nil); !errors.Is(err, alerting.ErrNilStore) {
		t.Errorf("NewEngine(..., nil policies) error = %v, want ErrNilStore", err)
	}
}
