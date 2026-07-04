package cicdgate

import (
	"errors"
	"testing"
)

func validRollbackTrigger() RollbackTrigger {
	return RollbackTrigger{
		ReleaseName:     "cicdgate-v0.1.0",
		Stage:           StageCanary,
		MaxErrorRate:    0.01,
		MaxLatencyP99Ms: 500,
	}
}

func TestRollbackTrigger_Validate(t *testing.T) {
	t.Run("valid trigger", func(t *testing.T) {
		tr := validRollbackTrigger()
		if err := tr.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var tr *RollbackTrigger
		if err := tr.Validate(); !errors.Is(err, ErrInvalidRollbackTrigger) {
			t.Errorf("Validate() error = %v, want ErrInvalidRollbackTrigger", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*RollbackTrigger)
	}{
		{name: "blank release name", mutate: func(tr *RollbackTrigger) { tr.ReleaseName = "" }},
		{name: "unrecognized stage", mutate: func(tr *RollbackTrigger) { tr.Stage = "bogus" }},
		{name: "negative max error rate", mutate: func(tr *RollbackTrigger) { tr.MaxErrorRate = -0.1 }},
		{name: "max error rate above 1", mutate: func(tr *RollbackTrigger) { tr.MaxErrorRate = 1.1 }},
		{name: "zero max latency", mutate: func(tr *RollbackTrigger) { tr.MaxLatencyP99Ms = 0 }},
		{name: "negative max latency", mutate: func(tr *RollbackTrigger) { tr.MaxLatencyP99Ms = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := validRollbackTrigger()
			tt.mutate(&tr)
			if err := tr.Validate(); !errors.Is(err, ErrInvalidRollbackTrigger) {
				t.Errorf("Validate() error = %v, want wrapping ErrInvalidRollbackTrigger", err)
			}
		})
	}

	t.Run("boundary max error rate of exactly 1 is valid", func(t *testing.T) {
		tr := validRollbackTrigger()
		tr.MaxErrorRate = 1
		if err := tr.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("boundary max error rate of exactly 0 is valid", func(t *testing.T) {
		tr := validRollbackTrigger()
		tr.MaxErrorRate = 0
		if err := tr.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})
}

func TestEvaluateRollback(t *testing.T) {
	trigger := validRollbackTrigger() // MaxErrorRate 0.01, MaxLatencyP99Ms 500

	t.Run("invalid trigger", func(t *testing.T) {
		bad := trigger
		bad.ReleaseName = ""
		_, err := EvaluateRollback(&bad, []StageHealth{{ErrorRate: 0.5}})
		if !errors.Is(err, ErrInvalidRollbackTrigger) {
			t.Errorf("EvaluateRollback() error = %v, want wrapping ErrInvalidRollbackTrigger", err)
		}
	})

	t.Run("no samples", func(t *testing.T) {
		_, err := EvaluateRollback(&trigger, nil)
		if !errors.Is(err, ErrRollbackConditionNotMet) {
			t.Errorf("EvaluateRollback() error = %v, want wrapping ErrRollbackConditionNotMet", err)
		}
	})

	t.Run("latest sample healthy, no rollback", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.5, LatencyP99Ms: 5000}, // an old bad sample, since superseded
			{ErrorRate: 0.001, LatencyP99Ms: 100},
		}
		_, err := EvaluateRollback(&trigger, samples)
		if !errors.Is(err, ErrRollbackConditionNotMet) {
			t.Errorf("EvaluateRollback() error = %v, want wrapping ErrRollbackConditionNotMet (only the latest sample matters)", err)
		}
	})

	t.Run("latest sample exceeds error rate ceiling", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.5, LatencyP99Ms: 100, Stage: StageCanary},
		}
		decision, err := EvaluateRollback(&trigger, samples)
		if err != nil {
			t.Fatalf("EvaluateRollback() error = %v, want nil", err)
		}
		if decision.Reason != RollbackReasonErrorRate {
			t.Errorf("EvaluateRollback() reason = %q, want %q", decision.Reason, RollbackReasonErrorRate)
		}
		if decision.Stage != trigger.Stage {
			t.Errorf("EvaluateRollback() stage = %q, want %q", decision.Stage, trigger.Stage)
		}
		if decision.Observed != samples[1] {
			t.Errorf("EvaluateRollback() observed = %+v, want %+v", decision.Observed, samples[1])
		}
	})

	t.Run("latest sample exceeds latency ceiling", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.001, LatencyP99Ms: 5000},
		}
		decision, err := EvaluateRollback(&trigger, samples)
		if err != nil {
			t.Fatalf("EvaluateRollback() error = %v, want nil", err)
		}
		if decision.Reason != RollbackReasonLatency {
			t.Errorf("EvaluateRollback() reason = %q, want %q", decision.Reason, RollbackReasonLatency)
		}
	})

	t.Run("error rate takes precedence when both ceilings exceeded", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.5, LatencyP99Ms: 5000},
		}
		decision, err := EvaluateRollback(&trigger, samples)
		if err != nil {
			t.Fatalf("EvaluateRollback() error = %v, want nil", err)
		}
		if decision.Reason != RollbackReasonErrorRate {
			t.Errorf("EvaluateRollback() reason = %q, want %q", decision.Reason, RollbackReasonErrorRate)
		}
	})

	t.Run("latest sample exactly at ceilings does not roll back", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.01, LatencyP99Ms: 500},
		}
		_, err := EvaluateRollback(&trigger, samples)
		if !errors.Is(err, ErrRollbackConditionNotMet) {
			t.Errorf("EvaluateRollback() error = %v, want wrapping ErrRollbackConditionNotMet", err)
		}
	})
}
