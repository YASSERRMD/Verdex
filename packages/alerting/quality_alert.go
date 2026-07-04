// Package alerting's quality_alert.go implements task 4,
// reasoning-quality alerts, by composing directly with
// packages/reasoningeval's existing RegressionDetector/RegressionResult
// (Phase 062) rather than redefining a second regression-detection
// concept. See doc.go's "Alert vs AlertEvent" section for why this
// file converts a RegressionResult into this package's own AlertEvent
// instead of reusing reasoningeval.Alert directly: reasoningeval.Alert
// is a jurisdiction-scoped signal with no Severity/RunbookName/
// EscalationPolicy attachment point of its own, while this package's
// AlertEvent needs to flow through the exact same
// Engine/Route/Runbook pipeline every other alert kind here does.
package alerting

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

// QualityAlertRule wraps a reasoningeval.RegressionDetector threshold
// plus this package's own Severity/RunbookName, so a reasoning-quality
// regression rule can be catalogued alongside every other alert kind.
type QualityAlertRule struct {
	// RuleName identifies this rule.
	RuleName string

	// Detector performs the baseline/current QualityScore comparison.
	// Required.
	Detector *reasoningeval.RegressionDetector

	// JurisdictionCode is passed through to
	// reasoningeval.NewRegressionAlert for display purposes; empty
	// means a global, non-jurisdiction-scoped comparison.
	JurisdictionCode string

	// Severity is the AlertEvent.Severity assigned when this rule
	// fires.
	Severity Severity

	// RunbookName, if non-empty, names the Runbook a responder should
	// follow for this rule.
	RunbookName string
}

// Validate checks r for structural well-formedness.
func (r QualityAlertRule) Validate() error {
	if r.RuleName == "" {
		return wrapf("QualityAlertRule.Validate", ErrInvalidRule)
	}
	if r.Detector == nil {
		return wrapf("QualityAlertRule.Validate", ErrInvalidRule)
	}
	if !r.Severity.IsValid() {
		return wrapf("QualityAlertRule.Validate", ErrInvalidSeverity)
	}
	return nil
}

// EvaluateQualityAlert runs rule.Detector.Compare(baseline, current) --
// the exact same comparison packages/reasoningeval.QualityAlertChecker
// itself performs -- and, only when the comparison reports Regressed,
// converts the resulting reasoningeval.RegressionResult into this
// package's own AlertEvent. Real composition: this function never
// recomputes an average Overall score or a per-dimension drop itself,
// it only calls into packages/reasoningeval and translates the
// result.
//
// Returns (AlertEvent{}, false, nil) when no regression is flagged --
// not firing is not an error.
func EvaluateQualityAlert(tenantID uuid.UUID, rule QualityAlertRule, baseline, current []reasoningeval.QualityScore, now time.Time) (AlertEvent, bool, error) {
	if err := rule.Validate(); err != nil {
		return AlertEvent{}, false, wrapf("EvaluateQualityAlert", err)
	}

	result, err := rule.Detector.Compare(baseline, current)
	if err != nil {
		return AlertEvent{}, false, wrapf("EvaluateQualityAlert", err)
	}
	if !result.Regressed {
		return AlertEvent{}, false, nil
	}

	// Reuse reasoningeval's own alert-message construction (including
	// its non-binding-quality-signal suffix) for the Detail field, so
	// this package's translation never drifts from how
	// packages/reasoningeval itself describes a regression.
	upstream := reasoningeval.NewRegressionAlert(rule.JurisdictionCode, result)

	event := AlertEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		RuleName:      rule.RuleName,
		Severity:      rule.Severity,
		ConditionKind: ConditionQualityRegression,
		TriggerValue:  result.CurrentAvg,
		Threshold:     result.BaselineAvg - rule.Detector.Threshold,
		Detail:        fmt.Sprintf("%s [run=%s]", upstream.Message, result.CurrentRunID),
		CreatedAt:     now,
	}
	return event, true, nil
}
