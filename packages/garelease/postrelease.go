package garelease

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PostReleaseCheckKind names one category of post-release verification
// check, mirroring packages/iac.CheckKind's per-check enumeration
// convention exactly (task 9: post-release verification -- health
// endpoint reachable, migration version matches expected, guardrail
// smoke-test passes).
type PostReleaseCheckKind string

const (
	// PostReleaseCheckHealthEndpoint verifies the newly-released
	// deployment's health endpoint is reachable, composing with
	// packages/observability's LivenessHandler/ReadinessHandler and
	// packages/iac.CheckKindHealthEndpoint by reference -- a real
	// caller's Run dials the deployed instance's readiness endpoint.
	// This package does not perform the HTTP call itself.
	PostReleaseCheckHealthEndpoint PostReleaseCheckKind = "health_endpoint"

	// PostReleaseCheckMigrationVersion verifies the deployed instance's
	// applied packages/persistence schema migration version matches the
	// version expected for this release, composing with
	// packages/iac.CheckKindMigrationVersion by reference -- a real
	// caller's Run queries the applied-migrations table and compares
	// against the release's expected version.
	PostReleaseCheckMigrationVersion PostReleaseCheckKind = "migration_version"

	// PostReleaseCheckGuardrailSmokeTest verifies the non-binding-
	// analysis guardrail is actually enforced post-release, composing
	// with packages/guardrail.RequireDisclaimer/CheckText and
	// packages/iac.CheckKindGuardrailSmokeTest by reference. Unlike the
	// other two kinds, this package CAN and does supply a real Run for
	// this one directly -- see DefaultGuardrailSmokeTestCheck -- since
	// packages/guardrail is already an in-process dependency this
	// package composes with for VerifyGuardrails.
	PostReleaseCheckGuardrailSmokeTest PostReleaseCheckKind = "guardrail_smoke_test"

	// PostReleaseCheckCustom is any additional check a caller registers
	// beyond the three named above.
	PostReleaseCheckCustom PostReleaseCheckKind = "custom"
)

// PostReleaseCheckResult is the outcome of running a single
// PostReleaseCheck, mirroring packages/iac.CheckResult's shape exactly.
type PostReleaseCheckResult struct {
	Kind   PostReleaseCheckKind `json:"kind"`
	Name   string               `json:"name"`
	Passed bool                 `json:"passed"`
	Detail string               `json:"detail,omitempty"`
}

// PostReleaseCheck is a single post-release verification step,
// mirroring packages/iac.Check's shape: Run is always caller-supplied
// for PostReleaseCheckHealthEndpoint and PostReleaseCheckMigrationVersion
// (a live HTTP dial and a live database query are both out of scope for
// this orchestration-only package to perform itself), following the
// exact "compose by reference, not duplicate" principle
// packages/iac.Check's own doc comment establishes.
type PostReleaseCheck struct {
	// Kind classifies this check for reporting purposes.
	Kind PostReleaseCheckKind `json:"kind"`

	// Name is a human-readable label for this specific check.
	Name string `json:"name"`

	// Run performs the actual check and returns a non-nil error if it
	// fails. A nil Run is invalid (ErrNilCheckFunc).
	Run func() error `json:"-"`
}

// PostReleaseChecklist is an ordered set of PostReleaseChecks a
// Release's post-release verification runs, mirroring
// packages/iac.Checklist.
type PostReleaseChecklist struct {
	Checks []PostReleaseCheck `json:"checks"`
}

// PostReleaseReport is the result of running a PostReleaseChecklist for
// a Release: every individual PostReleaseCheckResult plus an overall
// Passed aggregation, mirroring
// packages/iac.DeploymentVerificationReport's Passed()/Failures() shape
// exactly.
type PostReleaseReport struct {
	// ReleaseID identifies the Release this verification ran against.
	ReleaseID uuid.UUID `json:"release_id"`

	GeneratedAt time.Time                `json:"generated_at"`
	Results     []PostReleaseCheckResult `json:"results"`
}

// Passed reports whether every check in r succeeded. A report with no
// results is considered not passed, mirroring
// packages/iac.DeploymentVerificationReport.Passed's identical
// fail-closed rule.
func (r *PostReleaseReport) Passed() bool {
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
func (r *PostReleaseReport) Failures() []PostReleaseCheckResult {
	if r == nil {
		return nil
	}
	var out []PostReleaseCheckResult
	for _, c := range r.Results {
		if !c.Passed {
			out = append(out, c)
		}
	}
	return out
}

// DefaultGuardrailSmokeTestCheck returns a real, ready-to-use
// PostReleaseCheck for PostReleaseCheckGuardrailSmokeTest, backed by
// e.VerifyGuardrails -- unlike the health-endpoint and
// migration-version checks (which require dialing a live deployment
// this package has no business reaching into), the guardrail smoke
// test is something this package can and does verify for real using
// the exact same harness CheckReadiness's DimensionGuardrailIntegrity
// dimension runs.
func (e *Engine) DefaultGuardrailSmokeTestCheck() PostReleaseCheck {
	return PostReleaseCheck{
		Kind: PostReleaseCheckGuardrailSmokeTest,
		Name: "guardrail smoke test",
		Run: func() error {
			verification, err := e.VerifyGuardrails()
			if err != nil {
				return err
			}
			if !verification.Passed {
				failures := verification.Failures()
				return fmt.Errorf("%d guardrail assertion(s) failed post-release", len(failures))
			}
			return nil
		},
	}
}

// RunPostReleaseChecklist executes every PostReleaseCheck in checklist
// in order and aggregates the results into a PostReleaseReport for
// releaseID (task 9: post-release verification), requiring
// managePermission. It does not stop at the first failure -- every
// check runs regardless of an earlier one's outcome, mirroring
// packages/iac.RunDeploymentVerification's run-everything-then-aggregate
// behavior. Every call is recorded via AuditSink regardless of outcome.
//
// Returns ErrEmptyChecklist if checklist has no Checks, and
// ErrNilCheckFunc if any PostReleaseCheck.Run is nil.
func (e *Engine) RunPostReleaseChecklist(ctx context.Context, releaseID uuid.UUID, checklist PostReleaseChecklist) (PostReleaseReport, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return PostReleaseReport{}, err
	}

	if len(checklist.Checks) == 0 {
		return PostReleaseReport{}, wrapf("RunPostReleaseChecklist", ErrEmptyChecklist)
	}
	for _, c := range checklist.Checks {
		if c.Run == nil {
			return PostReleaseReport{}, wrapf("RunPostReleaseChecklist", fmt.Errorf("%w: check %q", ErrNilCheckFunc, c.Name))
		}
	}

	report := PostReleaseReport{
		ReleaseID:   releaseID,
		GeneratedAt: e.now(),
	}

	for _, c := range checklist.Checks {
		result := PostReleaseCheckResult{Kind: c.Kind, Name: c.Name}
		if err := c.Run(); err != nil {
			result.Passed = false
			result.Detail = err.Error()
		} else {
			result.Passed = true
		}
		report.Results = append(report.Results, result)
	}

	if e.audit != nil {
		_, _ = e.audit.RecordPostReleaseCheck(ctx, user.ID, releaseID, report)
	}

	return report, nil
}
