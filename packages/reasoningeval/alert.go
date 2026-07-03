package reasoningeval

import (
	"context"
	"fmt"
	"log"
	"time"
)

// AlertKind classifies the kind of quality event an Alert represents.
type AlertKind string

const (
	// AlertKindRegression fires when RegressionDetector flags a
	// threshold-exceeding drop between two runs.
	AlertKindRegression AlertKind = "regression"
)

// Alert is a single quality-drop notification, mirroring
// packages/accounting.AlertEvent's and packages/reasoningprofile.Event's
// shape: a structured record describing what tripped, for what scope,
// carrying the guardrail non-binding label since an Alert message
// surfaces reasoning-quality conclusions about a case that must never be
// read as authoritative.
type Alert struct {
	// Kind classifies this alert.
	Kind AlertKind

	// JurisdictionCode is the jurisdiction the regression was detected in,
	// empty if the regression was global (not jurisdiction-scoped).
	JurisdictionCode string

	// Regression carries the full RegressionResult that triggered this
	// alert.
	Regression RegressionResult

	// Message is a short human-readable summary of the alert, always
	// suffixed with the guardrail non-binding label (see NewRegressionAlert).
	Message string

	// CreatedAt records when this Alert was raised.
	CreatedAt time.Time
}

// nonBindingAlertSuffix is appended to every Alert.Message so a quality
// alert is never mistaken for a conclusion about the underlying case's
// merits — it is a signal about the reasoning pipeline's own output
// quality, not a legal determination. Mirrors
// packages/guardrail.DraftAnalysisLabel's wording without importing
// packages/guardrail solely for one string, since an Alert is a
// monitoring artifact, not a synthesisagent.Opinion or irac.ConclusionNode
// guardrail itself gates.
const nonBindingAlertSuffix = " (non-binding quality signal; not a legal determination)"

// NewRegressionAlert builds an Alert from a RegressionResult that was
// flagged as regressed.
func NewRegressionAlert(jurisdictionCode string, result RegressionResult) Alert {
	msg := fmt.Sprintf(
		"quality regression: run %q avg %.4f vs baseline %q avg %.4f (drop=%.4f)",
		result.CurrentRunID, result.CurrentAvg, result.BaselineRunID, result.BaselineAvg, result.Drop,
	) + nonBindingAlertSuffix

	return Alert{
		Kind:             AlertKindRegression,
		JurisdictionCode: jurisdictionCode,
		Regression:       result,
		Message:          msg,
		CreatedAt:        nowFunc().UTC(),
	}
}

// AlertSink receives Alerts for delivery to an external system (e.g. a
// paging service, Slack webhook, or metrics platform), mirroring
// packages/accounting.AlertSink and packages/reasoningprofile.AlertSink's
// interface shape exactly.
type AlertSink interface {
	// Send delivers an Alert. Implementations should be fast and
	// non-blocking; heavy I/O should be offloaded to a goroutine.
	Send(ctx context.Context, alert Alert) error
}

// LoggingAlertSink is an AlertSink that writes alerts to the standard
// logger, mirroring packages/accounting.LoggingAlertSink. Reuses
// packages/observability's AuditLogger convention of a distinct,
// structured sink rather than commingling with application logs, but
// implemented directly here (not via observability.AuditLogger) since an
// Alert is not itself an observability.AuditEvent — it is a
// quality-monitoring signal one layer removed from "who did what to
// what".
type LoggingAlertSink struct {
	Logger *log.Logger
}

// Send implements AlertSink by writing the alert to the configured
// logger.
func (s *LoggingAlertSink) Send(_ context.Context, alert Alert) error {
	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"[reasoningeval] alert kind=%s jurisdiction=%s message=%q at=%s",
		alert.Kind, alert.JurisdictionCode, alert.Message, alert.CreatedAt.Format(time.RFC3339),
	)
	return nil
}

// NoOpAlertSink is an AlertSink that silently discards every alert.
type NoOpAlertSink struct{}

// Send implements AlertSink by doing nothing.
func (NoOpAlertSink) Send(_ context.Context, _ Alert) error { return nil }

// MultiAlertSink fans an Alert out to multiple AlertSink implementations.
// The first error encountered is returned but all sinks are still
// attempted, mirroring packages/accounting.MultiAlertSink.
type MultiAlertSink struct {
	Sinks []AlertSink
}

// Send implements AlertSink by calling Send on each child sink.
func (m *MultiAlertSink) Send(ctx context.Context, alert Alert) error {
	var first error
	for _, s := range m.Sinks {
		if err := s.Send(ctx, alert); err != nil && first == nil {
			first = fmt.Errorf("reasoningeval: alert sink error: %w", err)
		}
	}
	return first
}

// QualityAlertChecker wires a RegressionDetector's threshold check to an
// AlertSink: every time Check is called with a baseline/current pair, it
// runs the comparison and — only when a regression is flagged — sends
// exactly one Alert to Sink.
type QualityAlertChecker struct {
	// Detector performs the regression comparison.
	Detector *RegressionDetector

	// Sink receives an Alert only when Detector flags a regression.
	Sink AlertSink
}

// NewQualityAlertChecker constructs a QualityAlertChecker with the given
// threshold and sink. A nil sink defaults to NoOpAlertSink.
func NewQualityAlertChecker(threshold float64, sink AlertSink) *QualityAlertChecker {
	if sink == nil {
		sink = NoOpAlertSink{}
	}
	return &QualityAlertChecker{
		Detector: NewRegressionDetector(threshold),
		Sink:     sink,
	}
}

// Check compares baseline against current for the given jurisdictionCode
// (empty for a global, non-jurisdiction-scoped comparison). If the
// comparison flags a regression, exactly one Alert is sent to c.Sink.
// Returns the RegressionResult regardless of whether an alert fired, so
// callers can inspect the comparison even when it did not cross the
// threshold.
func (c *QualityAlertChecker) Check(ctx context.Context, jurisdictionCode string, baseline, current []QualityScore) (RegressionResult, error) {
	result, err := c.Detector.Compare(baseline, current)
	if err != nil {
		return RegressionResult{}, err
	}
	if !result.Regressed {
		return result, nil
	}
	alert := NewRegressionAlert(jurisdictionCode, result)
	if err := c.Sink.Send(ctx, alert); err != nil {
		return result, err
	}
	return result, nil
}
