package iac

import (
	"fmt"
	"time"
)

// CheckKind names one category of post-deploy verification check,
// mirroring packages/dataresidency.CheckKind's/
// packages/airgapped.ConformanceCheckKind's per-check enumeration
// convention. This package does not import
// packages/observability, packages/persistence, or packages/guardrail
// to perform these checks itself -- see Check.Run's doc comment for
// why the actual check logic is always caller-supplied.
type CheckKind string

const (
	// CheckKindHealthEndpoint verifies a deployment's health endpoint
	// is reachable, composing with packages/observability's
	// LivenessHandler/ReadinessHandler ("/healthz"/"/readyz") by
	// reference: a real caller's Check.Run dials the deployed
	// instance's readiness endpoint and returns an error if it does not
	// respond 200 OK, exactly what ReadinessHandler already serves.
	// This package does not perform the HTTP call itself.
	CheckKindHealthEndpoint CheckKind = "health_endpoint"

	// CheckKindMigrationVersion verifies the deployed instance's
	// applied schema migration version matches the expected version,
	// composing with packages/persistence's migration runner (Phase
	// 003's migrate.go, which already tracks a
	// schema_migrations-style applied-version table) by reference: a
	// real caller's Check.Run queries that table and compares against
	// the release's expected version.
	CheckKindMigrationVersion CheckKind = "migration_version"

	// CheckKindGuardrailSmokeTest verifies the non-binding-analysis
	// guardrail is actually enforced post-deploy, composing with
	// packages/guardrail.RequireDisclaimer/CheckText (Phase 057) by
	// reference: a real caller's Check.Run submits a small sample
	// reasoning request to the deployed instance and asserts the
	// response carries the mandatory disclaimer / rejects verdict
	// language, exactly what packages/guardrail already enforces
	// in-process.
	CheckKindGuardrailSmokeTest CheckKind = "guardrail_smoke_test"

	// CheckKindCustom is any additional check a caller registers beyond
	// the three named above (e.g. a deployment-specific smoke test).
	// Not every deployment verification checklist is limited to
	// exactly these three -- see Checklist's doc comment.
	CheckKindCustom CheckKind = "custom"
)

// CheckResult is the outcome of running a single Check.
type CheckResult struct {
	Kind   CheckKind `json:"kind"`
	Name   string    `json:"name"`
	Passed bool      `json:"passed"`
	Detail string    `json:"detail,omitempty"`
}

// Check is a single post-deploy verification step. Run is always
// caller-supplied: this package defines the checklist/aggregation
// machinery, never the concrete probe logic for a live deployment
// (that would require importing packages/observability's HTTP client
// machinery, packages/persistence's database driver, and
// packages/guardrail's disclaimer text into a package whose only job
// is orchestration -- exactly the "compose by reference, not
// duplicate" principle packages/compliance's MappedTo convention
// already established for this monorepo).
type Check struct {
	// Kind classifies this check for reporting purposes.
	Kind CheckKind `json:"kind"`

	// Name is a human-readable label for this specific check (e.g.
	// "gateway /readyz", "compliance_controls migration v41").
	Name string `json:"name"`

	// Run performs the actual check and returns a non-nil error if it
	// fails. A nil Run is invalid (ErrNilCheckFunc) -- there is no
	// meaningful "always passes" default check.
	Run func() error `json:"-"`
}

// Checklist is an ordered set of Checks a DeploymentVerification runs.
// Not limited to exactly CheckKindHealthEndpoint/
// CheckKindMigrationVersion/CheckKindGuardrailSmokeTest -- a caller may
// append any number of CheckKindCustom checks specific to their own
// deployment.
type Checklist struct {
	Checks []Check `json:"checks"`
}

// DeploymentVerificationReport is the result of running a Checklist:
// every individual CheckResult plus an overall Passed aggregation,
// mirroring packages/airgapped.ConformanceReport/
// packages/dataresidency.Report's Passed()/Failures() shape.
type DeploymentVerificationReport struct {
	// DeploymentID identifies the deployment this verification ran
	// against, matching packages/iac.DeploymentProfile.DeploymentID.
	DeploymentID string `json:"deployment_id"`

	// Stage names the promotion stage this verification was run for
	// (e.g. "staging"), when run as part of a PromotionPipeline. Empty
	// when DeploymentVerification is run standalone, outside any
	// pipeline.
	Stage Stage `json:"stage,omitempty"`

	GeneratedAt time.Time     `json:"generated_at"`
	Results     []CheckResult `json:"results"`
}

// Passed reports whether every check in r succeeded. A report with no
// results is considered not passed -- RunDeploymentVerification never
// returns an empty, vacuously-true report, matching this package's
// other report types' fail-closed convention.
func (r *DeploymentVerificationReport) Passed() bool {
	if r == nil || len(r.Results) == 0 {
		return false
	}
	for _, c := range r.Results {
		if !c.Passed {
			return false
		}
	}
	return true
}

// Failures returns the subset of r.Results that did not pass.
func (r *DeploymentVerificationReport) Failures() []CheckResult {
	if r == nil {
		return nil
	}
	var out []CheckResult
	for _, c := range r.Results {
		if !c.Passed {
			out = append(out, c)
		}
	}
	return out
}

// RunDeploymentVerification executes every Check in checklist in
// order and aggregates the results into a
// DeploymentVerificationReport. It does not stop at the first
// failure -- every check runs regardless of an earlier one's outcome,
// so a caller sees the full picture of what is and is not healthy
// post-deploy, mirroring packages/airgapped.Conformance's
// run-everything-then-aggregate behavior.
//
// Returns ErrEmptyChecklist if checklist has no Checks, and
// ErrNilCheckFunc if any Check.Run is nil (a checklist entry with no
// actual check is a configuration bug, not a check that vacuously
// passes).
func RunDeploymentVerification(deploymentID string, stage Stage, checklist Checklist) (DeploymentVerificationReport, error) {
	if len(checklist.Checks) == 0 {
		return DeploymentVerificationReport{}, ErrEmptyChecklist
	}
	for _, c := range checklist.Checks {
		if c.Run == nil {
			return DeploymentVerificationReport{}, wrapf("RunDeploymentVerification", fmt.Errorf("%w: check %q", ErrNilCheckFunc, c.Name))
		}
	}

	report := DeploymentVerificationReport{
		DeploymentID: deploymentID,
		Stage:        stage,
		GeneratedAt:  time.Now().UTC(),
	}

	for _, c := range checklist.Checks {
		result := CheckResult{Kind: c.Kind, Name: c.Name}
		if err := c.Run(); err != nil {
			result.Passed = false
			result.Detail = err.Error()
		} else {
			result.Passed = true
		}
		report.Results = append(report.Results, result)
	}

	return report, nil
}
