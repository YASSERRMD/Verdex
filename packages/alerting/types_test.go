package alerting_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

func TestSeverity_IsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		severity alerting.Severity
		want     bool
	}{
		{alerting.SeverityInfo, true},
		{alerting.SeverityWarning, true},
		{alerting.SeverityCritical, true},
		{alerting.Severity("bogus"), false},
		{alerting.Severity(""), false},
	}
	for _, c := range cases {
		if got := c.severity.IsValid(); got != c.want {
			t.Errorf("Severity(%q).IsValid() = %v, want %v", c.severity, got, c.want)
		}
	}
}

func TestSeverity_AtLeast(t *testing.T) {
	t.Parallel()
	if !alerting.SeverityCritical.AtLeast(alerting.SeverityWarning) {
		t.Error("Critical.AtLeast(Warning) = false, want true")
	}
	if alerting.SeverityWarning.AtLeast(alerting.SeverityCritical) {
		t.Error("Warning.AtLeast(Critical) = true, want false")
	}
	if !alerting.SeverityWarning.AtLeast(alerting.SeverityWarning) {
		t.Error("Warning.AtLeast(Warning) = false, want true (equal counts as at-least)")
	}
}

func TestConditionKind_IsValid(t *testing.T) {
	t.Parallel()
	valid := []alerting.ConditionKind{
		alerting.ConditionThresholdAbove,
		alerting.ConditionThresholdBelow,
		alerting.ConditionSLOBreached,
		alerting.ConditionQualityRegression,
		alerting.ConditionCostThreshold,
	}
	for _, k := range valid {
		if !k.IsValid() {
			t.Errorf("ConditionKind(%q).IsValid() = false, want true", k)
		}
	}
	if alerting.ConditionKind("bogus").IsValid() {
		t.Error(`ConditionKind("bogus").IsValid() = true, want false`)
	}
}

func TestCondition_Validate(t *testing.T) {
	t.Parallel()

	valid := alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed Condition = %v, want nil", err)
	}

	missingMetric := alerting.Condition{Kind: alerting.ConditionThresholdAbove}
	if err := missingMetric.Validate(); !errors.Is(err, alerting.ErrInvalidRule) {
		t.Errorf("Validate() with blank MetricName = %v, want ErrInvalidRule", err)
	}

	badKind := alerting.Condition{Kind: "bogus", MetricName: "m"}
	if err := badKind.Validate(); !errors.Is(err, alerting.ErrInvalidRule) {
		t.Errorf("Validate() with invalid Kind = %v, want ErrInvalidRule", err)
	}
}

func TestAlertRule_Validate(t *testing.T) {
	t.Parallel()

	valid := &alerting.AlertRule{
		Name:      "r1",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m", Threshold: 1},
		Severity:  alerting.SeverityWarning,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed AlertRule = %v, want nil", err)
	}

	if err := (&alerting.AlertRule{}).Validate(); !errors.Is(err, alerting.ErrInvalidRule) {
		t.Errorf("Validate() on empty AlertRule = %v, want ErrInvalidRule", err)
	}

	badSeverity := &alerting.AlertRule{
		Name:      "r1",
		Condition: alerting.Condition{Kind: alerting.ConditionThresholdAbove, MetricName: "m"},
		Severity:  "bogus",
	}
	if err := badSeverity.Validate(); !errors.Is(err, alerting.ErrInvalidSeverity) {
		t.Errorf("Validate() with invalid Severity = %v, want ErrInvalidSeverity", err)
	}

	var nilRule *alerting.AlertRule
	if err := nilRule.Validate(); !errors.Is(err, alerting.ErrInvalidRule) {
		t.Errorf("Validate() on nil *AlertRule = %v, want ErrInvalidRule", err)
	}
}

func TestAlertEvent_Validate(t *testing.T) {
	t.Parallel()

	valid := &alerting.AlertEvent{
		TenantID: uuid.New(),
		RuleName: "r1",
		Severity: alerting.SeverityCritical,
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed AlertEvent = %v, want nil", err)
	}

	missingTenant := &alerting.AlertEvent{RuleName: "r1", Severity: alerting.SeverityCritical}
	if err := missingTenant.Validate(); !errors.Is(err, alerting.ErrEmptyTenantID) {
		t.Errorf("Validate() with empty TenantID = %v, want ErrEmptyTenantID", err)
	}

	missingRuleName := &alerting.AlertEvent{TenantID: uuid.New(), Severity: alerting.SeverityCritical}
	if err := missingRuleName.Validate(); !errors.Is(err, alerting.ErrInvalidEvent) {
		t.Errorf("Validate() with blank RuleName = %v, want ErrInvalidEvent", err)
	}

	var nilEvent *alerting.AlertEvent
	if err := nilEvent.Validate(); !errors.Is(err, alerting.ErrInvalidEvent) {
		t.Errorf("Validate() on nil *AlertEvent = %v, want ErrInvalidEvent", err)
	}
}
