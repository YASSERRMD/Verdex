package reliability

import (
	"context"
	"errors"
	"testing"
)

func TestTrafficShifter_AllHealthy_RoundRobins(t *testing.T) {
	ts := NewTrafficShifter("a", "b", "c")
	ts.SetStatus("a", HealthHealthy)
	ts.SetStatus("b", HealthHealthy)
	ts.SetStatus("c", HealthHealthy)

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		name, err := ts.Select()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error %v", i, err)
		}
		seen[name]++
	}

	for _, name := range []string{"a", "b", "c"} {
		if seen[name] != 3 {
			t.Errorf("expected backend %q selected exactly 3 times over 9 calls, got %d", name, seen[name])
		}
	}
}

func TestTrafficShifter_DegradesToRemainingHealthySet(t *testing.T) {
	ts := NewTrafficShifter("a", "b", "c")
	ts.SetStatus("a", HealthHealthy)
	ts.SetStatus("b", HealthUnhealthy)
	ts.SetStatus("c", HealthHealthy)

	for i := 0; i < 6; i++ {
		name, err := ts.Select()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error %v", i, err)
		}
		if name == "b" {
			t.Fatalf("iteration %d: unhealthy backend %q must never be selected", i, name)
		}
	}

	if got := ts.HealthyCount(); got != 2 {
		t.Fatalf("expected HealthyCount()=2, got %d", got)
	}
}

func TestTrafficShifter_FailsClosedWhenNoneHealthy(t *testing.T) {
	ts := NewTrafficShifter("a", "b")
	ts.SetStatus("a", HealthUnhealthy)
	ts.SetStatus("b", HealthUnhealthy)

	_, err := ts.Select()
	if !errors.Is(err, ErrNoHealthyBackends) {
		t.Fatalf("expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestTrafficShifter_UnknownStatusTreatedAsUnhealthy(t *testing.T) {
	ts := NewTrafficShifter("a", "b")
	// Neither backend has had SetStatus called yet -- both HealthUnknown.

	_, err := ts.Select()
	if !errors.Is(err, ErrNoHealthyBackends) {
		t.Fatalf("expected ErrNoHealthyBackends for all-unknown backends (fail closed), got %v", err)
	}
}

func TestTrafficShifter_RecoveryAddsBackendBackToRotation(t *testing.T) {
	ts := NewTrafficShifter("a", "b")
	ts.SetStatus("a", HealthHealthy)
	ts.SetStatus("b", HealthUnhealthy)

	name, err := ts.Select()
	if err != nil || name != "a" {
		t.Fatalf("expected only healthy backend 'a' selected, got %q, err=%v", name, err)
	}

	// "b" recovers.
	ts.SetStatus("b", HealthHealthy)

	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		n, selErr := ts.Select()
		if selErr != nil {
			t.Fatalf("unexpected error %v", selErr)
		}
		seen[n] = true
	}
	if !seen["b"] {
		t.Fatal("expected 'b' back in rotation after recovering to HealthHealthy")
	}
}

func TestTrafficShifter_SelectAll_ReturnsEveryHealthyBackendSorted(t *testing.T) {
	ts := NewTrafficShifter("z", "a", "m")
	ts.SetStatus("z", HealthHealthy)
	ts.SetStatus("a", HealthHealthy)
	ts.SetStatus("m", HealthUnhealthy)

	all, err := ts.SelectAll()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	want := []string{"a", "z"}
	if len(all) != len(want) {
		t.Fatalf("expected %v, got %v", want, all)
	}
	for i := range want {
		if all[i] != want[i] {
			t.Fatalf("expected sorted %v, got %v", want, all)
		}
	}
}

func TestTrafficShifter_SelectAll_ErrorWhenNoneHealthy(t *testing.T) {
	ts := NewTrafficShifter("a")
	ts.SetStatus("a", HealthUnhealthy)

	_, err := ts.SelectAll()
	if !errors.Is(err, ErrNoHealthyBackends) {
		t.Fatalf("expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestTrafficShifter_SetStatus_RegistersUnknownBackend(t *testing.T) {
	ts := NewTrafficShifter() // empty
	ts.SetStatus("new-backend", HealthHealthy)

	name, err := ts.Select()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if name != "new-backend" {
		t.Fatalf("expected 'new-backend', got %q", name)
	}
}

func TestTrafficShifter_Backends_ReturnsRegistrationOrderWithStatus(t *testing.T) {
	ts := NewTrafficShifter("first", "second")
	ts.SetStatus("first", HealthHealthy)
	ts.SetStatus("second", HealthUnhealthy)

	backends := ts.Backends()
	if len(backends) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(backends))
	}
	if backends[0].Name != "first" || backends[0].Status != HealthHealthy {
		t.Errorf("expected first=healthy, got %+v", backends[0])
	}
	if backends[1].Name != "second" || backends[1].Status != HealthUnhealthy {
		t.Errorf("expected second=unhealthy, got %+v", backends[1])
	}
}

func TestTrafficShifter_RefreshFromCheckers(t *testing.T) {
	ts := NewTrafficShifter("db", "graph")
	checkers := map[string]HealthCheckFunc{
		"db":    func(_ context.Context) error { return nil },
		"graph": func(_ context.Context) error { return errBoom },
	}

	ts.RefreshFromCheckers(context.Background(), checkers)

	name, err := ts.Select()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if name != "db" {
		t.Fatalf("expected only 'db' healthy after refresh, got %q", name)
	}
}

func TestTrafficShifter_RefreshFromCheckers_NilCheckerMarksUnknown(t *testing.T) {
	ts := NewTrafficShifter("db")
	ts.SetStatus("db", HealthHealthy)

	ts.RefreshFromCheckers(context.Background(), map[string]HealthCheckFunc{"db": nil})

	_, err := ts.Select()
	if !errors.Is(err, ErrNoHealthyBackends) {
		t.Fatalf("expected a nil checker to demote status to Unknown (fail closed), got %v", err)
	}
}

func TestHealthStatus_String(t *testing.T) {
	cases := map[HealthStatus]string{
		HealthHealthy:   "healthy",
		HealthUnhealthy: "unhealthy",
		HealthUnknown:   "unknown",
		HealthStatus(9): "unknown",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("HealthStatus(%d).String() = %q, want %q", status, got, want)
		}
	}
}

// TestTrafficShifter_ConcurrentSelectIsRaceFree exercises Select and
// SetStatus concurrently to confirm the round-robin cursor and status map
// are properly synchronized under -race.
func TestTrafficShifter_ConcurrentSelectIsRaceFree(t *testing.T) {
	ts := NewTrafficShifter("a", "b", "c")
	ts.SetStatus("a", HealthHealthy)
	ts.SetStatus("b", HealthHealthy)
	ts.SetStatus("c", HealthHealthy)

	const goroutines = 30
	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			if idx%3 == 0 {
				ts.SetStatus("a", HealthHealthy)
				return
			}
			_, _ = ts.Select()
		}(i)
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}
