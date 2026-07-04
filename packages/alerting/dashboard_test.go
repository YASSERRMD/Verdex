package alerting_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

func TestBuildDashboard_KnownFlows(t *testing.T) {
	t.Parallel()
	now := time.Now()

	for _, flow := range alerting.KnownFlows() {
		dashboard, err := alerting.BuildDashboard(flow, now)
		if err != nil {
			t.Fatalf("BuildDashboard(%q): %v", flow, err)
		}
		if dashboard.FlowName != flow {
			t.Errorf("dashboard.FlowName = %q, want %q", dashboard.FlowName, flow)
		}
		if len(dashboard.Panels) == 0 {
			t.Errorf("BuildDashboard(%q) returned no panels", flow)
		}
		for _, panel := range dashboard.Panels {
			if panel.Title == "" {
				t.Errorf("BuildDashboard(%q) panel has empty Title", flow)
			}
			if panel.MetricName == "" {
				t.Errorf("BuildDashboard(%q) panel has empty MetricName", flow)
			}
		}
		if !dashboard.GeneratedAt.Equal(now) {
			t.Errorf("dashboard.GeneratedAt = %v, want %v", dashboard.GeneratedAt, now)
		}
	}
}

func TestBuildDashboard_UnknownFlow(t *testing.T) {
	t.Parallel()
	_, err := alerting.BuildDashboard("nonexistent-flow", time.Now())
	if !errors.Is(err, alerting.ErrUnknownFlow) {
		t.Fatalf("BuildDashboard(unknown) error = %v, want ErrUnknownFlow", err)
	}
}

func TestKnownFlows_ContainsExpectedNames(t *testing.T) {
	t.Parallel()
	flows := alerting.KnownFlows()
	want := map[string]bool{"ingestion": false, "reasoning": false, "sign-off": false}
	for _, f := range flows {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("KnownFlows() missing expected flow %q", name)
		}
	}
}

func TestBuildDashboard_ReturnsIndependentCopy(t *testing.T) {
	t.Parallel()
	d1, err := alerting.BuildDashboard("ingestion", time.Now())
	if err != nil {
		t.Fatalf("BuildDashboard: %v", err)
	}
	d1.Panels[0].Title = "mutated"

	d2, err := alerting.BuildDashboard("ingestion", time.Now())
	if err != nil {
		t.Fatalf("BuildDashboard: %v", err)
	}
	if d2.Panels[0].Title == "mutated" {
		t.Error("mutating one BuildDashboard result's Panels affected a subsequent call, want independent copies")
	}
}
