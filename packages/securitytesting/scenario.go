package securitytesting

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Scenario is the single extension point every adversarial suite in
// this package implements (task 2's harness contract): a named,
// categorized, self-contained probe of one defense. Run must be
// side-effect-free with respect to production data -- a Scenario
// exercises fixtures or lightweight black-box calls against public
// constructors of the packages it probes, never a live tenant's real
// records.
type Scenario interface {
	// Name is a short, stable, human-referenceable identifier for this
	// scenario (e.g. "prompt-injection/role-override-corpus",
	// "authz/expired-grant-still-denied"). Used as RunRecord.ScenarioName
	// and as the natural key a Finding's SourceScenario points back to.
	Name() string

	// Category classifies which adversarial suite this scenario belongs
	// to.
	Category() Category

	// Run executes the scenario and reports whether the defense it
	// probes held. A non-nil error means the scenario itself could not
	// be evaluated (e.g. a malformed fixture) -- distinct from a
	// Result{Outcome: OutcomeFailed}, which means the scenario ran
	// successfully and *found* that the defense did not hold.
	Run(ctx context.Context) (Result, error)
}

// ScenarioFunc adapts a plain function to the Scenario interface,
// mirroring the standard library's http.HandlerFunc adapter pattern --
// most scenarios in this package are a name/category pair plus one
// function body, and do not need a bespoke struct type.
type ScenarioFunc struct {
	name     string
	category Category
	run      func(ctx context.Context) (Result, error)
}

// NewScenarioFunc builds a Scenario from name, category, and run.
// Panics if name is blank or category is not IsValid, since these are
// programmer errors caught at harness-construction time (every call
// site is a package-level var in this codebase's *_suite.go files, not
// user input) -- mirroring packages/threatmodel's SeedThreatModels
// convention of failing loud and immediately on a malformed catalogue
// entry rather than propagating a runtime error through every caller.
func NewScenarioFunc(name string, category Category, run func(ctx context.Context) (Result, error)) ScenarioFunc {
	if name == "" {
		panic("securitytesting: NewScenarioFunc: name must not be blank")
	}
	if !category.IsValid() {
		panic(fmt.Sprintf("securitytesting: NewScenarioFunc(%q): invalid category %q", name, category))
	}
	if run == nil {
		panic(fmt.Sprintf("securitytesting: NewScenarioFunc(%q): run must not be nil", name))
	}
	return ScenarioFunc{name: name, category: category, run: run}
}

// Name implements Scenario.
func (f ScenarioFunc) Name() string { return f.name }

// Category implements Scenario.
func (f ScenarioFunc) Category() Category { return f.category }

// Run implements Scenario.
func (f ScenarioFunc) Run(ctx context.Context) (Result, error) { return f.run(ctx) }

var _ Scenario = ScenarioFunc{}

// Harness runs a fixed collection of Scenarios and reports every
// RunRecord it produced -- the automated security regression suite's
// engine (task 1). A Harness holds no tenant/permission state of its
// own; Engine.RunSuite (engine.go) is the permission-gated,
// audited, persistence-writing wrapper around it. Harness exists as a
// separate, lower-level type so a scenario author can run and debug a
// suite directly in a test without wiring an Engine, an audit sink, or
// a repository.
type Harness struct {
	scenarios []Scenario
	clock     func() time.Time
}

// NewHarness builds a Harness over scenarios. A Harness with zero
// scenarios is valid (RunAll returns an empty slice) -- useful for a
// caller building up a suite incrementally via Add.
func NewHarness(scenarios ...Scenario) *Harness {
	h := &Harness{clock: time.Now}
	h.scenarios = append(h.scenarios, scenarios...)
	return h
}

// Add appends scenario to h's collection, returning h for chaining.
func (h *Harness) Add(scenario Scenario) *Harness {
	h.scenarios = append(h.scenarios, scenario)
	return h
}

// Scenarios returns a copy of h's scenario collection, sorted by Name
// for deterministic iteration -- convenience for a caller (e.g. a CLI
// or a dashboard) that wants to list what a Harness covers without
// running anything.
func (h *Harness) Scenarios() []Scenario {
	out := make([]Scenario, len(h.scenarios))
	copy(out, h.scenarios)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// ByCategory returns every scenario in h whose Category matches cat,
// sorted by Name.
func (h *Harness) ByCategory(cat Category) []Scenario {
	out := make([]Scenario, 0)
	for _, s := range h.scenarios {
		if s.Category() == cat {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// RunAll runs every scenario in h against ctx and tenantID (uuid.Nil
// for a platform-wide run with no single tenant target), returning one
// RunRecord per scenario in deterministic Name order. A Scenario whose
// Run call returns a non-nil error produces a RunRecord with
// Result.Outcome = OutcomeError rather than being dropped from the
// output -- a Harness caller must always get back exactly
// len(h.scenarios) records, so a silently-skipped scenario can never
// be mistaken for a passing one.
func (h *Harness) RunAll(ctx context.Context, tenantID uuid.UUID, runBy uuid.UUID) []RunRecord {
	scenarios := h.Scenarios()
	out := make([]RunRecord, 0, len(scenarios))
	for _, s := range scenarios {
		out = append(out, h.runOne(ctx, s, tenantID, runBy))
	}
	return out
}

// RunCategory runs only the scenarios in h belonging to cat, otherwise
// identical to RunAll.
func (h *Harness) RunCategory(ctx context.Context, cat Category, tenantID uuid.UUID, runBy uuid.UUID) []RunRecord {
	scenarios := h.ByCategory(cat)
	out := make([]RunRecord, 0, len(scenarios))
	for _, s := range scenarios {
		out = append(out, h.runOne(ctx, s, tenantID, runBy))
	}
	return out
}

// RunOne runs the single scenario in h whose Name equals name, and
// reports (RunRecord{}, false) if no such scenario is registered --
// used by Engine.VerifyRemediation to re-run exactly the scenario a
// Finding originated from.
func (h *Harness) RunOne(ctx context.Context, name string, tenantID uuid.UUID, runBy uuid.UUID) (RunRecord, bool) {
	for _, s := range h.scenarios {
		if s.Name() == name {
			return h.runOne(ctx, s, tenantID, runBy), true
		}
	}
	return RunRecord{}, false
}

func (h *Harness) runOne(ctx context.Context, s Scenario, tenantID uuid.UUID, runBy uuid.UUID) RunRecord {
	result, err := s.Run(ctx)
	if err != nil {
		result = Result{Outcome: OutcomeError, Detail: err.Error()}
	} else if verr := result.Validate(); verr != nil {
		// A Scenario that returns a structurally invalid Result (e.g. a
		// blank Detail) is itself a harness bug -- surfaced as
		// OutcomeError rather than silently accepted, so "the harness
		// can fail loudly" extends to malformed scenario implementations
		// too, not just to the vulnerabilities it is looking for.
		result = Result{Outcome: OutcomeError, Detail: "scenario returned invalid result: " + verr.Error()}
	}
	return RunRecord{
		ID:               uuid.New(),
		TenantID:         tenantID,
		ScenarioName:     s.Name(),
		ScenarioCategory: s.Category(),
		Result:           result,
		RunBy:            runBy,
		RanAt:            h.now(),
	}
}

func (h *Harness) now() time.Time {
	if h.clock != nil {
		return h.clock().UTC()
	}
	return time.Now().UTC()
}

// FailedRecords filters records down to those with Result.Outcome ==
// OutcomeFailed -- the subset that should become Findings.
func FailedRecords(records []RunRecord) []RunRecord {
	out := make([]RunRecord, 0)
	for _, r := range records {
		if r.Result.Outcome == OutcomeFailed {
			out = append(out, r)
		}
	}
	return out
}

// ErroredRecords filters records down to those with Result.Outcome ==
// OutcomeError -- runs that were inconclusive and need attention
// distinct from either a pass or a confirmed vulnerability.
func ErroredRecords(records []RunRecord) []RunRecord {
	out := make([]RunRecord, 0)
	for _, r := range records {
		if r.Result.Outcome == OutcomeError {
			out = append(out, r)
		}
	}
	return out
}

// AllPassed reports whether every record in records has Result.Outcome
// == OutcomePassed -- the "is the suite green" check a CI gate wants,
// deliberately strict: an OutcomeError run counts as not-all-passed,
// since an inconclusive run must never be silently treated as green.
func AllPassed(records []RunRecord) bool {
	for _, r := range records {
		if r.Result.Outcome != OutcomePassed {
			return false
		}
	}
	return true
}
