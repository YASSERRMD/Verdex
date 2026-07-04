package airgapped

import (
	"context"
	"time"
)

// EgressTarget describes one configured endpoint a running deployment
// might reach, as declared by the active configuration -- e.g. a
// provider base URL, a database DSN host, or any other outbound
// dependency the operator has wired in. VerifyZeroEgress consults a
// caller-supplied list of these rather than introspecting the process'
// live sockets, so the check is deterministic and testable against a
// constructed config (task 7).
type EgressTarget struct {
	// Name identifies what this target is, for the report (e.g.
	// "provider:local:llama3:8b", "database").
	Name string `json:"name"`

	// Address is the host, host:port, or URL this target resolves to.
	Address string `json:"address"`
}

// EgressCheckResult is the outcome of checking a single EgressTarget.
type EgressCheckResult struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Allowed bool   `json:"allowed"`
	Detail  string `json:"detail,omitempty"`
}

// EgressReport is the result of VerifyZeroEgress: a point-in-time
// assessment of whether every configured target in a deployment
// resolves to an allowed local/offline address.
type EgressReport struct {
	DeploymentID string              `json:"deployment_id"`
	GeneratedAt  time.Time           `json:"generated_at"`
	Checks       []EgressCheckResult `json:"checks"`
}

// Passed reports whether every check in r succeeded. A report with no
// checks is considered not passed, matching
// dataresidency.Report.Passed's fail-closed convention.
func (r *EgressReport) Passed() bool {
	if r == nil || len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if !c.Allowed {
			return false
		}
	}
	return true
}

// Failures returns the subset of r.Checks that are not allowed.
func (r *EgressReport) Failures() []EgressCheckResult {
	if r == nil {
		return nil
	}
	var out []EgressCheckResult
	for _, c := range r.Checks {
		if !c.Allowed {
			out = append(out, c)
		}
	}
	return out
}

// VerifyZeroEgress asserts that every target in targets resolves to an
// allowed local/offline address under profile's NetworkPolicy (task 7)
// -- a startup check callers run once every configured
// provider/endpoint is known, before accepting live traffic. It never
// dials any target itself; it is a pure address-allow-list assertion,
// matching this phase's "no network call" posture for verification
// steps.
func VerifyZeroEgress(ctx context.Context, profile *Profile, targets []EgressTarget) (EgressReport, error) {
	if profile == nil {
		return EgressReport{}, ErrNilProfile
	}
	if err := ctx.Err(); err != nil {
		return EgressReport{}, err
	}

	policy, err := NewNetworkPolicy(profile)
	if err != nil {
		return EgressReport{}, wrapf("VerifyZeroEgress", err)
	}

	report := EgressReport{
		DeploymentID: profile.DeploymentID.String(),
		GeneratedAt:  time.Now().UTC(),
	}
	for _, t := range targets {
		res := EgressCheckResult{Name: t.Name, Address: t.Address}
		if err := policy.CheckAddress(t.Address); err != nil {
			res.Detail = err.Error()
		} else {
			res.Allowed = true
		}
		report.Checks = append(report.Checks, res)
	}
	return report, nil
}
