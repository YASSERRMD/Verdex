package accessgovernance

import "context"

// Scenario is one representative request a policy author dry-runs
// against a candidate Policy, plus the outcome they expect (task 8).
type Scenario struct {
	// Name labels this scenario for the Result it produces.
	Name string

	// Request is the attribute bundle to evaluate. TenantID/ActorRoles
	// are taken directly from this Request -- TestPolicy evaluates
	// entirely in-memory against the candidate Policy, with no
	// authorization/tenant check of its own (a policy author dry-running
	// a draft policy is not "requesting access", so the ordinary
	// Evaluate authorization gate does not apply here).
	Request Request

	// WantEffect is the DecisionEffect the scenario author expects
	// this Request to resolve to under the candidate Policy.
	WantEffect DecisionEffect
}

// Result is the outcome of dry-running one Scenario: what the policy
// actually produced, and whether it matched WantEffect.
type Result struct {
	Name        string         `json:"name"`
	Got         DecisionEffect `json:"got"`
	Want        DecisionEffect `json:"want"`
	MatchedRule bool           `json:"matched_rule"`
	Passed      bool           `json:"passed"`
}

// Results is the full outcome of a TestPolicy run.
type Results struct {
	Total  int      `json:"total"`
	Passed int      `json:"passed"`
	Failed int      `json:"failed"`
	Cases  []Result `json:"cases"`
}

// AllPassed reports whether every scenario in r matched its expected
// effect.
func (r Results) AllPassed() bool {
	return r.Total > 0 && r.Failed == 0
}

// TestPolicy dry-runs scenarios against policy (task 8): a policy
// author can construct a candidate Policy -- typically with
// Active=false, since TestPolicy evaluates the policy directly and
// does not consult policy.Active at all -- and a representative set of
// Scenarios, and see exactly which ones would be allowed or denied
// before ever activating it via a PolicyRepository. This is a real
// evaluation over the same PolicyRule.matches/Policy.evaluate logic
// Engine.Evaluate uses, not a stub: TestPolicy forces policy.Active to
// true internally (on a copy) purely so evaluate() considers its
// rules, without mutating the caller's policy or requiring the caller
// to activate a policy just to test it.
func TestPolicy(_ context.Context, policy Policy, scenarios []Scenario) (Results, error) {
	if err := policy.Validate(); err != nil {
		return Results{}, err
	}

	candidate := policy
	candidate.Active = true

	// A zero sc.Request.Now is left as-is: TimeWindow.Allows treats a
	// zero time's UTC hour deterministically (hour 0), which is the
	// correct, reproducible behavior for a scenario author who did not
	// care about time-of-day for a given case.
	results := Results{Cases: make([]Result, 0, len(scenarios))}
	for _, sc := range scenarios {
		effect, rule := candidate.evaluate(sc.Request)
		res := Result{
			Name:        sc.Name,
			Got:         effect,
			Want:        sc.WantEffect,
			MatchedRule: rule != nil,
			Passed:      effect == sc.WantEffect,
		}
		results.Cases = append(results.Cases, res)
		results.Total++
		if res.Passed {
			results.Passed++
		} else {
			results.Failed++
		}
	}
	return results, nil
}
