package alerting_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func qualityScores(runID string, overalls ...float64) []reasoningeval.QualityScore {
	out := make([]reasoningeval.QualityScore, 0, len(overalls))
	for _, v := range overalls {
		out = append(out, reasoningeval.QualityScore{
			CaseID:  "case-" + uuid.NewString(),
			RunID:   runID,
			Overall: v,
		})
	}
	return out
}

func TestEvaluateQualityAlert_FiresOnRegression(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()
	rule := alerting.QualityAlertRule{
		RuleName: "reasoning-quality",
		Detector: reasoningeval.NewRegressionDetector(0.05),
		Severity: alerting.SeverityWarning,
	}

	baseline := qualityScores("run-baseline", 0.90, 0.92, 0.88)
	current := qualityScores("run-current", 0.60, 0.62, 0.58) // a large drop

	event, fired, err := alerting.EvaluateQualityAlert(tenantID, rule, baseline, current, now)
	if err != nil {
		t.Fatalf("EvaluateQualityAlert: %v", err)
	}
	if !fired {
		t.Fatal("EvaluateQualityAlert did not fire for a large quality drop")
	}
	if event.Severity != alerting.SeverityWarning {
		t.Errorf("event.Severity = %v, want SeverityWarning", event.Severity)
	}
	if event.ConditionKind != alerting.ConditionQualityRegression {
		t.Errorf("event.ConditionKind = %v, want ConditionQualityRegression", event.ConditionKind)
	}
	if event.TenantID != tenantID {
		t.Errorf("event.TenantID = %v, want %v", event.TenantID, tenantID)
	}
	if event.Detail == "" {
		t.Error("event.Detail is empty, want a human-readable explanation")
	}
}

func TestEvaluateQualityAlert_DoesNotFireWithoutRegression(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()
	rule := alerting.QualityAlertRule{
		RuleName: "reasoning-quality",
		Detector: reasoningeval.NewRegressionDetector(0.05),
		Severity: alerting.SeverityWarning,
	}

	baseline := qualityScores("run-baseline", 0.90, 0.92, 0.88)
	current := qualityScores("run-current", 0.91, 0.89, 0.90) // no meaningful drop

	_, fired, err := alerting.EvaluateQualityAlert(tenantID, rule, baseline, current, now)
	if err != nil {
		t.Fatalf("EvaluateQualityAlert: %v", err)
	}
	if fired {
		t.Fatal("EvaluateQualityAlert fired without a meaningful quality drop")
	}
}

func TestEvaluateQualityAlert_InvalidRule(t *testing.T) {
	t.Parallel()
	_, _, err := alerting.EvaluateQualityAlert(uuid.New(), alerting.QualityAlertRule{}, nil, nil, time.Now())
	if !errors.Is(err, alerting.ErrInvalidRule) {
		t.Fatalf("EvaluateQualityAlert with nil Detector error = %v, want ErrInvalidRule", err)
	}
}

func TestEvaluateQualityAlert_PropagatesNoScoresError(t *testing.T) {
	t.Parallel()
	rule := alerting.QualityAlertRule{
		RuleName: "reasoning-quality",
		Detector: reasoningeval.NewRegressionDetector(0.05),
		Severity: alerting.SeverityWarning,
	}
	_, _, err := alerting.EvaluateQualityAlert(uuid.New(), rule, nil, qualityScores("run-current", 0.9), time.Now())
	if !errors.Is(err, reasoningeval.ErrNoScores) {
		t.Fatalf("EvaluateQualityAlert with empty baseline error = %v, want reasoningeval.ErrNoScores", err)
	}
}
