package airgapped

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// ConformanceCheckKind classifies a single check performed by
// Conformance, mirroring dataresidency.CheckKind's enumeration
// convention.
type ConformanceCheckKind string

const (
	// CheckProfileValid verifies profile.Validate() succeeds.
	CheckProfileValid ConformanceCheckKind = "profile_valid"

	// CheckProviderAllowlist verifies every provider currently
	// registered in the supplied provider.Registry carries the
	// "local:" prefix.
	CheckProviderAllowlist ConformanceCheckKind = "provider_allowlist"

	// CheckNetworkPolicy verifies profile's AllowedNetworkTargets (plus
	// loopback) is internally well-formed and usable to build a
	// NetworkPolicy.
	CheckNetworkPolicy ConformanceCheckKind = "network_policy"

	// CheckZeroEgress verifies every supplied EgressTarget resolves to
	// an allowed local/offline address.
	CheckZeroEgress ConformanceCheckKind = "zero_egress"
)

// ConformanceCheckResult is the outcome of a single Conformance check.
type ConformanceCheckResult struct {
	Kind   ConformanceCheckKind `json:"kind"`
	Passed bool                 `json:"passed"`
	Detail string               `json:"detail,omitempty"`
}

// ConformanceReport rolls up every air-gap conformance check into one
// pass/fail report with per-check detail (task 8).
type ConformanceReport struct {
	DeploymentID string                   `json:"deployment_id"`
	GeneratedAt  time.Time                `json:"generated_at"`
	Checks       []ConformanceCheckResult `json:"checks"`
}

// Passed reports whether every check in r succeeded. A report with no
// checks is considered not passed, matching this phase's other reports'
// fail-closed convention.
func (r *ConformanceReport) Passed() bool {
	if r == nil || len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// Failures returns the subset of r.Checks that did not pass.
func (r *ConformanceReport) Failures() []ConformanceCheckResult {
	if r == nil {
		return nil
	}
	var out []ConformanceCheckResult
	for _, c := range r.Checks {
		if !c.Passed {
			out = append(out, c)
		}
	}
	return out
}

// ConformanceInput bundles the optional live-state inputs Conformance
// consults beyond profile itself. Both fields may be left zero-valued:
// a nil Registry skips CheckProviderAllowlist's registry scan (it still
// runs, vacuously passing with zero providers inspected), and a nil/
// empty EgressTargets skips CheckZeroEgress's per-target assertions
// (it still runs, vacuously passing with zero targets).
type ConformanceInput struct {
	// Registry is the provider.Registry to audit for non-local
	// providers. Typically provider.DefaultRegistry.
	Registry *provider.Registry

	// EgressTargets is the set of configured endpoints to check via
	// VerifyZeroEgress.
	EgressTargets []EgressTarget
}

// Conformance rolls up profile validity, the provider allowlist, the
// network policy, and zero-egress verification into a single pass/fail
// ConformanceReport (task 8). It is the one function an air-gapped
// deployment's startup path is expected to call before accepting live
// traffic.
func Conformance(ctx context.Context, profile *Profile, input ConformanceInput) (ConformanceReport, error) {
	if profile == nil {
		return ConformanceReport{}, ErrNilProfile
	}
	if err := ctx.Err(); err != nil {
		return ConformanceReport{}, err
	}

	report := ConformanceReport{
		DeploymentID: profile.DeploymentID.String(),
		GeneratedAt:  time.Now().UTC(),
	}

	report.Checks = append(report.Checks, checkProfileValid(profile))
	report.Checks = append(report.Checks, checkProviderAllowlist(input.Registry))

	policy, err := NewNetworkPolicy(profile)
	if err != nil {
		report.Checks = append(report.Checks, ConformanceCheckResult{
			Kind: CheckNetworkPolicy, Passed: false, Detail: err.Error(),
		})
	} else {
		report.Checks = append(report.Checks, ConformanceCheckResult{Kind: CheckNetworkPolicy, Passed: true})
		report.Checks = append(report.Checks, checkZeroEgress(ctx, policy, input.EgressTargets))
	}

	return report, nil
}

func checkProfileValid(profile *Profile) ConformanceCheckResult {
	if err := profile.Validate(); err != nil {
		return ConformanceCheckResult{Kind: CheckProfileValid, Passed: false, Detail: err.Error()}
	}
	return ConformanceCheckResult{Kind: CheckProfileValid, Passed: true}
}

func checkProviderAllowlist(reg *provider.Registry) ConformanceCheckResult {
	violations := AuditRegistry(reg)
	if len(violations) > 0 {
		return ConformanceCheckResult{
			Kind:   CheckProviderAllowlist,
			Passed: false,
			Detail: "non-local providers registered: " + joinStrings(violations),
		}
	}
	return ConformanceCheckResult{Kind: CheckProviderAllowlist, Passed: true}
}

func checkZeroEgress(ctx context.Context, policy *NetworkPolicy, targets []EgressTarget) ConformanceCheckResult {
	for _, t := range targets {
		if err := policy.CheckAddress(t.Address); err != nil {
			return ConformanceCheckResult{
				Kind:   CheckZeroEgress,
				Passed: false,
				Detail: t.Name + " (" + t.Address + "): " + err.Error(),
			}
		}
	}
	_ = ctx
	return ConformanceCheckResult{Kind: CheckZeroEgress, Passed: true}
}

func joinStrings(vals []string) string {
	out := ""
	for i, v := range vals {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
