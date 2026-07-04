package alerting

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Severity classifies how urgently an AlertEvent needs a human
// response, gating both dashboard sorting and EscalationPolicy
// routing (a Critical alert may skip straight to a later tier's
// on-call schedule, depending on policy configuration).
type Severity string

const (
	// SeverityInfo is informational: worth recording, not worth paging
	// anyone.
	SeverityInfo Severity = "info"

	// SeverityWarning indicates a developing problem that does not yet
	// require immediate human intervention (e.g. an error budget at
	// 60% consumed).
	SeverityWarning Severity = "warning"

	// SeverityCritical indicates an active incident requiring prompt
	// human response (e.g. an SLO currently breached, an error budget
	// exhausted).
	SeverityCritical Severity = "critical"
)

// IsValid reports whether s is one of the named Severity constants.
func (s Severity) IsValid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s Severity) String() string { return string(s) }

// rank returns an ordinal for sorting/comparison, higher meaning more
// severe. Unknown values rank below SeverityInfo.
func (s Severity) rank() int {
	switch s {
	case SeverityInfo:
		return 1
	case SeverityWarning:
		return 2
	case SeverityCritical:
		return 3
	default:
		return 0
	}
}

// AtLeast reports whether s is at least as severe as other.
func (s Severity) AtLeast(other Severity) bool {
	return s.rank() >= other.rank()
}

// ConditionKind names what kind of comparison an AlertRule's Condition
// performs.
type ConditionKind string

const (
	// ConditionThresholdAbove fires when the observed value exceeds
	// Condition.Threshold.
	ConditionThresholdAbove ConditionKind = "threshold_above"

	// ConditionThresholdBelow fires when the observed value falls
	// below Condition.Threshold.
	ConditionThresholdBelow ConditionKind = "threshold_below"

	// ConditionSLOBreached fires from an externally computed
	// reliability.SLOStatus/ErrorBudget signal -- see
	// slo_alert.go's EvaluateSLOAlert, which is the only code path
	// that produces this condition's AlertEvent. Engine.Evaluate does
	// not itself know how to compute an SLO breach; it only records
	// that this AlertRule's condition kind is "externally evaluated"
	// so the rule can still be catalogued and displayed alongside
	// plain threshold rules.
	ConditionSLOBreached ConditionKind = "slo_breached"

	// ConditionQualityRegression fires from an externally computed
	// reasoningeval.RegressionResult signal -- see
	// quality_alert.go's EvaluateQualityAlert.
	ConditionQualityRegression ConditionKind = "quality_regression"

	// ConditionCostThreshold fires from an externally computed
	// accounting budget-threshold signal -- see cost_alert.go's
	// EvaluateCostAlert.
	ConditionCostThreshold ConditionKind = "cost_threshold"
)

// IsValid reports whether k is one of the named ConditionKind
// constants.
func (k ConditionKind) IsValid() bool {
	switch k {
	case ConditionThresholdAbove, ConditionThresholdBelow, ConditionSLOBreached,
		ConditionQualityRegression, ConditionCostThreshold:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (k ConditionKind) String() string { return string(k) }

// externallyEvaluated reports whether k's AlertEvents are produced by
// a dedicated Evaluate* function (slo_alert.go, quality_alert.go,
// cost_alert.go) rather than by Engine.Evaluate's own threshold
// comparison.
func (k ConditionKind) externallyEvaluated() bool {
	switch k {
	case ConditionSLOBreached, ConditionQualityRegression, ConditionCostThreshold:
		return true
	}
	return false
}

// Condition is the trigger an AlertRule evaluates. For
// ConditionThresholdAbove/Below, MetricName and Threshold are the only
// fields Engine.Evaluate consults. For the three externally-evaluated
// kinds (ConditionSLOBreached, ConditionQualityRegression,
// ConditionCostThreshold), Condition only records *that* this rule is
// evaluated by the corresponding Evaluate* function elsewhere in this
// package -- MetricName/Threshold are ignored by Engine.Evaluate for
// those kinds, but may still be populated for display purposes (e.g.
// naming which reliability.SLO.Name this rule wraps).
type Condition struct {
	// Kind selects the comparison Engine.Evaluate performs (for
	// threshold kinds) or the external signal this rule represents
	// (for the other three kinds).
	Kind ConditionKind `json:"kind"`

	// MetricName names the Catalogue metric (metrics.go) this
	// condition reads, for ConditionThresholdAbove/Below. For the
	// three externally-evaluated kinds, MetricName is a free-form
	// display label (e.g. an reliability.SLO.Name, a
	// reasoningeval jurisdiction code, an accounting tenant budget
	// name) rather than a Catalogue lookup key.
	MetricName string `json:"metric_name"`

	// Threshold is the comparison value for ConditionThresholdAbove/
	// Below. Ignored for the three externally-evaluated kinds.
	Threshold float64 `json:"threshold"`
}

// Validate checks c for structural well-formedness.
func (c Condition) Validate() error {
	if !c.Kind.IsValid() {
		return wrapf("Condition.Validate", ErrInvalidRule)
	}
	if strings.TrimSpace(c.MetricName) == "" {
		return wrapf("Condition.Validate", ErrInvalidRule)
	}
	return nil
}

// AlertRule is a named, catalogued alerting rule (task 3-5's shared
// mechanism): a Condition plus a Severity. AlertRule is shared
// reference/configuration data scoped to a tenant (a tenant may
// customize its own threshold for a shared rule name), mirroring
// packages/compliance.Control's catalogue-row shape but, unlike
// Control, tenant-scoped from the start since alert thresholds are
// inherently a per-deployment operational choice (a tenant with heavy
// nightly batch ingestion may want a higher "cases_ingested_total"
// floor than a light-traffic tenant).
type AlertRule struct {
	// ID uniquely identifies this rule.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this rule to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// Name is a short, stable, human-referenceable identifier (e.g.
	// "ingestion-slo-availability", "reasoning-quality-regression"),
	// unique per tenant.
	Name string `json:"name"`

	// Description explains what this rule watches for and why.
	Description string `json:"description"`

	// Condition is this rule's trigger.
	Condition Condition `json:"condition"`

	// Severity classifies how urgently a firing of this rule needs a
	// human response.
	Severity Severity `json:"severity"`

	// RunbookName, if non-empty, names the Runbook (runbook.go)
	// attached to this rule -- the remediation procedure an on-call
	// responder should follow when this rule fires.
	RunbookName string `json:"runbook_name,omitempty"`

	// CreatedBy is the identity.User who registered this rule.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks r for structural well-formedness.
func (r *AlertRule) Validate() error {
	if r == nil {
		return ErrInvalidRule
	}
	if strings.TrimSpace(r.Name) == "" {
		return wrapf("AlertRule.Validate", ErrInvalidRule)
	}
	if err := r.Condition.Validate(); err != nil {
		return wrapf("AlertRule.Validate", err)
	}
	if !r.Severity.IsValid() {
		return wrapf("AlertRule.Validate", ErrInvalidSeverity)
	}
	return nil
}

// AlertEvent is a single firing of an AlertRule (the uniform shape
// every alert kind in this package -- plain threshold, SLO, quality
// regression, cost -- resolves to; see doc.go's "Alert vs AlertEvent"
// section for why this is not simply reasoningeval.Alert or
// accounting.AlertEvent reused directly).
type AlertEvent struct {
	// ID uniquely identifies this event.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this event to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// RuleID and RuleName identify the AlertRule that fired. RuleName
	// is denormalized onto the event so history remains readable even
	// if the rule is later renamed or deleted.
	RuleID   uuid.UUID `json:"rule_id"`
	RuleName string    `json:"rule_name"`

	// Severity is copied from the firing AlertRule at the time this
	// event was raised.
	Severity Severity `json:"severity"`

	// ConditionKind is copied from the firing AlertRule's Condition.
	ConditionKind ConditionKind `json:"condition_kind"`

	// TriggerValue is the observed value that caused this event: the
	// metric reading for a threshold condition, the observed SLO
	// value, the current-run quality average, or the current
	// usage/limit ratio.
	TriggerValue float64 `json:"trigger_value"`

	// Threshold is the configured comparison value the TriggerValue
	// was measured against, for display alongside TriggerValue.
	Threshold float64 `json:"threshold"`

	// Detail is a short, human-readable summary of why this event
	// fired.
	Detail string `json:"detail"`

	// CreatedAt records when this event was raised.
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks e for structural well-formedness.
func (e *AlertEvent) Validate() error {
	if e == nil {
		return ErrInvalidEvent
	}
	if e.TenantID == uuid.Nil {
		return wrapf("AlertEvent.Validate", ErrEmptyTenantID)
	}
	if strings.TrimSpace(e.RuleName) == "" {
		return wrapf("AlertEvent.Validate", ErrInvalidEvent)
	}
	if !e.Severity.IsValid() {
		return wrapf("AlertEvent.Validate", ErrInvalidSeverity)
	}
	return nil
}
