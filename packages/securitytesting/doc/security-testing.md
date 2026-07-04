# Security testing & red team (Phase 086)

This phase validates the platform's defenses adversarially: it does
not add a new defense mechanism of its own, it proves the ones already
shipped actually hold when someone tries to break them. It composes
seven earlier threads -- the non-binding guardrail added in Phase 057
(`packages/guardrail`), the cross-case isolation guard added in
Phase 047 (`packages/knowledgeisolation`), the role/permission model
added in Phase 006 (`packages/identity`), the per-tenant intake quotas
added in Phase 019 (`packages/intake`), the attribute-based
access-governance engine added in Phase 080
(`packages/accessgovernance`), the STRIDE threat catalogue and
prompt-injection detector added in Phase 083 (`packages/threatmodel`),
and the durable, hash-chained audit trail added in Phase 077
(`packages/auditlog`) -- into one adversarial test harness rather than
duplicating any of them.

## Goal

Add automated security regression tests, a penetration-test scope and
harness, a prompt-injection adversarial suite, data-leakage tests,
authz-bypass tests, abuse-case tests, findings tracking, and
remediation verification -- validated by the harness's own test suite,
which proves it correctly distinguishes a vulnerable fixture from a
fixed one.

## This package IS a test harness, not a wrapper around one

Every other recent phase's `Engine` orchestrates domain state
(`Control`s, `ThreatModel`s). This package's domain state is largely
the *result of running tests*: a `Scenario` is a probe of one defense;
a `Harness` runs a fixed collection of them; a `RunRecord` is what one
execution produced; a `Finding` is what gets tracked when a probe
discovers a defense did not hold. "Tests for the security suite" is
therefore not a separate concern layered on top of "the harness that
runs the suites" -- the harness's own test suite is what proves the
harness correctly distinguishes a vulnerable case from a fixed one (see
`selftest.go` / `selftest_test.go`): it ships one deliberately-
vulnerable fixture it correctly flags (`VulnerableFixtureScenario`) and
one fixed fixture it correctly passes (`FixedFixtureScenario`), running
the *identical* assertion against both. A harness that can't fail is
worthless.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/guardrail` (Phase 057) | `CheckText`/`RequireDisclaimer`: the only non-binding-disclaimer/verdict-language enforcement in this codebase | `NewRegressionSuite` (regression_suite.go): scenarios that genuinely try to defeat `CheckText` (mixed casing, mid-paragraph burial) and confirm it holds, plus a documented, currently-open known gap (see below) |
| `packages/threatmodel` (Phase 083) | `DetectInjectionAttempt`: role-override/instruction-marker/delimiter-break/data-exfiltration detection | `NewPromptInjectionSuite` (injection_suite.go): an independently-written adversarial corpus run against it, asserting every known-bad string IS flagged as its expected kind, plus a false-positive guard over legitimate legal prose |
| `packages/knowledgeisolation` (Phase 047) / `packages/tenancy` | Cross-case (`CaseScopedStore`) and cross-tenant (RLS-enforced `WithTenantScope`) isolation | `NewDataLeakageSuite` (dataleakage_suite.go): documents both mechanisms by name, then re-verifies the identical isolation invariant class against this package's own tenant-scoped fixtures (see "Why documented + scoped-fixture, not a direct import" below) |
| `packages/identity` (Phase 006) / `packages/accessgovernance` (Phase 080) | `HasPermission`; `Engine.Evaluate` (attribute-based access decisions, JIT elevation grants) | `NewAuthzBypassSuite` (authzbypass_suite.go): real black-box calls constructing wrong-tenant, expired-grant, and forged-role bypass attempts against both, asserting each is rejected |
| `packages/intake` (Phase 019) | `InMemoryQuotaChecker` (daily/concurrent per-tenant quotas); `ValidateSizeMB` | `NewAbuseCaseSuite` (abusecase_suite.go): drives real quota/oversized-payload abuse against them, plus a replay-abuse scenario against this package's own append-only `RunRecord` store |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store | `AuditSink` records every suite run, finding open/transition, and remediation verification into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewSecuritytesting`/`PermManageSecuritytesting`: the two fine-grained permissions this package's `Engine` gates every operation on |

## The harness contract (task 1's engine, task 2's runner interface)

`Scenario` (scenario.go) is the single extension point every
adversarial suite implements:

```go
type Scenario interface {
    Name() string
    Category() Category
    Run(ctx context.Context) (Result, error)
}
```

`ScenarioFunc` adapts a plain function to this interface (most
scenarios are a name/category pair plus one function body, mirroring
`http.HandlerFunc`). `Harness` runs a fixed collection of `Scenario`s
and reports one `RunRecord` per scenario -- critically, a `Scenario`
whose `Run` call returns a non-nil error produces a `RunRecord` with
`Result.Outcome = OutcomeError` rather than being silently dropped, so
a caller always gets back exactly as many records as scenarios
registered. `AllPassed` treats `OutcomeError` as not-all-passed: an
inconclusive run must never count as green.

## Penetration-test scope and rules of engagement (task 2)

`ScopeDocument` (pentest.go) names which `ScopeComponent`s an
engagement may probe, which are explicitly excluded (an explicit
exclusion always wins over an allowlist match, even if the same name
appears in both -- see `ScopeDocument.IsInScope`), and the
`RulesOfEngagement` that bound it regardless of scope (data handling,
escalation contact, prohibited actions, an optional test-time window).
`DefaultScopeDocument()` is this harness's own default engagement scope
-- every component this phase's suites actually exercise is in scope,
production tenant data and third-party services are explicitly out of
scope, and the rules of engagement state the synthetic-fixtures-only
constraint every `Scenario` in this package already satisfies
mechanically. `ScopeDocument` is enforced by construction-time review
and this package's own test suite (`pentest_test.go`), not by a
runtime interception inside `Engine.RunSuite`: every `Scenario` this
package ships already targets only in-fixture/lightweight-black-box
surfaces per the rules of engagement, so there is nothing left at
runtime to intercept for this phase's own suites.

## Prompt-injection adversarial suite (task 3)

`NewPromptInjectionSuite` (injection_suite.go) runs a real,
independently-written adversarial corpus -- role-override attempts
("ignore all prior instructions"), instruction-smuggling via fake turn
markers (`system:`, `[INST]`, ChatML tokens), delimiter-breaking
sequences (`</document>`, a `system` code fence, an "end of
instructions" banner), and data-exfiltration probes -- against
`packages/threatmodel.DetectInjectionAttempt`, asserting each is
flagged as its expected `FindingKind`. A companion
`legitimateLegalCorpus` of ordinary case-document prose proves the
detector does not false-positive on legitimate filings that happen to
use words like "instructions" or "system". Per this phase's brief,
`packages/threatmodel` had already landed (Phase 083) by the time this
suite was built, so this reuses its detector rather than shipping a
second, competing implementation.

## Data-leakage tests (task 4)

`packages/knowledgeisolation.CaseScopedStore` (cross-case) and
`packages/tenancy.WithTenantScope` (cross-tenant, RLS-enforced) are the
real isolation guarantees this platform ships. Both sit at the bottom
of a dependency chain considerably heavier than this module's
otherwise-lean footprint: `knowledgeisolation` pulls in
`packages/graph`, `packages/irac`, and `packages/embedding` (which
itself pulls the vector/provider stack); `tenancy`'s own guarantee is
exercised through a live Postgres connection with RLS enabled, not an
in-memory fixture -- exactly the Docker/testcontainers dependency this
phase's brief says to skip.

### Why documented + scoped-fixture, not a direct import

Importing either package just to make one black-box call would
roughly double this module's dependency footprint for no proportionate
gain. `NewDataLeakageSuite` (dataleakage_suite.go) instead pairs a
written description of the real guarantee (naming the exact mechanism
and the exact rejection behavior it must exhibit) with a concrete,
executable assertion against this package's *own* tenant-scoped
in-memory repositories, which implement the identical
"reject-don't-filter, `requireMatchingTenant` guard" pattern
`packages/knowledgeisolation.CaseScopedStore` and
`packages/tenancy.WithTenantScope` both establish. This is a real,
executable assertion of the same invariant class -- not a vacuous
`assert true` -- scoped to this package's own fixtures rather than
wiring the heavier cross-package call.

## Authz bypass tests (task 5)

`packages/accessgovernance`'s own dependency footprint is as lean as
`packages/compliance`'s or `packages/threatmodel`'s (`auditlog`,
`identity`, `persistence`, `tenancy` -- all already dependencies of
this module), so `NewAuthzBypassSuite` (authzbypass_suite.go) drives
real, lightweight black-box calls against its public `Engine.Evaluate`
and against `packages/identity.HasPermission`:

- **Wrong tenant, `HasPermission`**: proves `identity.HasPermission` is
  a pure role→permission lookup with no tenant notion at all, and that
  neither an unprivileged real role nor a forged/unknown role string
  ever resolves to a permission it shouldn't.
- **Expired grant still denied**: inserts an already-expired
  `accessgovernance.Grant` directly into the repository (bypassing
  `Engine.Elevate`'s own validity checks -- what an attacker who
  compromised the grant store would attempt) and confirms `Evaluate`
  still resolves to `EffectDeny`.
- **Wrong tenant, `Evaluate`**: authenticates an actor against tenant A
  but issues a `Request` scoped to tenant B, confirming `Evaluate`
  returns `ErrCrossTenantAccess` rather than a `Decision` of either
  effect.
- **No policy or grant fails closed**: confirms a tenant with zero
  `Policy`/`Grant` records on file resolves every `Evaluate` call to
  `EffectDeny`, not `EffectAllow`.

## Abuse-case tests (task 6)

`NewAbuseCaseSuite` (abusecase_suite.go) picks three concrete, real,
testable abuse cases relevant to this platform:

- **Intake daily/concurrent quota abuse**: drives
  `packages/intake.InMemoryQuotaChecker` past its configured
  `MaxDailyUploadsPerTenant` and `MaxConcurrentPerTenant` limits for a
  single tenant, confirming the over-limit request is rejected with
  `ErrQuotaExceeded` and that `RecordComplete` correctly frees a
  concurrent slot.
- **Oversized-payload abuse**: confirms `packages/intake.ValidateSizeMB`
  rejects a payload well past a configured ceiling while still
  accepting one exactly at it (a check that is only useful if it is
  not also *too* strict).
- **`RunRecord` replay**: re-submits an already-persisted
  `RunRecord.ID` with a mutated `Detail`, confirming
  `InMemoryRunRecordRepository.Create` rejects it with
  `ErrDuplicateRunRecord` and leaves the original record untouched --
  this package's own append-only persistence guarantee, backed at the
  database layer by a unique index (see "Storage" below).

## Findings tracking and remediation verification (tasks 7-8)

`Finding` (finding.go) mirrors, conceptually, the shape a sibling
phase's `packages/vulnmanagement` is expected to define for its own
(broader, dependency-scan-sourced) findings -- this package does not
import `packages/vulnmanagement`; the shape is deliberately parallel,
not shared code. `FindingStatus` is a state machine
(`FindingOpen` → `FindingTriaged` → `FindingRemediationPending` →
`FindingVerifiedFixed`, with an `FindingRiskAccepted` escape hatch
requiring a justification), mirroring
`packages/threatmodel.MitigationStatus`'s and
`packages/privacy.SARStatus`'s allowed-transitions-map shape.

`FindingVerifiedFixed` is reachable *only* through
`Engine.VerifyRemediation`, never through the general-purpose
`Engine.TransitionFinding` -- `TransitionFinding` explicitly rejects
`to == FindingVerifiedFixed` regardless of what the transitions map
says. `VerifyRemediation` re-runs the `Finding`'s `SourceScenario`
through the `Harness` and flips `Status` to `FindingVerifiedFixed`
*only* if that re-run's `Result.Outcome` is `OutcomePassed`; a
still-failing re-run leaves `Status` completely unchanged (still
`FindingRemediationPending`) and returns `ErrRemediationNotVerified`.
"Verified fixed" therefore always means an actual passing re-run
happened, never a human's unverified assertion -- see
`TestEngine_VerifyRemediation_OnlyFlipsWhenReRunPasses` in
`engine_test.go` for the definitive proof (a single togglable
`Scenario`, still-failing re-run does not flip status, then-fixed
re-run does).

## Storage

Two tables, both tenant-scoped with Row-Level Security enforced (see
`packages/persistence/migrations/000032_create_securitytesting.up.sql`
and `000033_enable_rls_securitytesting.up.sql`):

- **`securitytesting_run_records`**: append-only. A unique index on
  `id` backs `ErrDuplicateRunRecord` at the database layer, the same
  guarantee `InMemoryRunRecordRepository.Create` enforces in-process
  (verified by `ScenarioAuditReplayRejected` and
  `TestInMemoryRunRecordRepository_RejectsReplay`).
- **`securitytesting_findings`**: mutable. `source_run_id`
  foreign-keys into `securitytesting_run_records`, so "what exactly did
  the scenario observe when this finding was opened" is always
  answerable without re-running anything. `status` transitions via
  `Engine.TransitionFinding` / `VerifyRemediation`.

Both follow the usual three-layer repository pattern
(`FindingRepository`/`RunRecordRepository` interfaces,
`InMemory*`/`Postgres*`/`TenantScoped*` implementations) established by
`packages/compliance` and `packages/privacy`.

## Known gaps this suite found

Building this suite's adversarial regression coverage
(`regression_suite.go`) surfaced a real, narrow gap in
`packages/irac.ContainsVerdictLanguage` (which
`packages/guardrail.CheckText` wraps): its verdict/directive wordlist
match is a plain, case-insensitive `strings.Contains` substring check.
Mixed-casing and mid-paragraph-burial evasion attempts are correctly
caught (`ScenarioGuardrailNotDefeatedByObfuscation`), but collapsing or
expanding the internal whitespace of an exact wordlist phrase (e.g.
`"shall pay"` → `"shall   pay"`) defeats the exact-substring match
entirely, since a literal match requires an identical run of
characters.

`ScenarioGuardrailWhitespaceObfuscationKnownGap` (regression_suite.go)
documents and asserts this gap precisely (verified currently-failing
by `TestScenarioGuardrailWhitespaceObfuscationKnownGap_CurrentlyFails`
in regression_suite_test.go), and is deliberately excluded from
`NewRegressionSuite`/`SeedHarness`'s default "everything must pass"
composition -- a suite whose default run is permanently red on a
known, tracked, un-remediated gap in a shared, foundational function
(`packages/irac.ContainsVerdictLanguage` is also used by
`packages/reasoningeval`, `packages/synthesisagent`, and
`packages/uncertainty`) stops being a useful signal for anything else
regressing. Patching `packages/irac`'s matching semantics is out of
scope for this phase -- the blast radius belongs to a dedicated,
reviewed change against that package, not a side effect of adding a
test harness. Once that gap is closed upstream, re-running
`ScenarioGuardrailWhitespaceObfuscationKnownGap` through
`Engine.VerifyRemediation` is exactly the mechanism for confirming a
`Finding` opened against it.

## What is explicitly reused, not duplicated

- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink` -- exactly the composition
  pattern `packages/compliance`'s and `packages/threatmodel`'s own
  `AuditSink` established.
- `identity.Role`/`identity.Permission`/`identity.HasPermission`
  (Phase 006) remain the coarse RBAC gate every `Engine` method calls
  through `authorizeManage`/`authorizeView` -- and `HasPermission`
  itself is one of this suite's own authz-bypass probe targets, not
  reimplemented.
- `packages/guardrail.CheckText`/`RequireDisclaimer` (Phase 057) remain
  the only non-binding-disclaimer/verdict-language enforcement in this
  codebase; this package's regression suite calls through them
  adversarially rather than reimplementing any check.
- `packages/threatmodel.DetectInjectionAttempt` (Phase 083) remains the
  only prompt-injection detector in this codebase; this package's
  prompt-injection suite calls through it with an independently-written
  corpus rather than shipping a second, competing detector.
- `packages/accessgovernance.Engine.Evaluate` (Phase 080) and
  `packages/intake`'s `QuotaChecker`/`ValidateSizeMB` (Phase 019)
  remain the only attribute-based access-governance engine and
  per-tenant intake quota enforcement in this codebase; this package's
  authz-bypass and abuse-case suites drive real, lightweight black-box
  calls against their public constructors rather than reimplementing
  policy evaluation or quota accounting.
- `packages/knowledgeisolation` (Phase 047) and `packages/tenancy`
  remain the only cross-case and cross-tenant isolation enforcement in
  this codebase; this package's data-leakage suite documents both by
  name and mechanism rather than importing either's heavier dependency
  chain.
