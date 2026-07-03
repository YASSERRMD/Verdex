package reasoningeval_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

// captureSink is an AlertSink that records every Alert it receives, for
// assertions on whether/how many alerts fired.
type captureSink struct {
	alerts []reasoningeval.Alert
}

func (c *captureSink) Send(_ context.Context, alert reasoningeval.Alert) error {
	c.alerts = append(c.alerts, alert)
	return nil
}

func TestQualityAlertChecker_FiresOnlyWhenThresholdCrossed(t *testing.T) {
	baseline := scoresWithOverall("v1", 0.90, 0.92, 0.88)
	regressed := scoresWithOverall("v2", 0.5, 0.52, 0.48)
	noisy := scoresWithOverall("v3", 0.895, 0.905, 0.885)

	sink := &captureSink{}
	checker := reasoningeval.NewQualityAlertChecker(0.05, sink)

	// First comparison: a real regression, should fire exactly one alert.
	result, err := checker.Check(context.Background(), "AE-DXB", baseline, regressed)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.Regressed {
		t.Fatal("expected Regressed = true for the real regression")
	}
	if len(sink.alerts) != 1 {
		t.Fatalf("len(sink.alerts) = %d, want 1 after a real regression", len(sink.alerts))
	}
	if sink.alerts[0].JurisdictionCode != "AE-DXB" {
		t.Errorf("alert JurisdictionCode = %q, want AE-DXB", sink.alerts[0].JurisdictionCode)
	}
	if sink.alerts[0].Kind != reasoningeval.AlertKindRegression {
		t.Errorf("alert Kind = %q, want %q", sink.alerts[0].Kind, reasoningeval.AlertKindRegression)
	}

	// Second comparison: noise-level drop, should NOT fire another alert.
	result, err = checker.Check(context.Background(), "AE-DXB", baseline, noisy)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Regressed {
		t.Fatal("expected Regressed = false for the noise-level drop")
	}
	if len(sink.alerts) != 1 {
		t.Fatalf("len(sink.alerts) = %d after noise comparison, want still 1 (no new alert)", len(sink.alerts))
	}
}

func TestQualityAlertChecker_NilSinkDefaultsToNoOp(t *testing.T) {
	checker := reasoningeval.NewQualityAlertChecker(0.05, nil)
	baseline := scoresWithOverall("v1", 0.9)
	regressed := scoresWithOverall("v2", 0.1)

	// Must not panic with a nil sink, and must still report the regression.
	result, err := checker.Check(context.Background(), "", baseline, regressed)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.Regressed {
		t.Fatal("expected Regressed = true")
	}
}

func TestMultiAlertSink_FansOutToAllSinks(t *testing.T) {
	sinkA := &captureSink{}
	sinkB := &captureSink{}
	multi := &reasoningeval.MultiAlertSink{Sinks: []reasoningeval.AlertSink{sinkA, sinkB}}

	alert := reasoningeval.NewRegressionAlert("AE-DXB", reasoningeval.RegressionResult{
		BaselineRunID: "v1", CurrentRunID: "v2", BaselineAvg: 0.9, CurrentAvg: 0.5, Drop: 0.4, Regressed: true,
	})

	if err := multi.Send(context.Background(), alert); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(sinkA.alerts) != 1 || len(sinkB.alerts) != 1 {
		t.Errorf("expected both sinks to receive the alert, got sinkA=%d sinkB=%d", len(sinkA.alerts), len(sinkB.alerts))
	}
}
