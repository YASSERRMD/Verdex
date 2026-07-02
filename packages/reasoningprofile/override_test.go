package reasoningprofile_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

func TestOverrideRegistry_SetAndGet(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)

	if _, ok := reg.OverrideFor("case-1"); ok {
		t.Fatal("OverrideFor on empty registry should report no override")
	}

	if err := reg.SetOverride("case-1", reasoningprofile.FamilyIslamicLaw, "parties agreed to Sharia-influenced evidentiary rules"); err != nil {
		t.Fatalf("SetOverride returned error: %v", err)
	}

	family, ok := reg.OverrideFor("case-1")
	if !ok || family != reasoningprofile.FamilyIslamicLaw {
		t.Fatalf("OverrideFor(case-1) = (%v, %v), want (islamic_law, true)", family, ok)
	}
}

func TestOverrideRegistry_SetOverrideEmptyCaseID(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)
	if err := reg.SetOverride("", reasoningprofile.FamilyMixed, "reason"); !errors.Is(err, reasoningprofile.ErrEmptyCaseID) {
		t.Fatalf("SetOverride(empty case) = %v, want ErrEmptyCaseID", err)
	}
}

func TestOverrideRegistry_SetOverrideUnknownFamily(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)
	if err := reg.SetOverride("case-1", "unrecognized", "reason"); !errors.Is(err, reasoningprofile.ErrUnknownFamily) {
		t.Fatalf("SetOverride(unrecognized family) = %v, want ErrUnknownFamily", err)
	}
}

func TestOverrideRegistry_RecordsAndForwardsEvent(t *testing.T) {
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var forwarded []reasoningprofile.Event

	reg := reasoningprofile.NewOverrideRegistry(
		reasoningprofile.FuncAlertSink(func(e reasoningprofile.Event) { forwarded = append(forwarded, e) }),
		func() time.Time { return fixedTime },
	)

	if err := reg.SetOverride("case-42", reasoningprofile.FamilyMixed, "manual review"); err != nil {
		t.Fatalf("SetOverride returned error: %v", err)
	}

	events := reg.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	if events[0].CaseID != "case-42" || events[0].OverrideFamily != reasoningprofile.FamilyMixed || events[0].Reason != "manual review" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if events[0].AppliedAt != fixedTime {
		t.Fatalf("AppliedAt = %v, want %v", events[0].AppliedAt, fixedTime)
	}
	if len(forwarded) != 1 || forwarded[0].CaseID != "case-42" {
		t.Fatalf("event was not forwarded to sink: %+v", forwarded)
	}
}

func TestOverrideRegistry_PreviousFamilyTracksPriorOverride(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)

	if err := reg.SetOverride("case-1", reasoningprofile.FamilyCommonLaw, "first"); err != nil {
		t.Fatalf("first SetOverride error: %v", err)
	}
	if err := reg.SetOverride("case-1", reasoningprofile.FamilyCivilLaw, "corrected"); err != nil {
		t.Fatalf("second SetOverride error: %v", err)
	}

	events := reg.Events()
	if len(events) != 2 {
		t.Fatalf("Events() len = %d, want 2", len(events))
	}
	if events[1].PreviousFamily != reasoningprofile.FamilyCommonLaw {
		t.Errorf("second event PreviousFamily = %v, want common_law", events[1].PreviousFamily)
	}
	if events[1].OverrideFamily != reasoningprofile.FamilyCivilLaw {
		t.Errorf("second event OverrideFamily = %v, want civil_law", events[1].OverrideFamily)
	}
}

func TestOverrideRegistry_EventsReturnsDefensiveCopy(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)
	_ = reg.SetOverride("case-1", reasoningprofile.FamilyMixed, "reason")

	events := reg.Events()
	events[0].CaseID = "mutated"

	again := reg.Events()
	if again[0].CaseID == "mutated" {
		t.Fatal("Events() did not return a defensive copy")
	}
}

func TestOverrideRegistry_ConcurrentAccess(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reg.SetOverride("case-1", reasoningprofile.FamilyMixed, "concurrent")
			_, _ = reg.OverrideFor("case-1")
			_ = reg.Events()
		}()
	}
	wg.Wait()

	if len(reg.Events()) != 50 {
		t.Fatalf("Events() len = %d, want 50", len(reg.Events()))
	}
}

func TestResolveWithOverride_UsesOverrideWhenPresent(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)
	_ = reg.SetOverride("case-1", reasoningprofile.FamilyIslamicLaw, "reason")

	got := reasoningprofile.ResolveWithOverride(reg, "case-1", reasoningprofile.FamilyCommonLaw)
	if got != reasoningprofile.FamilyIslamicLaw {
		t.Errorf("ResolveWithOverride = %v, want islamic_law (the override)", got)
	}
}

func TestResolveWithOverride_FallsBackWhenNoOverride(t *testing.T) {
	reg := reasoningprofile.NewOverrideRegistry(nil, nil)

	got := reasoningprofile.ResolveWithOverride(reg, "case-unknown", reasoningprofile.FamilyCivilLaw)
	if got != reasoningprofile.FamilyCivilLaw {
		t.Errorf("ResolveWithOverride = %v, want civil_law (the fallback)", got)
	}
}

func TestResolveWithOverride_NilRegistryFallsBack(t *testing.T) {
	got := reasoningprofile.ResolveWithOverride(nil, "case-1", reasoningprofile.FamilyMixed)
	if got != reasoningprofile.FamilyMixed {
		t.Errorf("ResolveWithOverride(nil registry) = %v, want mixed (the fallback)", got)
	}
}

func TestNoOpAlertSinkDiscardsSilently(t *testing.T) {
	// Must not panic.
	reasoningprofile.NoOpAlertSink{}.Notify(reasoningprofile.Event{CaseID: "case-1"})
}

func TestMultiAlertSinkFansOut(t *testing.T) {
	var count1, count2 int
	sink := reasoningprofile.MultiAlertSink{Sinks: []reasoningprofile.AlertSink{
		reasoningprofile.FuncAlertSink(func(reasoningprofile.Event) { count1++ }),
		nil, // must tolerate a nil child sink
		reasoningprofile.FuncAlertSink(func(reasoningprofile.Event) { count2++ }),
	}}

	sink.Notify(reasoningprofile.Event{CaseID: "case-1"})

	if count1 != 1 || count2 != 1 {
		t.Fatalf("MultiAlertSink did not fan out to all children: count1=%d count2=%d", count1, count2)
	}
}
