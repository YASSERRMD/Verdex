package securitytesting_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func TestNewEngine_RequiresDependencies(t *testing.T) {
	t.Parallel()

	harness := securitytesting.NewHarness()
	runs := securitytesting.NewInMemoryRunRecordRepository()
	finds := securitytesting.NewInMemoryFindingRepository()
	scope := securitytesting.DefaultScopeDocument()

	if _, err := securitytesting.NewEngine(nil, scope, runs, finds, nil); !errors.Is(err, securitytesting.ErrNilStore) {
		t.Errorf("NewEngine(nil harness, ...) error = %v, want ErrNilStore", err)
	}
	if _, err := securitytesting.NewEngine(harness, scope, nil, finds, nil); !errors.Is(err, securitytesting.ErrNilStore) {
		t.Errorf("NewEngine(nil runs, ...) error = %v, want ErrNilStore", err)
	}
	if _, err := securitytesting.NewEngine(harness, scope, runs, nil, nil); !errors.Is(err, securitytesting.ErrNilStore) {
		t.Errorf("NewEngine(nil finds, ...) error = %v, want ErrNilStore", err)
	}
	if _, err := securitytesting.NewEngine(harness, securitytesting.ScopeDocument{}, runs, finds, nil); err == nil {
		t.Error("NewEngine(invalid scope, ...) error = nil, want a validation error")
	}
	if _, err := securitytesting.NewEngine(harness, scope, runs, finds, nil); err != nil {
		t.Errorf("NewEngine(valid deps, nil audit) error = %v, want nil (nil audit sink is permitted)", err)
	}
}

func TestEngine_RunSuite_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	t.Run("unauthenticated rejected", func(t *testing.T) {
		t.Parallel()
		_, err := engine.RunSuite(context.Background(), tenantID, securitytesting.CategoryRegression)
		if !errors.Is(err, securitytesting.ErrUnauthenticated) {
			t.Errorf("RunSuite() with no ctx actor error = %v, want ErrUnauthenticated", err)
		}
	})

	t.Run("view-only actor rejected", func(t *testing.T) {
		t.Parallel()
		ctx := ctxWithUser(auditorUser(tenantID))
		_, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryRegression)
		if !errors.Is(err, securitytesting.ErrForbidden) {
			t.Errorf("RunSuite() as auditor (view-only) error = %v, want ErrForbidden", err)
		}
	})

	t.Run("unprivileged actor rejected", func(t *testing.T) {
		t.Parallel()
		ctx := ctxWithUser(advocateUser(tenantID))
		_, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryRegression)
		if !errors.Is(err, securitytesting.ErrForbidden) {
			t.Errorf("RunSuite() as advocate error = %v, want ErrForbidden", err)
		}
	})

	t.Run("admin permitted", func(t *testing.T) {
		t.Parallel()
		ctx := ctxWithUser(adminUser(tenantID))
		records, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryRegression)
		if err != nil {
			t.Fatalf("RunSuite() as admin error = %v, want nil", err)
		}
		if len(records) == 0 {
			t.Error("RunSuite() as admin returned zero records for CategoryRegression, want at least the seeded regression scenarios")
		}
	})
}

func TestEngine_RunSuite_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()

	// admin is authenticated against tenantA but RunSuite is called
	// with tenantB as the target scope.
	admin := adminUser(tenantA)
	_, err := engine.RunSuite(ctxWithUser(admin), tenantB, securitytesting.CategoryRegression)
	if !errors.Is(err, securitytesting.ErrCrossTenantAccess) {
		t.Errorf("RunSuite() cross-tenant error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_RunSuite_PersistsRunRecords(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	ctx := ctxWithUser(admin)

	records, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryAbuseCase)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("RunSuite returned zero records")
	}

	persisted, err := engine.ListRuns(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(persisted) != len(records) {
		t.Errorf("ListRuns returned %d records, want %d (every RunSuite record persisted)", len(persisted), len(records))
	}
}

func TestEngine_OpenFindingsFromRun_OnlyOpensFailedRecords(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(
		alwaysPass("pass-scenario", securitytesting.CategoryRegression),
		alwaysFail("fail-scenario"),
		alwaysErrors("error-scenario"),
	)
	engine, _, tenantID := newTestEngineWithHarness(t, h)
	ctx := ctxWithUser(adminUser(tenantID))

	records, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryRegression)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("RunSuite returned %d records, want 3", len(records))
	}

	findings, err := engine.OpenFindingsFromRun(ctx, tenantID, records, securitytesting.SeverityHigh)
	if err != nil {
		t.Fatalf("OpenFindingsFromRun: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("OpenFindingsFromRun opened %d findings, want exactly 1 (only the failed record)", len(findings))
	}
	if findings[0].SourceScenario != "fail-scenario" {
		t.Errorf("opened finding SourceScenario = %q, want %q", findings[0].SourceScenario, "fail-scenario")
	}
	if findings[0].Status != securitytesting.FindingOpen {
		t.Errorf("opened finding Status = %v, want FindingOpen", findings[0].Status)
	}
}

func TestEngine_TransitionFinding(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	f := openTestFinding(t, engine, tenantID, securitytesting.CategoryAuthzBypass)
	ctx := ctxWithUser(adminUser(tenantID))

	t.Run("open to triaged succeeds", func(t *testing.T) {
		updated, err := engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingTriaged, "")
		if err != nil {
			t.Fatalf("TransitionFinding(open->triaged): %v", err)
		}
		if updated.Status != securitytesting.FindingTriaged {
			t.Errorf("Status = %v, want FindingTriaged", updated.Status)
		}
	})

	t.Run("triaged directly to verified fixed is illegal even via TransitionFinding", func(t *testing.T) {
		_, err := engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingVerifiedFixed, "")
		if !errors.Is(err, securitytesting.ErrIllegalStatusTransition) {
			t.Errorf("TransitionFinding(triaged->verified_fixed) error = %v, want ErrIllegalStatusTransition -- that state must only be reachable via VerifyRemediation", err)
		}
	})

	t.Run("risk accepted without justification rejected", func(t *testing.T) {
		_, err := engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingRiskAccepted, "")
		if err == nil {
			t.Error("TransitionFinding(->risk_accepted, blank justification) error = nil, want an error")
		}
	})

	t.Run("risk accepted with justification succeeds", func(t *testing.T) {
		updated, err := engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingRiskAccepted, "compensating control exists")
		if err != nil {
			t.Fatalf("TransitionFinding(->risk_accepted): %v", err)
		}
		if updated.Status != securitytesting.FindingRiskAccepted {
			t.Errorf("Status = %v, want FindingRiskAccepted", updated.Status)
		}
		if updated.RiskAcceptedJustification != "compensating control exists" {
			t.Errorf("RiskAcceptedJustification = %q, want %q", updated.RiskAcceptedJustification, "compensating control exists")
		}
	})

	t.Run("terminal state rejects further transitions", func(t *testing.T) {
		_, err := engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingTriaged, "")
		if !errors.Is(err, securitytesting.ErrIllegalStatusTransition) {
			t.Errorf("TransitionFinding from a terminal FindingRiskAccepted error = %v, want ErrIllegalStatusTransition", err)
		}
	})
}

func TestEngine_TransitionFinding_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	f := openTestFinding(t, engine, tenantID, securitytesting.CategoryAuthzBypass)

	viewerCtx := ctxWithUser(auditorUser(tenantID))
	_, err := engine.TransitionFinding(viewerCtx, tenantID, f.ID, securitytesting.FindingTriaged, "")
	if !errors.Is(err, securitytesting.ErrForbidden) {
		t.Errorf("TransitionFinding as viewer error = %v, want ErrForbidden", err)
	}
}

// TestEngine_VerifyRemediation_OnlyFlipsWhenReRunPasses is the
// definitive proof of task 8's remediation-verification requirement:
// a Finding sourced from a Scenario that keeps failing on re-run must
// NEVER transition to FindingVerifiedFixed, and one sourced from a
// Scenario that now passes must transition to exactly that state, with
// the same Engine and the same code path -- the only difference is
// whether the underlying Scenario itself was fixed.
func TestEngine_VerifyRemediation_OnlyFlipsWhenReRunPasses(t *testing.T) {
	t.Parallel()

	stillBroken := true
	toggleable := securitytesting.NewScenarioFunc("toggleable", securitytesting.CategoryRegression, func(context.Context) (securitytesting.Result, error) {
		if stillBroken {
			return securitytesting.Result{Outcome: securitytesting.OutcomeFailed, Detail: "still vulnerable"}, nil
		}
		return securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "fix confirmed"}, nil
	})
	h := securitytesting.NewHarness(toggleable)
	engine, _, tenantID := newTestEngineWithHarness(t, h)
	ctx := ctxWithUser(adminUser(tenantID))

	records, err := engine.RunSuite(ctx, tenantID, securitytesting.CategoryRegression)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	findings, err := engine.OpenFindingsFromRun(ctx, tenantID, records, securitytesting.SeverityCritical)
	if err != nil {
		t.Fatalf("OpenFindingsFromRun: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 opened finding, got %d", len(findings))
	}
	f := findings[0]

	// Move through the state machine to FindingRemediationPending, the
	// only state VerifyRemediation is callable from.
	f, err = engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingTriaged, "")
	if err != nil {
		t.Fatalf("TransitionFinding(->triaged): %v", err)
	}
	f, err = engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingRemediationPending, "")
	if err != nil {
		t.Fatalf("TransitionFinding(->remediation_pending): %v", err)
	}

	t.Run("still-broken re-run does not verify", func(t *testing.T) {
		got, _, err := engine.VerifyRemediation(ctx, tenantID, f.ID)
		if !errors.Is(err, securitytesting.ErrRemediationNotVerified) {
			t.Errorf("VerifyRemediation() error = %v, want ErrRemediationNotVerified", err)
		}
		if got.Status != securitytesting.FindingRemediationPending {
			t.Errorf("Finding.Status after a failed re-run = %v, want unchanged FindingRemediationPending", got.Status)
		}

		// Re-fetch independently to make sure the persisted record (not
		// just the returned value) was left untouched.
		stored, err := engine.ListFindings(ctx, tenantID)
		if err != nil {
			t.Fatalf("ListFindings: %v", err)
		}
		for _, sf := range stored {
			if sf.ID == f.ID && sf.Status != securitytesting.FindingRemediationPending {
				t.Errorf("persisted Finding.Status = %v, want unchanged FindingRemediationPending", sf.Status)
			}
		}
	})

	t.Run("now-fixed re-run verifies", func(t *testing.T) {
		stillBroken = false // the underlying vulnerability is now fixed
		got, rerun, err := engine.VerifyRemediation(ctx, tenantID, f.ID)
		if err != nil {
			t.Fatalf("VerifyRemediation() after fix error = %v, want nil", err)
		}
		if got.Status != securitytesting.FindingVerifiedFixed {
			t.Errorf("Finding.Status after a passing re-run = %v, want FindingVerifiedFixed", got.Status)
		}
		if rerun.Result.Outcome != securitytesting.OutcomePassed {
			t.Errorf("re-run RunRecord outcome = %v, want OutcomePassed", rerun.Result.Outcome)
		}
	})

	t.Run("cannot verify a finding not in remediation_pending", func(t *testing.T) {
		_, _, err := engine.VerifyRemediation(ctx, tenantID, f.ID) // now FindingVerifiedFixed (terminal)
		if err == nil {
			t.Error("VerifyRemediation() on an already-verified finding error = nil, want an error")
		}
	})
}

func TestEngine_ListFindings_RequiresViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.ListFindings(context.Background(), tenantID)
	if !errors.Is(err, securitytesting.ErrUnauthenticated) {
		t.Errorf("ListFindings() unauthenticated error = %v, want ErrUnauthenticated", err)
	}

	unprivileged := ctxWithUser(advocateUser(tenantID))
	_, err = engine.ListFindings(unprivileged, tenantID)
	if !errors.Is(err, securitytesting.ErrForbidden) {
		t.Errorf("ListFindings() as advocate error = %v, want ErrForbidden", err)
	}

	viewerCtx := ctxWithUser(auditorUser(tenantID))
	if _, err := engine.ListFindings(viewerCtx, tenantID); err != nil {
		t.Errorf("ListFindings() as auditor error = %v, want nil", err)
	}
}

func TestEngine_ListOpenFindings(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	f := openTestFinding(t, engine, tenantID, securitytesting.CategoryDataLeakage)
	ctx := ctxWithUser(adminUser(tenantID))

	open, err := engine.ListOpenFindings(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListOpenFindings: %v", err)
	}
	found := false
	for _, o := range open {
		if o.ID == f.ID {
			found = true
		}
	}
	if !found {
		t.Error("ListOpenFindings did not include a freshly-opened FindingOpen record")
	}

	// Risk-accept it and confirm it drops out of ListOpenFindings.
	if _, err := engine.TransitionFinding(ctx, tenantID, f.ID, securitytesting.FindingRiskAccepted, "accepted for test"); err != nil {
		t.Fatalf("TransitionFinding(->risk_accepted): %v", err)
	}
	open, err = engine.ListOpenFindings(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListOpenFindings: %v", err)
	}
	for _, o := range open {
		if o.ID == f.ID {
			t.Error("ListOpenFindings still includes a FindingRiskAccepted (terminal) record")
		}
	}
}
