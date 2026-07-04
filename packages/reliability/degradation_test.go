package reliability

import (
	"context"
	"errors"
	"testing"
)

func TestDegrader_PrimarySucceeds_NotDegraded(t *testing.T) {
	d := NewDegrader(ModeServeStale,
		func(_ context.Context) (string, error) { return "fresh", nil },
		func(_ context.Context, _ error) (string, error) { return "stale", nil },
	)

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.Degraded {
		t.Fatal("expected Degraded == false when primary succeeds")
	}
	if res.Value != "fresh" {
		t.Fatalf("expected primary's value %q, got %q", "fresh", res.Value)
	}
	if res.Mode != "" {
		t.Fatalf("expected empty Mode when not degraded, got %q", res.Mode)
	}
}

func TestDegrader_PrimaryFails_FallbackInvokedAndMarkedDegraded(t *testing.T) {
	primaryCalls, fallbackCalls := 0, 0
	d := NewDegrader(ModeServeStale,
		func(_ context.Context) (string, error) {
			primaryCalls++
			return "", errBoom
		},
		func(_ context.Context, primaryErr error) (string, error) {
			fallbackCalls++
			if !errors.Is(primaryErr, errBoom) {
				t.Errorf("expected fallback to receive primary's error, got %v", primaryErr)
			}
			return "stale-cached-value", nil
		},
	)

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("expected nil error (fallback succeeded), got %v", err)
	}
	if !res.Degraded {
		t.Fatal("expected Degraded == true when primary fails and fallback runs")
	}
	if res.Mode != ModeServeStale {
		t.Fatalf("expected Mode %q, got %q", ModeServeStale, res.Mode)
	}
	if res.Value != "stale-cached-value" {
		t.Fatalf("expected fallback's value, got %q", res.Value)
	}
	if primaryCalls != 1 || fallbackCalls != 1 {
		t.Fatalf("expected exactly 1 primary call and 1 fallback call, got %d/%d", primaryCalls, fallbackCalls)
	}
}

func TestDegrader_BothPrimaryAndFallbackFail_ReturnsFallbackError(t *testing.T) {
	fallbackErr := errors.New("fallback also failed")
	d := NewDegrader(ModeSkipEnrichment,
		func(_ context.Context) (int, error) { return 0, errBoom },
		func(_ context.Context, _ error) (int, error) { return 0, fallbackErr },
	)

	_, err := d.Run(context.Background())
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("expected the fallback's own error, got %v", err)
	}
	if errors.Is(err, errBoom) {
		t.Fatal("did not expect the primary's error in the chain when fallback also fails")
	}
}

func TestDegrader_NilPrimaryOrFallback_ReturnsError(t *testing.T) {
	fallback := func(_ context.Context, _ error) (int, error) { return 0, nil }
	primary := func(_ context.Context) (int, error) { return 0, nil }

	d1 := NewDegrader[int](ModeStaticDefault, nil, fallback)
	if _, err := d1.Run(context.Background()); !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc for nil primary, got %v", err)
	}

	d2 := NewDegrader[int](ModeStaticDefault, primary, nil)
	if _, err := d2.Run(context.Background()); !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc for nil fallback, got %v", err)
	}
}

func TestDegrader_DifferentDegradationModes(t *testing.T) {
	modes := []DegradationMode{ModeServeStale, ModeSkipEnrichment, ModeReducedScope, ModeStaticDefault}
	for _, mode := range modes {
		d := NewDegrader(mode,
			func(_ context.Context) (bool, error) { return false, errBoom },
			func(_ context.Context, _ error) (bool, error) { return true, nil },
		)
		res, err := d.Run(context.Background())
		if err != nil {
			t.Fatalf("mode %q: unexpected error %v", mode, err)
		}
		if res.Mode != mode {
			t.Errorf("mode %q: expected Result.Mode == %q, got %q", mode, mode, res.Mode)
		}
	}
}
