package e2e

import (
	"context"
	"sort"
	"time"
)

// SuiteRecord is what one Suite.RunAll/RunOne call produces for a
// single Scenario: its ScenarioResult, plus bookkeeping for reporting.
// Mirrors packages/securitytesting.RunRecord's shape (a Scenario ran,
// here is what happened), scoped to this package's own ScenarioResult
// rather than securitytesting's adversarial Result.
type SuiteRecord struct {
	// ScenarioName is the Scenario.Name() this record reports on.
	ScenarioName string

	// ScenarioCategory is the Scenario.CaseCategory() this record
	// reports on.
	ScenarioCategory string

	// Result is the scenario's ScenarioResult. If the Scenario's Run
	// method itself returned a non-nil error, Result.Outcome is
	// OutcomeErrored and Result.Detail carries the error text.
	Result ScenarioResult

	// Attempts is how many times RetryOnFlake (or a direct single call,
	// Attempts == 1) invoked the scenario before landing on Result --
	// see flaky.go.
	Attempts int

	// RanAt is when this record was produced.
	RanAt time.Time
}

// Suite runs a fixed collection of Scenarios and reports one
// SuiteRecord per scenario -- this package's own lightweight
// Scenario-runner, built local to this package rather than importing
// packages/securitytesting.Harness: that type's Run/RunAll signature is
// shaped around securitytesting.Scenario (Result, tenant/permission
// bookkeeping, RunRecord persistence via Engine) which does not fit
// this package's plain-function, no-persistence, no-tenant-gating
// full-journey Scenario shape without an awkward adapter layer for
// every call site. The two types share the same Name/Category/Run
// design vocabulary deliberately (see doc/e2e-suite.md's "reused, not
// duplicated" section) without one importing the other.
type Suite struct {
	scenarios []Scenario
	clock     func() time.Time
}

// NewSuite builds a Suite over scenarios. A Suite with zero scenarios
// is valid (RunAll returns an empty slice).
func NewSuite(scenarios ...Scenario) *Suite {
	s := &Suite{clock: time.Now}
	s.scenarios = append(s.scenarios, scenarios...)
	return s
}

// Add appends scenario to s's collection, returning s for chaining.
func (s *Suite) Add(scenario Scenario) *Suite {
	s.scenarios = append(s.scenarios, scenario)
	return s
}

// Scenarios returns a copy of s's scenario collection, sorted by Name
// for deterministic iteration.
func (s *Suite) Scenarios() []Scenario {
	out := make([]Scenario, len(s.scenarios))
	copy(out, s.scenarios)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// RunAll runs every scenario in s against ctx, in deterministic Name
// order, returning one SuiteRecord per scenario -- a caller must always
// get back exactly len(s.scenarios) records, so a scenario that errored
// can never be mistaken for one silently skipped.
func (s *Suite) RunAll(ctx context.Context) []SuiteRecord {
	scenarios := s.Scenarios()
	out := make([]SuiteRecord, 0, len(scenarios))
	for _, sc := range scenarios {
		out = append(out, s.runOne(ctx, sc))
	}
	return out
}

// RunOne runs the single scenario in s whose Name equals name, and
// reports (SuiteRecord{}, false) if no such scenario is registered.
func (s *Suite) RunOne(ctx context.Context, name string) (SuiteRecord, bool) {
	for _, sc := range s.scenarios {
		if sc.Name() == name {
			return s.runOne(ctx, sc), true
		}
	}
	return SuiteRecord{}, false
}

func (s *Suite) runOne(ctx context.Context, sc Scenario) SuiteRecord {
	result, err := sc.Run(ctx)
	if err != nil {
		result = ScenarioResult{Outcome: OutcomeErrored, Detail: err.Error()}
	} else if verr := result.Validate(); verr != nil {
		result = ScenarioResult{Outcome: OutcomeErrored, Detail: "scenario returned invalid result: " + verr.Error()}
	}
	return SuiteRecord{
		ScenarioName:     sc.Name(),
		ScenarioCategory: string(sc.CaseCategory()),
		Result:           result,
		Attempts:         1,
		RanAt:            s.now(),
	}
}

func (s *Suite) now() time.Time {
	if s.clock != nil {
		return s.clock().UTC()
	}
	return time.Now().UTC()
}

// AllPassed reports whether every record in records has
// Result.Outcome == OutcomePassed -- the "is the suite green" check a
// CI gate wants, deliberately strict: an OutcomeErrored run counts as
// not-all-passed, since an inconclusive run must never be silently
// treated as green.
func AllPassed(records []SuiteRecord) bool {
	for _, r := range records {
		if r.Result.Outcome != OutcomePassed {
			return false
		}
	}
	return true
}

// FailedRecords filters records down to those with Result.Outcome ==
// OutcomeFailed.
func FailedRecords(records []SuiteRecord) []SuiteRecord {
	out := make([]SuiteRecord, 0)
	for _, r := range records {
		if r.Result.Outcome == OutcomeFailed {
			out = append(out, r)
		}
	}
	return out
}

// ErroredRecords filters records down to those with Result.Outcome ==
// OutcomeErrored.
func ErroredRecords(records []SuiteRecord) []SuiteRecord {
	out := make([]SuiteRecord, 0)
	for _, r := range records {
		if r.Result.Outcome == OutcomeErrored {
			out = append(out, r)
		}
	}
	return out
}
