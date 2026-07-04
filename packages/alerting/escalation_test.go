package alerting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

func testPolicy() alerting.EscalationPolicy {
	return alerting.EscalationPolicy{
		TenantID:    uuid.New(),
		Name:        "test-policy",
		MinSeverity: alerting.SeverityWarning,
		Tiers: []alerting.EscalationTier{
			{Responder: alerting.Responder{Name: "tier-1-primary"}, DelayBeforeNext: 15 * time.Minute},
			{Responder: alerting.Responder{Name: "tier-2-secondary"}, DelayBeforeNext: 30 * time.Minute},
			{Responder: alerting.Responder{Name: "tier-3-platform-team"}},
		},
	}
}

// TestRoute_EscalatesToTier2AfterConfiguredDelay is the test the phase
// brief calls out explicitly: an alert open since openSince should
// route to tier 1 immediately, and escalate to tier 2 once tier 1's
// DelayBeforeNext (15 minutes) has elapsed.
func TestRoute_EscalatesToTier2AfterConfiguredDelay(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	openSince := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	alert := alerting.AlertEvent{CreatedAt: openSince, Severity: alerting.SeverityCritical}

	// Immediately: tier 1.
	responder, err := alerting.Route(alert, policy, openSince)
	if err != nil {
		t.Fatalf("Route (t=0): %v", err)
	}
	if responder.Name != "tier-1-primary" {
		t.Errorf("Route (t=0) = %q, want tier-1-primary", responder.Name)
	}

	// Just before tier 1's delay elapses: still tier 1.
	almostThere := openSince.Add(14*time.Minute + 59*time.Second)
	responder, err = alerting.Route(alert, policy, almostThere)
	if err != nil {
		t.Fatalf("Route (t=14m59s): %v", err)
	}
	if responder.Name != "tier-1-primary" {
		t.Errorf("Route (t=14m59s) = %q, want tier-1-primary (delay not yet elapsed)", responder.Name)
	}

	// Exactly at 15 minutes: escalated to tier 2.
	exactly15 := openSince.Add(15 * time.Minute)
	responder, err = alerting.Route(alert, policy, exactly15)
	if err != nil {
		t.Fatalf("Route (t=15m): %v", err)
	}
	if responder.Name != "tier-2-secondary" {
		t.Errorf("Route (t=15m) = %q, want tier-2-secondary", responder.Name)
	}

	// Well past 15 minutes but before the cumulative 45 minutes: still
	// tier 2.
	after20 := openSince.Add(20 * time.Minute)
	responder, err = alerting.Route(alert, policy, after20)
	if err != nil {
		t.Fatalf("Route (t=20m): %v", err)
	}
	if responder.Name != "tier-2-secondary" {
		t.Errorf("Route (t=20m) = %q, want tier-2-secondary", responder.Name)
	}
}

// TestRoute_EscalatesToFinalTierAndStaysThere proves Route lands on
// the final tier once every earlier tier's cumulative delay has
// elapsed, and never advances past it (there is nothing further to
// escalate to).
func TestRoute_EscalatesToFinalTierAndStaysThere(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	openSince := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	alert := alerting.AlertEvent{CreatedAt: openSince, Severity: alerting.SeverityCritical}

	// Cumulative delay to reach tier 3 is 15m + 30m = 45m.
	exactly45 := openSince.Add(45 * time.Minute)
	responder, err := alerting.Route(alert, policy, exactly45)
	if err != nil {
		t.Fatalf("Route (t=45m): %v", err)
	}
	if responder.Name != "tier-3-platform-team" {
		t.Errorf("Route (t=45m) = %q, want tier-3-platform-team", responder.Name)
	}

	// Much later: still the final tier, does not wrap or error.
	muchLater := openSince.Add(48 * time.Hour)
	responder, err = alerting.Route(alert, policy, muchLater)
	if err != nil {
		t.Fatalf("Route (t=+48h): %v", err)
	}
	if responder.Name != "tier-3-platform-team" {
		t.Errorf("Route (t=+48h) = %q, want tier-3-platform-team", responder.Name)
	}
}

func TestRoute_NoTiers(t *testing.T) {
	t.Parallel()
	policy := alerting.EscalationPolicy{
		TenantID:    uuid.New(),
		Name:        "empty",
		MinSeverity: alerting.SeverityWarning,
	}
	_, err := alerting.Route(alerting.AlertEvent{Severity: alerting.SeverityCritical}, policy, time.Now())
	if !errors.Is(err, alerting.ErrInvalidPolicy) {
		t.Fatalf("Route with no tiers error = %v, want ErrInvalidPolicy (Validate rejects an empty Tiers slice)", err)
	}
}

func TestEscalationPolicy_Validate(t *testing.T) {
	t.Parallel()

	valid := testPolicy()
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate() on well-formed policy = %v, want nil", err)
	}

	noName := testPolicy()
	noName.Name = ""
	if err := noName.Validate(); !errors.Is(err, alerting.ErrInvalidPolicy) {
		t.Errorf("Validate() with blank Name = %v, want ErrInvalidPolicy", err)
	}

	noTiers := testPolicy()
	noTiers.Tiers = nil
	if err := noTiers.Validate(); !errors.Is(err, alerting.ErrInvalidPolicy) {
		t.Errorf("Validate() with no tiers = %v, want ErrInvalidPolicy", err)
	}

	badSeverity := testPolicy()
	badSeverity.MinSeverity = "bogus"
	if err := badSeverity.Validate(); !errors.Is(err, alerting.ErrInvalidSeverity) {
		t.Errorf("Validate() with invalid MinSeverity = %v, want ErrInvalidSeverity", err)
	}

	negativeDelay := testPolicy()
	negativeDelay.Tiers[0].DelayBeforeNext = -1
	if err := negativeDelay.Validate(); !errors.Is(err, alerting.ErrInvalidPolicy) {
		t.Errorf("Validate() with negative delay = %v, want ErrInvalidPolicy", err)
	}
}

func TestEscalationPolicy_AppliesTo(t *testing.T) {
	t.Parallel()
	policy := testPolicy() // MinSeverity: SeverityWarning
	if !policy.AppliesTo(alerting.AlertEvent{Severity: alerting.SeverityCritical}) {
		t.Error("AppliesTo(Critical) = false, want true (Critical >= Warning)")
	}
	if !policy.AppliesTo(alerting.AlertEvent{Severity: alerting.SeverityWarning}) {
		t.Error("AppliesTo(Warning) = false, want true (equal to MinSeverity)")
	}
	if policy.AppliesTo(alerting.AlertEvent{Severity: alerting.SeverityInfo}) {
		t.Error("AppliesTo(Info) = true, want false (Info < Warning)")
	}
}

func TestCurrentTierIndex(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	openSince := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{10 * time.Minute, 0},
		{15 * time.Minute, 1},
		{40 * time.Minute, 1},
		{45 * time.Minute, 2},
		{100 * time.Hour, 2},
	}
	for _, c := range cases {
		got, err := alerting.CurrentTierIndex(openSince, policy, openSince.Add(c.elapsed))
		if err != nil {
			t.Fatalf("CurrentTierIndex(elapsed=%s): %v", c.elapsed, err)
		}
		if got != c.want {
			t.Errorf("CurrentTierIndex(elapsed=%s) = %d, want %d", c.elapsed, got, c.want)
		}
	}
}

// fakeRecipientSink records every Deliver call for assertion.
type fakeRecipientSink struct {
	delivered []alerting.Responder
}

func (f *fakeRecipientSink) Deliver(_ context.Context, _ alerting.AlertEvent, responder alerting.Responder) error {
	f.delivered = append(f.delivered, responder)
	return nil
}

func TestRouteAndDeliver(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	openSince := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	alert := alerting.AlertEvent{CreatedAt: openSince, Severity: alerting.SeverityCritical}

	sink := &fakeRecipientSink{}
	responder, err := alerting.RouteAndDeliver(t.Context(), alert, policy, openSince, sink)
	if err != nil {
		t.Fatalf("RouteAndDeliver: %v", err)
	}
	if responder.Name != "tier-1-primary" {
		t.Errorf("responder.Name = %q, want tier-1-primary", responder.Name)
	}
	if len(sink.delivered) != 1 || sink.delivered[0].Name != "tier-1-primary" {
		t.Errorf("sink.delivered = %v, want exactly one delivery to tier-1-primary", sink.delivered)
	}
}

func TestRouteAndDeliver_NilSinkDefaultsToNoOp(t *testing.T) {
	t.Parallel()
	policy := testPolicy()
	now := time.Now()
	alert := alerting.AlertEvent{CreatedAt: now, Severity: alerting.SeverityCritical}

	// Must not panic with a nil sink.
	if _, err := alerting.RouteAndDeliver(t.Context(), alert, policy, now, nil); err != nil {
		t.Fatalf("RouteAndDeliver with nil sink: %v", err)
	}
}

func TestDefaultEscalationPolicy_Valid(t *testing.T) {
	t.Parallel()
	policy := alerting.DefaultEscalationPolicy(uuid.New())
	if err := policy.Validate(); err != nil {
		t.Fatalf("DefaultEscalationPolicy().Validate() = %v, want nil", err)
	}
	if len(policy.Tiers) != 3 {
		t.Fatalf("len(policy.Tiers) = %d, want 3", len(policy.Tiers))
	}
}
