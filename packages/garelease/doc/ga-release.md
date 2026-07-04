# General availability hardening & release (Phase 100)

## Goal

Phase 100 is the final phase of Verdex's 100-phase build. It is the
capstone: `packages/garelease` aggregates every governance, quality,
and security signal the platform has produced across its entire
history into one release-readiness gate, models the GA (general
availability) cut itself, and composes with `packages/cicdgate` to
produce release artifacts, attestation, and release notes -- without
duplicating anything any of those upstream packages already do well.

See this package's `doc.go` for the full eight-part summary of the
100-phase plan this capstone sits on top of.

## What this package composes from, versus what is new

Nothing in this package reimplements a check another phase already
performs. What is new is the aggregation layer and the
freeze/cut lifecycle itself:

| Concern | Owned by | This package's role |
|---|---|---|
| Critical/high vulnerability findings | `packages/vulnmanagement` (Phase 084) | Reads `Finding`/`Status` directly as a `ReadinessInput` field |
| Adversarial-probe findings | `packages/securitytesting` (Phase 086) | Reads `Finding`/`FindingStatus` directly |
| Compliance gap analysis | `packages/compliance` (Phase 082) | Reads `GapAnalysisReport` directly |
| Performance budget evaluation | `packages/perf` (Phase 091) | Reads `Verdict` directly |
| Full-journey E2E regression | `packages/e2e` (Phase 097), run as a CI gate | Accepts a typed `E2EResult`, does not import `packages/e2e` |
| Non-binding guardrail enforcement | `packages/guardrail` (Phase 057) | Calls `CheckText`/`RequireDisclaimer`/`HasDisclaimer`/`ValidateLabeled` live against fixtures |
| Durable, hash-chained audit trail | `packages/auditlog` (Phase 077) | Calls `Query`/`VerifyTenantChain` live via a `Store`-shaped interface |
| Signed builds, attestation, release notes | `packages/cicdgate` (Phase 095) | Calls `Verify`/`GenerateReleaseNotes` directly |
| Post-deploy verification shape | `packages/iac` (Phase 094) | `PostReleaseChecklist` mirrors `Checklist`/`Check`/`DeploymentVerificationReport` by reference, no import |

## `ReadinessCheck` / `ReleaseReadiness` / `Engine.CheckReadiness`

A `ReadinessCheck` is one named dimension's outcome: a `DimensionName`,
a `CheckStatus` (`CheckPassed`/`CheckFailed`), a `Detail` explaining
what was evaluated, and an `EvaluatedAt` timestamp. `ReleaseReadiness`
aggregates every dimension's `ReadinessCheck` into one snapshot: `Ready`
is `true` only when every check passed. An empty `Checks` slice is
never considered `Ready` -- a readiness snapshot with nothing evaluated
is a configuration bug, not a vacuously green release.

`Engine.CheckReadiness` computes all six dimensions from a
`ReadinessInput`:

```go
input := garelease.ReadinessInput{
    VulnFindings:            findings,       // []vulnmanagement.Finding
    SecurityTestingFindings: probeFindings,  // []securitytesting.Finding
    ComplianceGapReport:     gapReport,       // compliance.GapAnalysisReport
    PerfVerdicts:            verdicts,        // []perf.Verdict
    E2EResult: garelease.E2EResult{
        TotalScenarios:  42,
        PassedScenarios: 42,
        SourceCIRunURL:  "https://github.com/.../actions/runs/123456",
    },
}

readiness, err := engine.CheckReadiness(ctx, input)
if err != nil {
    // A genuine operational failure (missing permission, a nil
    // dependency an internal check needs) -- not "the platform isn't
    // ready yet".
}
if !readiness.Ready {
    for _, failed := range readiness.FailedChecks() {
        log.Printf("blocking: %s -- %s", failed.Dimension, failed.Detail)
    }
}
```

`CheckReadiness` itself never returns a non-nil error for a merely
unready platform: `ReleaseReadiness{Ready: false}` IS the correct,
successful result of evaluating a platform that is not yet ready to
release.

### Why `E2EResult` is a plain struct, not an import of `packages/e2e`

`packages/e2e`'s own `go.mod` pulls in `packages/ingestion`,
`packages/reasoningorchestration`, `packages/signoff`,
`packages/category`, and the rest of that suite's transitive dependency
tree. Importing it here purely to reuse a "scenario name + pass/fail +
detail" shape would drag that entire tree into `packages/garelease`'s
own module graph for a single read-only report type. Task 5's "run
full regression and E2E" is already satisfied by the existing CI gate:
`packages/e2e`'s suite already runs in CI as a blocking check before
this package's readiness check is ever consulted for a real release.
This package accepts that suite's outcome as an explicit, typed input
sourced from CI -- it does not, and should not, re-invoke the suite
itself.

## `Engine.VerifyGuardrails` -- a harness that can fail

Task 6's guardrail half requires a REAL check, not a rubber stamp.
`VerifyGuardrails` makes four real calls into `packages/guardrail`'s
own functions:

1. `guardrail.CheckText` against a deliberately verdict-worded fixture
   (`"...the defendant shall pay damages of $50,000..."`) -- must
   return a non-nil error.
2. `guardrail.CheckText` against a properly-labeled, non-binding draft
   analysis fixture -- must return `nil` (no false positive on
   ordinary legal prose).
3. `guardrail.HasDisclaimer`/`guardrail.RequireDisclaimer` -- the
   properly-labeled fixture must NOT carry the disclaimer before
   `RequireDisclaimer` runs, MUST carry it after, and a second
   `RequireDisclaimer` call must be a no-op (idempotent).
4. `guardrail.ValidateLabeled` -- a value labeled exactly
   `guardrail.DraftAnalysisLabel` must pass; a value labeled
   `"final_verdict"` must fail.

All four assertions run every time `VerifyGuardrails` is called; all
four must hold for `GuardrailVerification.Passed`. This mirrors
`packages/pilot.ValidateNonBindingCompliance`'s and
`packages/securitytesting/selftest.go`'s identical
vulnerable-fixture/fixed-fixture proof pattern: a harness that cannot
fail is worthless, so this package's own tests
(`guardrail_test.go`) prove the underlying `packages/guardrail`
functions genuinely discriminate rather than trusting the assertion
logic blindly.

## `Engine.VerifyAuditTrail` -- a real structural check

Task 6's audit half checks two things against an `AuditTrailStore`
-shaped dependency (a small interface a real `*packages/auditlog.Store`
satisfies directly, so this package never needs its own competing audit
implementation):

1. `store.Query` succeeds for a representative tenant -- the store is
   genuinely queryable, not merely constructed. A query *error* is a
   hard failure; an *empty result set* is not (a freshly-provisioned
   tenant with no audit history yet is not itself a broken trail).
2. `store.VerifyTenantChain` reports `valid=true` -- the hash-chain
   tamper-evidence guarantee `packages/auditlog` established in Phase
   077 actually holds, not merely assumed.

This package has no tenant registry of its own to iterate every tenant
(`ReleaseCandidate`/`Release` are platform-global, see below), so a
caller supplies one `representativeTenantID` to spot-check as
representative of the whole store's health.

## `ReleaseCandidate` / `Engine.FreezeReleaseCandidate` (task 4)

```go
readiness, _ := engine.CheckReadiness(ctx, input)
if !readiness.Ready {
    return errors.New("not ready to freeze a candidate yet")
}

candidate, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", commitSHA, readiness)
// err is ErrReadinessNotReady if readiness.Ready was false -- refused
// outright, nothing persisted. See candidate_test.go's
// TestFreezeReleaseCandidate_RefusesUnreadyCandidate for the explicit
// proof.
```

`FreezeReleaseCandidate` does not itself call `CheckReadiness` -- the
caller passes in the exact snapshot that justified freezing, so there
is no time-of-check/time-of-use gap between evaluating readiness and
freezing against it.

## `Release` / `Engine.CutRelease` (task 8)

```go
release, err := engine.CutRelease(ctx, candidate.ID)
// Requires: candidate exists, candidate.Readiness.Ready is STILL true
// (re-checked, not merely trusted from freeze time), and the candidate
// has not already been cut (ErrAlreadyReleased otherwise -- CutRelease
// is not idempotent-by-retry).
```

## The git-tag boundary

**Neither `FreezeReleaseCandidate` nor `CutRelease` shells out to `git
tag`.** Both model the software-side record only. The actual git tag
against a frozen candidate's `CommitSHA` is a separate, deliberate step
a human or an orchestrating CI job performs *after* this package's
records (and the pull request introducing them) have merged. This
boundary is deliberate: freezing/cutting a software record and tagging
a repository are two different actions with two different blast radii.
Conflating them into one Go function call would make an irreversible
git-history action (a pushed tag) an unreviewable side effect of
calling a library function. `packages/garelease` never contains a call
to `os/exec` or any git-plumbing library, by design.

## Release artifacts, attestation, and release notes (tasks 7, 10)

```go
artifact := cicdgate.ReleaseArtifact{
    Name:            "verdex-platform@1.100.0",
    DigestAlgorithm: cicdgate.DigestSHA256,
    Digest:          sha256Hex,
    SignatureState:  cicdgate.SignatureStatePlaceholder,
    SignatureRef:    "placeholder-ref-001",
    BuiltAt:         buildFinishedAt,
}
attestation := cicdgate.BuildAttestation{
    SourceCommit:          commitSHA,
    BuilderID:             "github-actions/phase-100-ga-release",
    BuildTimestamp:        buildStartedAt,
    InputsDigestAlgorithm: cicdgate.DigestSHA256,
    InputsDigest:          inputsDigestHex,
}

artifact, attestation, err := garelease.BuildReleaseArtifactAttestation(artifact, attestation)
// err is non-nil if cicdgate.Verify finds the pair inconsistent or
// either record individually malformed.

notes := garelease.BuildReleaseNotes(commits) // []cicdgate.CommitInfo
```

`packages/cicdgate` exposes no `NewReleaseArtifact`/`NewBuildAttestation`
constructor function -- both types are plain structs built via literal
and checked with their own `Validate()` methods (confirmed directly
against that package's source before writing `artifact.go`). This
package composes with `cicdgate.Verify` and
`cicdgate.GenerateReleaseNotes` unmodified; it adds no artifact
validation or changelog-generation logic of its own. See
`CHANGELOG.md` at the repository root for the human-authored,
eight-part-plan-level summary these generated release notes
complement.

## `PostReleaseChecklist` (task 9)

Mirrors `packages/iac.DeploymentVerification`'s `Checklist`/`Check`/
`DeploymentVerificationReport` shape exactly, applied to a `Release`
instead of a deployment:

```go
checklist := garelease.PostReleaseChecklist{
    Checks: []garelease.PostReleaseCheck{
        {Kind: garelease.PostReleaseCheckHealthEndpoint, Name: "gateway /readyz", Run: dialReadyzEndpoint},
        {Kind: garelease.PostReleaseCheckMigrationVersion, Name: "migration version", Run: checkMigrationVersion},
        engine.DefaultGuardrailSmokeTestCheck(), // real, backed by VerifyGuardrails
    },
}

report, err := engine.RunPostReleaseChecklist(ctx, release.ID, checklist)
if !report.Passed() {
    for _, f := range report.Failures() {
        log.Printf("post-release check failed: %s -- %s", f.Name, f.Detail)
    }
}
```

The health-endpoint and migration-version checks remain caller-supplied
`Run` functions, for the same reason `packages/iac.Check.Run` is always
caller-supplied: this package has no business dialing a live HTTP
endpoint or opening a live database connection just to verify a
release. The guardrail smoke-test check is the one exception --
`DefaultGuardrailSmokeTestCheck` is real and ready-to-use, backed
directly by `Engine.VerifyGuardrails`, since `packages/guardrail` is
already an in-process dependency this package composes with.

## Persistence: platform-global, not tenant-scoped

A software release is not a per-tenant concept: one `ReleaseCandidate`
and one `Release` are shared by every tenant of a given deployment.
This mirrors `packages/compliance.Control`'s shared catalogue exactly
(see `packages/compliance.ControlRepository`'s doc comment) -- a
deployment does not fork its own copy of "which version is currently
GA" per tenant, any more than it forks its own copy of the compliance
control catalogue per tenant.

Accordingly:

- `garelease_candidates` and `garelease_releases`
  (`packages/persistence/migrations/000046_create_garelease.up.sql`)
  carry no `tenant_id` column.
- Neither table gets a companion `enable_rls` migration -- there is no
  per-tenant row to isolate.
- `ReleaseCandidateRepository`/`ReleaseRepository` methods take no
  `tenantID` parameter, and their Postgres implementations
  (`PostgresReleaseCandidateRepository`/`PostgresReleaseRepository`)
  are never wrapped by `packages/tenancy.WithTenantScope`.

The `ReleaseReadiness` snapshot a candidate was frozen against is
stored inline (as `JSONB`) on the candidate row itself, rather than in
a separate readiness-check-history table: "the readiness snapshot it
was frozen against" is explicitly part of a `ReleaseCandidate` per this
phase's design brief, and a candidate's readiness never changes after
freezing (`CutRelease` re-reads the stored snapshot; it never
recomputes one), so there is no independent history to track beyond
what each candidate row already carries.

### The `AuditSink` seam: platform-global data meets a tenant-first audit log

`packages/auditlog.Store.Append` hard-rejects a `uuid.Nil` `TenantID`
(`ErrEmptyTenantID`) -- a real architectural seam between "some data is
legitimately platform-global" and "`packages/auditlog` was designed
tenant-first." This package resolves that seam honestly: `audit.go`'s
`platformScope` is a fixed, deterministic, non-nil UUID (derived via
`uuid.NewSHA1` over a stable namespace+name, so it is identical across
every process and restart, never randomly regenerated), distinguishable
from any real tenant ID by construction (real tenants are always minted
via `uuid.New()`). Every event this package's `AuditSink` appends uses
this fixed scope. Reading that trail back requires a platform-operator
`identity.User` explicitly provisioned with `TenantID == platformScope`
-- a deliberate, narrow escape hatch a deployment operator sets up once,
documented in `audit.go`, not a gap this package's own API papers over
silently.

## Access control

`identity.PermViewRelease` gates every read-only operation
(`CheckReadiness`, `GetReleaseCandidate`, `ListReleaseCandidates`,
`GetRelease`, `ListReleases`); `identity.PermManageRelease` gates every
mutation (`FreezeReleaseCandidate`, `CutRelease`,
`RunPostReleaseChecklist`) plus everything `PermViewRelease` gates,
following the exact `PermView<X>`/`PermManage<X>` precedent every
recent phase establishes. `RoleAdmin` holds both; `RoleAuditor` holds
only the view permission.

Unlike `packages/compliance`'s and `packages/privacy`'s identically
named `authorizeView`, this package's version returns only an `error`,
not the resolved `*identity.User` -- every operation it gates operates
on platform-global data with no tenant to match against, so there is
nothing further to do with the actor's identity once the permission
check itself has passed.

## What is explicitly reused, not duplicated

See `doc.go`'s "What is explicitly reused, not duplicated" section for
the full list.

## What GA does NOT mean

See `doc.go`'s "What GA does NOT mean" section. In short: GA hardening
verifies the platform's non-binding-analysis guarantee holds -- it does
not, and structurally cannot, relax it. There is no "GA mode" flag
anywhere in this codebase that loosens verdict-language rejection,
disables the mandatory disclaimer, or exempts any code path from
`packages/guardrail.CanFinalize`'s fail-closed sign-off gate.
