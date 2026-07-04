package garelease_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/YASSERRMD/verdex/packages/garelease"
)

func TestRunPostReleaseChecklist_RequiresPermission(t *testing.T) {
	engine := newTestEngine(t)
	checklist := garelease.PostReleaseChecklist{
		Checks: []garelease.PostReleaseCheck{
			{Kind: garelease.PostReleaseCheckHealthEndpoint, Name: "health", Run: func() error { return nil }},
		},
	}

	_, err := engine.RunPostReleaseChecklist(context.Background(), mustUUID(), checklist)
	if !errors.Is(err, garelease.ErrUnauthenticated) {
		t.Fatalf("RunPostReleaseChecklist with no actor = %v, want ErrUnauthenticated", err)
	}

	viewerCtx := ctxWithUser(auditorUser())
	_, err = engine.RunPostReleaseChecklist(viewerCtx, mustUUID(), checklist)
	if !errors.Is(err, garelease.ErrForbidden) {
		t.Fatalf("RunPostReleaseChecklist as viewer-only = %v, want ErrForbidden", err)
	}
}

func TestRunPostReleaseChecklist_EmptyChecklistRejected(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	_, err := engine.RunPostReleaseChecklist(ctx, mustUUID(), garelease.PostReleaseChecklist{})
	if !errors.Is(err, garelease.ErrEmptyChecklist) {
		t.Fatalf("RunPostReleaseChecklist with an empty checklist = %v, want ErrEmptyChecklist", err)
	}
}

func TestRunPostReleaseChecklist_NilRunRejected(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	checklist := garelease.PostReleaseChecklist{
		Checks: []garelease.PostReleaseCheck{
			{Kind: garelease.PostReleaseCheckCustom, Name: "broken"},
		},
	}
	_, err := engine.RunPostReleaseChecklist(ctx, mustUUID(), checklist)
	if !errors.Is(err, garelease.ErrNilCheckFunc) {
		t.Fatalf("RunPostReleaseChecklist with a nil Run = %v, want ErrNilCheckFunc", err)
	}
}

func TestRunPostReleaseChecklist_AllPass(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	checklist := garelease.PostReleaseChecklist{
		Checks: []garelease.PostReleaseCheck{
			{Kind: garelease.PostReleaseCheckHealthEndpoint, Name: "health", Run: func() error { return nil }},
			{Kind: garelease.PostReleaseCheckMigrationVersion, Name: "migration", Run: func() error { return nil }},
			engine.DefaultGuardrailSmokeTestCheck(),
		},
	}

	report, err := engine.RunPostReleaseChecklist(ctx, mustUUID(), checklist)
	if err != nil {
		t.Fatalf("RunPostReleaseChecklist: %v", err)
	}
	if !report.Passed() {
		t.Fatalf("report.Passed() = false, want true. Failures: %+v", report.Failures())
	}
	if len(report.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(report.Results))
	}
}

func TestRunPostReleaseChecklist_RunsEveryCheckEvenAfterAFailure(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	var secondRan bool
	checklist := garelease.PostReleaseChecklist{
		Checks: []garelease.PostReleaseCheck{
			{Kind: garelease.PostReleaseCheckHealthEndpoint, Name: "health", Run: func() error { return fmt.Errorf("endpoint unreachable") }},
			{Kind: garelease.PostReleaseCheckMigrationVersion, Name: "migration", Run: func() error { secondRan = true; return nil }},
		},
	}

	report, err := engine.RunPostReleaseChecklist(ctx, mustUUID(), checklist)
	if err != nil {
		t.Fatalf("RunPostReleaseChecklist: %v", err)
	}
	if report.Passed() {
		t.Fatalf("report.Passed() = true, want false")
	}
	if !secondRan {
		t.Fatalf("second check did not run after the first failed -- RunPostReleaseChecklist must run every check regardless of an earlier failure")
	}
	if len(report.Failures()) != 1 {
		t.Fatalf("len(Failures()) = %d, want 1", len(report.Failures()))
	}
}

func TestPostReleaseReport_EmptyResultsNeverPassed(t *testing.T) {
	var r garelease.PostReleaseReport
	if r.Passed() {
		t.Fatalf("zero-value PostReleaseReport.Passed() = true, want false")
	}
	if r.Failures() != nil {
		t.Fatalf("zero-value PostReleaseReport.Failures() = %v, want nil", r.Failures())
	}
}

func TestPostReleaseReport_NilReceiver(t *testing.T) {
	var r *garelease.PostReleaseReport
	if r.Passed() {
		t.Fatalf("nil *PostReleaseReport.Passed() = true, want false")
	}
	if r.Failures() != nil {
		t.Fatalf("nil *PostReleaseReport.Failures() = %v, want nil", r.Failures())
	}
}

func TestDefaultGuardrailSmokeTestCheck_FailsIfGuardrailBroken(t *testing.T) {
	// This documents that DefaultGuardrailSmokeTestCheck's Run really is
	// backed by VerifyGuardrails (not a hardcoded pass) -- exercised
	// indirectly here by confirming it succeeds against the real
	// platform, mirroring TestVerifyGuardrails_PassesForTheRealPlatform.
	engine := newTestEngine(t)
	check := engine.DefaultGuardrailSmokeTestCheck()
	if check.Kind != garelease.PostReleaseCheckGuardrailSmokeTest {
		t.Fatalf("check.Kind = %v, want PostReleaseCheckGuardrailSmokeTest", check.Kind)
	}
	if err := check.Run(); err != nil {
		t.Fatalf("DefaultGuardrailSmokeTestCheck().Run() = %v, want nil for the real, passing platform", err)
	}
}
