package cicdgate

import (
	"errors"
	"testing"
)

func validRolloutTrigger() RolloutTrigger {
	return RolloutTrigger{
		ReleaseName:            "cicdgate-v0.1.0",
		TargetStage:            StagePartial,
		RequiredHealthySamples: 3,
	}
}

func TestRolloutTrigger_Validate(t *testing.T) {
	t.Run("valid trigger", func(t *testing.T) {
		tr := validRolloutTrigger()
		if err := tr.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var tr *RolloutTrigger
		if err := tr.Validate(); !errors.Is(err, ErrInvalidRolloutTrigger) {
			t.Errorf("Validate() error = %v, want ErrInvalidRolloutTrigger", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*RolloutTrigger)
	}{
		{name: "blank release name", mutate: func(tr *RolloutTrigger) { tr.ReleaseName = "  " }},
		{name: "unrecognized target stage", mutate: func(tr *RolloutTrigger) { tr.TargetStage = "bogus" }},
		{name: "zero required healthy samples", mutate: func(tr *RolloutTrigger) { tr.RequiredHealthySamples = 0 }},
		{name: "negative required healthy samples", mutate: func(tr *RolloutTrigger) { tr.RequiredHealthySamples = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := validRolloutTrigger()
			tt.mutate(&tr)
			if err := tr.Validate(); !errors.Is(err, ErrInvalidRolloutTrigger) {
				t.Errorf("Validate() error = %v, want wrapping ErrInvalidRolloutTrigger", err)
			}
		})
	}
}

func TestRolloutStage_IsValid(t *testing.T) {
	tests := []struct {
		stage RolloutStage
		want  bool
	}{
		{StageCanary, true},
		{StagePartial, true},
		{StageFull, true},
		{"bogus", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.stage.IsValid(); got != tt.want {
			t.Errorf("%q.IsValid() = %v, want %v", tt.stage, got, tt.want)
		}
	}
}

func TestNextStage(t *testing.T) {
	tests := []struct {
		name      string
		current   RolloutStage
		wantStage RolloutStage
		wantOK    bool
	}{
		{name: "canary to partial", current: StageCanary, wantStage: StagePartial, wantOK: true},
		{name: "partial to full", current: StagePartial, wantStage: StageFull, wantOK: true},
		{name: "full has no next stage", current: StageFull, wantStage: "", wantOK: false},
		{name: "unrecognized stage has no next stage", current: "bogus", wantStage: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStage, gotOK := NextStage(tt.current)
			if gotStage != tt.wantStage || gotOK != tt.wantOK {
				t.Errorf("NextStage(%q) = (%q, %v), want (%q, %v)", tt.current, gotStage, gotOK, tt.wantStage, tt.wantOK)
			}
		})
	}
}

func TestStageHealth_IsHealthy(t *testing.T) {
	tests := []struct {
		name string
		h    StageHealth
		want bool
	}{
		{name: "within both thresholds", h: StageHealth{ErrorRate: 0.001, LatencyP99Ms: 100}, want: true},
		{name: "exactly at thresholds", h: StageHealth{ErrorRate: 0.01, LatencyP99Ms: 500}, want: true},
		{name: "error rate exceeds threshold", h: StageHealth{ErrorRate: 0.02, LatencyP99Ms: 100}, want: false},
		{name: "latency exceeds threshold", h: StageHealth{ErrorRate: 0.001, LatencyP99Ms: 600}, want: false},
		{name: "both exceed threshold", h: StageHealth{ErrorRate: 0.5, LatencyP99Ms: 5000}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.IsHealthy(0.01, 500); got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluatePromotion(t *testing.T) {
	trigger := validRolloutTrigger() // RequiredHealthySamples: 3

	t.Run("invalid trigger", func(t *testing.T) {
		bad := trigger
		bad.ReleaseName = ""
		_, err := EvaluatePromotion(&bad, nil, 0.01, 500)
		if !errors.Is(err, ErrInvalidRolloutTrigger) {
			t.Errorf("EvaluatePromotion() error = %v, want wrapping ErrInvalidRolloutTrigger", err)
		}
	})

	t.Run("exactly enough consecutive healthy samples", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.002, LatencyP99Ms: 110},
			{ErrorRate: 0.001, LatencyP99Ms: 120},
		}
		stage, err := EvaluatePromotion(&trigger, samples, 0.01, 500)
		if err != nil {
			t.Fatalf("EvaluatePromotion() error = %v, want nil", err)
		}
		if stage != StagePartial {
			t.Errorf("EvaluatePromotion() stage = %q, want %q", stage, StagePartial)
		}
	})

	t.Run("more than enough consecutive healthy samples", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
		}
		stage, err := EvaluatePromotion(&trigger, samples, 0.01, 500)
		if err != nil {
			t.Fatalf("EvaluatePromotion() error = %v, want nil", err)
		}
		if stage != StagePartial {
			t.Errorf("EvaluatePromotion() stage = %q, want %q", stage, StagePartial)
		}
	})

	t.Run("too few samples overall", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
		}
		_, err := EvaluatePromotion(&trigger, samples, 0.01, 500)
		if !errors.Is(err, ErrPromotionNotReady) {
			t.Errorf("EvaluatePromotion() error = %v, want wrapping ErrPromotionNotReady", err)
		}
	})

	t.Run("no samples at all", func(t *testing.T) {
		_, err := EvaluatePromotion(&trigger, nil, 0.01, 500)
		if !errors.Is(err, ErrPromotionNotReady) {
			t.Errorf("EvaluatePromotion() error = %v, want wrapping ErrPromotionNotReady", err)
		}
	})

	t.Run("an early unhealthy sample resets the consecutive count", func(t *testing.T) {
		samples := []StageHealth{
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.5, LatencyP99Ms: 100}, // unhealthy: resets count
			{ErrorRate: 0.001, LatencyP99Ms: 100},
			{ErrorRate: 0.001, LatencyP99Ms: 100},
		}
		// Only 2 consecutive healthy samples trail the bad one, below
		// RequiredHealthySamples: 3.
		_, err := EvaluatePromotion(&trigger, samples, 0.01, 500)
		if !errors.Is(err, ErrPromotionNotReady) {
			t.Errorf("EvaluatePromotion() error = %v, want wrapping ErrPromotionNotReady", err)
		}
	})
}
