package securitytesting_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

func alwaysPass(name string, cat securitytesting.Category) securitytesting.Scenario {
	return securitytesting.NewScenarioFunc(name, cat, func(_ context.Context) (securitytesting.Result, error) {
		return securitytesting.Result{Outcome: securitytesting.OutcomePassed, Detail: "ok"}, nil
	})
}

// alwaysFail and alwaysErrors are always registered under
// CategoryRegression at every call site in this package's tests, so
// (unlike alwaysPass, which callers also register under other
// categories to exercise Harness.ByCategory) they take no Category
// parameter.
func alwaysFail(name string) securitytesting.Scenario {
	return securitytesting.NewScenarioFunc(name, securitytesting.CategoryRegression, func(_ context.Context) (securitytesting.Result, error) {
		return securitytesting.Result{Outcome: securitytesting.OutcomeFailed, Detail: "found it"}, nil
	})
}

func alwaysErrors(name string) securitytesting.Scenario {
	return securitytesting.NewScenarioFunc(name, securitytesting.CategoryRegression, func(_ context.Context) (securitytesting.Result, error) {
		return securitytesting.Result{}, errors.New("scenario blew up")
	})
}

func TestNewScenarioFunc_PanicsOnBadInput(t *testing.T) {
	t.Parallel()

	t.Run("blank name panics", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if recover() == nil {
				t.Error("NewScenarioFunc with blank name did not panic")
			}
		}()
		securitytesting.NewScenarioFunc("", securitytesting.CategoryRegression, func(context.Context) (securitytesting.Result, error) {
			return securitytesting.Result{}, nil
		})
	})

	t.Run("invalid category panics", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if recover() == nil {
				t.Error("NewScenarioFunc with invalid category did not panic")
			}
		}()
		securitytesting.NewScenarioFunc("x", securitytesting.Category("bogus"), func(context.Context) (securitytesting.Result, error) {
			return securitytesting.Result{}, nil
		})
	})

	t.Run("nil run func panics", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if recover() == nil {
				t.Error("NewScenarioFunc with nil run func did not panic")
			}
		}()
		securitytesting.NewScenarioFunc("x", securitytesting.CategoryRegression, nil)
	})
}

func TestHarness_RunAll_NeverDropsAScenario(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(
		alwaysPass("a", securitytesting.CategoryRegression),
		alwaysFail("b"),
		alwaysErrors("c"),
	)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)
	if len(records) != 3 {
		t.Fatalf("RunAll() returned %d records, want exactly 3 (one per scenario, none dropped)", len(records))
	}

	byName := make(map[string]securitytesting.RunRecord, len(records))
	for _, r := range records {
		byName[r.ScenarioName] = r
	}
	if byName["a"].Result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("scenario a outcome = %v, want OutcomePassed", byName["a"].Result.Outcome)
	}
	if byName["b"].Result.Outcome != securitytesting.OutcomeFailed {
		t.Errorf("scenario b outcome = %v, want OutcomeFailed", byName["b"].Result.Outcome)
	}
	if byName["c"].Result.Outcome != securitytesting.OutcomeError {
		t.Errorf("scenario c (which returned a non-nil error) outcome = %v, want OutcomeError -- an erroring scenario must never be silently dropped or treated as passing", byName["c"].Result.Outcome)
	}
}

func TestHarness_RunAll_DeterministicOrder(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(
		alwaysPass("zeta", securitytesting.CategoryRegression),
		alwaysPass("alpha", securitytesting.CategoryRegression),
		alwaysPass("mike", securitytesting.CategoryRegression),
	)
	scenarios := h.Scenarios()
	if len(scenarios) != 3 {
		t.Fatalf("Scenarios() returned %d, want 3", len(scenarios))
	}
	if scenarios[0].Name() != "alpha" || scenarios[1].Name() != "mike" || scenarios[2].Name() != "zeta" {
		t.Errorf("Scenarios() order = [%s, %s, %s], want alphabetical [alpha, mike, zeta]", scenarios[0].Name(), scenarios[1].Name(), scenarios[2].Name())
	}
}

func TestHarness_ByCategory(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(
		alwaysPass("a", securitytesting.CategoryRegression),
		alwaysPass("b", securitytesting.CategoryAbuseCase),
		alwaysPass("c", securitytesting.CategoryRegression),
	)
	got := h.ByCategory(securitytesting.CategoryRegression)
	if len(got) != 2 {
		t.Fatalf("ByCategory(CategoryRegression) returned %d, want 2", len(got))
	}
	for _, s := range got {
		if s.Category() != securitytesting.CategoryRegression {
			t.Errorf("ByCategory returned scenario %s with category %v, want CategoryRegression", s.Name(), s.Category())
		}
	}
}

func TestHarness_RunOne(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(alwaysPass("target", securitytesting.CategoryRegression))

	record, ok := h.RunOne(t.Context(), "target", uuid.Nil, uuid.Nil)
	if !ok {
		t.Fatal("RunOne(\"target\") ok = false, want true")
	}
	if record.Result.Outcome != securitytesting.OutcomePassed {
		t.Errorf("RunOne(\"target\") outcome = %v, want OutcomePassed", record.Result.Outcome)
	}

	_, ok = h.RunOne(t.Context(), "does-not-exist", uuid.Nil, uuid.Nil)
	if ok {
		t.Error("RunOne(\"does-not-exist\") ok = true, want false")
	}
}

func TestFailedRecordsAndErroredRecords(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness(
		alwaysPass("a", securitytesting.CategoryRegression),
		alwaysFail("b"),
		alwaysErrors("c"),
	)
	records := h.RunAll(t.Context(), uuid.Nil, uuid.Nil)

	failed := securitytesting.FailedRecords(records)
	if len(failed) != 1 || failed[0].ScenarioName != "b" {
		t.Errorf("FailedRecords() = %+v, want exactly scenario b", failed)
	}

	errored := securitytesting.ErroredRecords(records)
	if len(errored) != 1 || errored[0].ScenarioName != "c" {
		t.Errorf("ErroredRecords() = %+v, want exactly scenario c", errored)
	}
}

func TestAllPassed(t *testing.T) {
	t.Parallel()

	t.Run("all passing", func(t *testing.T) {
		t.Parallel()
		h := securitytesting.NewHarness(alwaysPass("a", securitytesting.CategoryRegression), alwaysPass("b", securitytesting.CategoryRegression))
		if !securitytesting.AllPassed(h.RunAll(t.Context(), uuid.Nil, uuid.Nil)) {
			t.Error("AllPassed() = false, want true when every scenario passed")
		}
	})

	t.Run("one failing", func(t *testing.T) {
		t.Parallel()
		h := securitytesting.NewHarness(alwaysPass("a", securitytesting.CategoryRegression), alwaysFail("b"))
		if securitytesting.AllPassed(h.RunAll(t.Context(), uuid.Nil, uuid.Nil)) {
			t.Error("AllPassed() = true, want false when a scenario failed")
		}
	})

	t.Run("one erroring counts as not all passed", func(t *testing.T) {
		t.Parallel()
		h := securitytesting.NewHarness(alwaysPass("a", securitytesting.CategoryRegression), alwaysErrors("c"))
		if securitytesting.AllPassed(h.RunAll(t.Context(), uuid.Nil, uuid.Nil)) {
			t.Error("AllPassed() = true, want false when a scenario errored -- an inconclusive run must never count as green")
		}
	})
}

func TestHarness_Add_Chaining(t *testing.T) {
	t.Parallel()

	h := securitytesting.NewHarness()
	h.Add(alwaysPass("a", securitytesting.CategoryRegression)).Add(alwaysPass("b", securitytesting.CategoryRegression))
	if len(h.Scenarios()) != 2 {
		t.Errorf("len(Scenarios()) = %d, want 2 after two chained Add calls", len(h.Scenarios()))
	}
}
