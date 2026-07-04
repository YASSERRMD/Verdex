package reliability

import (
	"errors"
	"testing"
	"time"
)

func successRateSLO() SLO {
	return SLO{
		Name:   "ingestion-availability",
		Kind:   SLOKindSuccessRate,
		Target: 0.99,
		Window: 24 * time.Hour,
	}
}

func TestSLO_Validate(t *testing.T) {
	cases := []struct {
		name    string
		slo     SLO
		wantErr bool
	}{
		{"valid success rate", successRateSLO(), false},
		{"valid latency", SLO{Name: "x", Kind: SLOKindLatency, Target: float64(time.Second), Window: time.Hour}, false},
		{"blank name", SLO{Kind: SLOKindSuccessRate, Target: 0.9, Window: time.Hour}, true},
		{"zero window", SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 0.9, Window: 0}, true},
		{"negative window", SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 0.9, Window: -time.Hour}, true},
		{"success rate target > 1", SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 1.5, Window: time.Hour}, true},
		{"success rate target < 0", SLO{Name: "x", Kind: SLOKindSuccessRate, Target: -0.1, Window: time.Hour}, true},
		{"latency target <= 0", SLO{Name: "x", Kind: SLOKindLatency, Target: 0, Window: time.Hour}, true},
		{"unknown kind", SLO{Name: "x", Kind: "bogus", Target: 0.9, Window: time.Hour}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.slo.Validate()
			if c.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestEvaluateSLO_InvalidSLOReturnsError(t *testing.T) {
	_, err := EvaluateSLO(SLO{}, nil, time.Now())
	if !errors.Is(err, ErrInvalidSLO) {
		t.Fatalf("expected ErrInvalidSLO, got %v", err)
	}
}

func TestEvaluateSLO_EmptyObservationsMeansMet(t *testing.T) {
	status, err := EvaluateSLO(successRateSLO(), nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !status.Met {
		t.Fatal("expected Met=true with no observations yet")
	}
	if status.SampleCount != 0 {
		t.Fatalf("expected SampleCount=0, got %d", status.SampleCount)
	}
}

func TestEvaluateSLO_SuccessRate_WithinBudget(t *testing.T) {
	now := time.Now()
	slo := successRateSLO() // 99% target

	// 199 successes, 1 failure => 99.5% success, above the 99% target.
	obs := make([]Observation, 0, 200)
	for i := 0; i < 199; i++ {
		obs = append(obs, Observation{Success: true, At: now})
	}
	obs = append(obs, Observation{Success: false, At: now})

	status, err := EvaluateSLO(slo, obs, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !status.Met {
		t.Fatalf("expected Met=true (99.5%% observed >= 99%% target), got Observed=%v", status.Observed)
	}
	if status.SampleCount != 200 {
		t.Fatalf("expected SampleCount=200, got %d", status.SampleCount)
	}
	if status.Observed < 0.994 || status.Observed > 0.996 {
		t.Fatalf("expected Observed ~= 0.995, got %v", status.Observed)
	}
}

func TestEvaluateSLO_SuccessRate_Violated(t *testing.T) {
	now := time.Now()
	slo := successRateSLO() // 99% target

	// 90 successes, 10 failures => 90% success, below the 99% target.
	obs := make([]Observation, 0, 100)
	for i := 0; i < 90; i++ {
		obs = append(obs, Observation{Success: true, At: now})
	}
	for i := 0; i < 10; i++ {
		obs = append(obs, Observation{Success: false, At: now})
	}

	status, err := EvaluateSLO(slo, obs, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if status.Met {
		t.Fatalf("expected Met=false (90%% observed < 99%% target), got Observed=%v", status.Observed)
	}
}

func TestEvaluateSLO_ExcludesObservationsOutsideWindow(t *testing.T) {
	now := time.Now()
	slo := SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 0.99, Window: time.Hour}

	obs := []Observation{
		{Success: true, At: now.Add(-30 * time.Minute)}, // in window
		{Success: false, At: now.Add(-2 * time.Hour)},   // outside window (too old)
		{Success: true, At: now.Add(5 * time.Minute)},   // outside window (future beyond now)
	}

	status, err := EvaluateSLO(slo, obs, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if status.SampleCount != 1 {
		t.Fatalf("expected exactly 1 observation within the window, got %d", status.SampleCount)
	}
	if !status.Met {
		t.Fatal("expected Met=true (the only in-window observation succeeded)")
	}
}

func TestEvaluateSLO_Latency_WithinBudget(t *testing.T) {
	now := time.Now()
	slo := SLO{Name: "retrieval-latency", Kind: SLOKindLatency, Target: float64(500 * time.Millisecond), Window: time.Hour}

	obs := make([]Observation, 0, 100)
	for i := 0; i < 100; i++ {
		obs = append(obs, Observation{Success: true, Latency: 100 * time.Millisecond, At: now})
	}

	status, err := EvaluateSLO(slo, obs, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !status.Met {
		t.Fatalf("expected Met=true, P95 100ms is well under 500ms target, got Observed=%v", status.Observed)
	}
}

func TestEvaluateSLO_Latency_Violated(t *testing.T) {
	now := time.Now()
	slo := SLO{Name: "retrieval-latency", Kind: SLOKindLatency, Target: float64(200 * time.Millisecond), Window: time.Hour}

	obs := make([]Observation, 0, 100)
	for i := 0; i < 100; i++ {
		obs = append(obs, Observation{Success: true, Latency: 900 * time.Millisecond, At: now})
	}

	status, err := EvaluateSLO(slo, obs, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if status.Met {
		t.Fatalf("expected Met=false, P95 900ms exceeds 200ms target, got Observed=%v", status.Observed)
	}
}

func TestEvaluateSLO_Latency_ExcludesFailedCalls(t *testing.T) {
	now := time.Now()
	slo := SLO{Name: "x", Kind: SLOKindLatency, Target: float64(50 * time.Millisecond), Window: time.Hour}

	obs := []Observation{
		{Success: true, Latency: 10 * time.Millisecond, At: now},
		{Success: false, Latency: 5 * time.Second, At: now}, // slow failure, must be excluded
	}

	status, err := EvaluateSLO(slo, obs, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !status.Met {
		t.Fatalf("expected the slow failed call excluded from P95, got Observed=%v", status.Observed)
	}
}

func TestP95Latency_NearestRank(t *testing.T) {
	// 20 observations: 1..20 ms. P95 nearest-rank of 20 samples is rank
	// ceil(0.95*20)=19, i.e. the 19th smallest value = 19ms.
	obs := make([]Observation, 0, 20)
	for i := 1; i <= 20; i++ {
		obs = append(obs, Observation{Success: true, Latency: time.Duration(i) * time.Millisecond})
	}
	got := p95Latency(obs)
	want := 19 * time.Millisecond
	if got != want {
		t.Fatalf("expected P95 = %v, got %v", want, got)
	}
}

func TestP95Latency_EmptyReturnsZero(t *testing.T) {
	if got := p95Latency(nil); got != 0 {
		t.Fatalf("expected 0 for empty observations, got %v", got)
	}
}
