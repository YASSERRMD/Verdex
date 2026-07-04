package iac

import (
	"errors"
	"testing"
)

func TestRolloutStrategyIsValid(t *testing.T) {
	tests := []struct {
		strategy RolloutStrategy
		want     bool
	}{
		{RolloutStrategyDirect, true},
		{RolloutStrategyBlueGreen, true},
		{RolloutStrategyCanary, true},
		{RolloutStrategy("rainbow"), false},
	}
	for _, tc := range tests {
		if got := tc.strategy.IsValid(); got != tc.want {
			t.Errorf("RolloutStrategy(%q).IsValid() = %v, want %v", tc.strategy, got, tc.want)
		}
	}
}

func TestCanaryPlanValidate(t *testing.T) {
	valid := CanaryPlan{Stages: []CanaryStage{
		{Name: "5%", TrafficPercent: 5},
		{Name: "25%", TrafficPercent: 25},
		{Name: "100%", TrafficPercent: 100},
	}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid plan failed validation: %v", err)
	}

	empty := CanaryPlan{}
	if err := empty.Validate(); !errors.Is(err, ErrEmptyCanaryStages) {
		t.Errorf("empty plan: got %v, want ErrEmptyCanaryStages", err)
	}

	var nilPlan *CanaryPlan
	if err := nilPlan.Validate(); !errors.Is(err, ErrEmptyCanaryStages) {
		t.Errorf("nil plan: got %v, want ErrEmptyCanaryStages", err)
	}

	negative := CanaryPlan{Stages: []CanaryStage{{Name: "bad", TrafficPercent: -5}}}
	if err := negative.Validate(); !errors.Is(err, ErrInvalidTrafficPercentage) {
		t.Errorf("negative percent: got %v, want ErrInvalidTrafficPercentage", err)
	}

	over100 := CanaryPlan{Stages: []CanaryStage{{Name: "bad", TrafficPercent: 150}}}
	if err := over100.Validate(); !errors.Is(err, ErrInvalidTrafficPercentage) {
		t.Errorf("over 100: got %v, want ErrInvalidTrafficPercentage", err)
	}

	decreasing := CanaryPlan{Stages: []CanaryStage{
		{Name: "50%", TrafficPercent: 50},
		{Name: "25%", TrafficPercent: 25},
	}}
	if err := decreasing.Validate(); !errors.Is(err, ErrInvalidTrafficPercentage) {
		t.Errorf("decreasing percentages: got %v, want ErrInvalidTrafficPercentage", err)
	}

	unnamed := CanaryPlan{Stages: []CanaryStage{{Name: "", TrafficPercent: 5}}}
	if err := unnamed.Validate(); !errors.Is(err, ErrInvalidTrafficPercentage) {
		t.Errorf("unnamed stage: got %v, want ErrInvalidTrafficPercentage", err)
	}
}

// TestTrafficPercentageAt is the real-arithmetic test task 7 calls
// for: given a canary plan's stages, what percentage of traffic
// should go to the new version at step N.
func TestTrafficPercentageAt(t *testing.T) {
	plan := DefaultCanaryPlan()
	if err := plan.Validate(); err != nil {
		t.Fatalf("DefaultCanaryPlan produced an invalid plan: %v", err)
	}

	tests := []struct {
		step int
		want float64
	}{
		{0, 5},
		{1, 25},
		{2, 50},
		{3, 100},
	}
	for _, tc := range tests {
		got, err := plan.TrafficPercentageAt(tc.step)
		if err != nil {
			t.Fatalf("TrafficPercentageAt(%d): unexpected error: %v", tc.step, err)
		}
		if got != tc.want {
			t.Errorf("TrafficPercentageAt(%d) = %v, want %v", tc.step, got, tc.want)
		}
	}

	if _, err := plan.TrafficPercentageAt(-1); !errors.Is(err, ErrCanaryStageOutOfRange) {
		t.Errorf("negative step: got %v, want ErrCanaryStageOutOfRange", err)
	}
	if _, err := plan.TrafficPercentageAt(4); !errors.Is(err, ErrCanaryStageOutOfRange) {
		t.Errorf("step past end: got %v, want ErrCanaryStageOutOfRange", err)
	}

	var nilPlan *CanaryPlan
	if _, err := nilPlan.TrafficPercentageAt(0); !errors.Is(err, ErrEmptyCanaryStages) {
		t.Errorf("nil plan: got %v, want ErrEmptyCanaryStages", err)
	}
}

func TestRemainingPercentageAt(t *testing.T) {
	plan := DefaultCanaryPlan()

	tests := []struct {
		step int
		want float64
	}{
		{0, 95},
		{1, 75},
		{2, 50},
		{3, 0},
	}
	for _, tc := range tests {
		got, err := plan.RemainingPercentageAt(tc.step)
		if err != nil {
			t.Fatalf("RemainingPercentageAt(%d): unexpected error: %v", tc.step, err)
		}
		if got != tc.want {
			t.Errorf("RemainingPercentageAt(%d) = %v, want %v", tc.step, got, tc.want)
		}
	}

	if _, err := plan.RemainingPercentageAt(99); !errors.Is(err, ErrCanaryStageOutOfRange) {
		t.Errorf("out of range: got %v, want ErrCanaryStageOutOfRange", err)
	}
}

func TestDefaultCanaryPlan_MonotonicallyIncreasesToFull(t *testing.T) {
	plan := DefaultCanaryPlan()
	if len(plan.Stages) == 0 {
		t.Fatal("expected at least one stage")
	}
	last := plan.Stages[len(plan.Stages)-1]
	if last.TrafficPercent != 100 {
		t.Errorf("expected final stage to reach 100%%, got %v", last.TrafficPercent)
	}
}
