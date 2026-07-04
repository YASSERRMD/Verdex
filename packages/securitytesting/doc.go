// Package securitytesting is Phase 086: an adversarial validation
// harness proving this platform's defenses actually hold under attack,
// rather than merely asserting that they exist. It draws on the
// non-binding guardrail added in Phase 057 (packages/guardrail), the
// cross-case isolation guard added in Phase 047
// (packages/knowledgeisolation), the role/permission model added in
// Phase 006 (packages/identity), the per-tenant intake quotas added in
// Phase 019 (packages/intake), the attribute-based access-governance
// engine added in Phase 080 (packages/accessgovernance), the STRIDE
// threat catalogue and prompt-injection detector added in Phase 083
// (packages/threatmodel), and the durable, hash-chained audit trail
// added in Phase 077 (packages/auditlog), composing them into one
// adversarial test harness rather than duplicating any of them.
//
// # This package IS a test harness, not a wrapper around one
//
// Every other recent phase's Engine orchestrates domain state
// (Controls, ThreatModels, Findings-as-in-vulnerability-management).
// This package's "domain state" is largely the RESULT of running
// tests: a Scenario (scenario.go) is a probe of one defense; a Harness
// runs a fixed collection of them; a RunRecord is what one execution
// produced; a Finding (finding.go) is what gets tracked when a probe
// discovers a defense did not hold. "Tests for the security suite"
// (task 9) is therefore not a separate concern layered on top of "the
// harness that runs the suites" (tasks 1-6) -- the harness's own test
// suite is what proves the harness correctly distinguishes a
// vulnerable case from a fixed one. See selftest.go's
// VulnerableFixtureScenario/FixedFixtureScenario pair and
// selftest_test.go: a harness that cannot fail is worthless, so this
// package ships one deliberately-vulnerable fixture it correctly flags
// and one fixed fixture it correctly passes, using the identical
// assertion logic against both.
//
// # What is new here
//
//   - Scenario / ScenarioFunc / Harness (scenario.go): the harness
//     contract every adversarial suite below implements, plus the
//     runner that executes a fixed collection of them and reports one
//     RunRecord per scenario, never silently dropping a scenario that
//     errored (task 1's engine).
//   - NewRegressionSuite (regression_suite.go): the automated security
//     regression suite (task 1) -- scenarios that re-check, on every
//     run, that a previously-established defense (the non-binding
//     guardrail) still holds, including genuine attempts to defeat it
//     via casing and mid-paragraph-burial obfuscation. See "Known gaps
//     this suite found" below for a THIRD obfuscation technique this
//     suite's own adversarial testing discovered still evades
//     detection today.
//   - ScopeDocument / ScopeComponent / RulesOfEngagement /
//     DefaultScopeDocument (pentest.go): the penetration-test scope and
//     rules-of-engagement type (task 2) -- which named components an
//     engagement may probe, which are explicitly excluded, and the
//     operational constraints (data handling, escalation contact,
//     prohibited actions) that bound it regardless of scope. Every
//     Engine is constructed with an active ScopeDocument
//     (Engine.Scope() exposes it) that a caller or reviewer checks a
//     Scenario's target component against via ScopeDocument.IsInScope
//     before wiring that Scenario into a production Harness --
//     enforced by construction-time review and this package's own
//     test suite (see pentest_test.go), not by a runtime interception
//     inside RunSuite: every Scenario this package ships already
//     targets only in-fixture/lightweight-black-box surfaces per
//     RulesOfEngagement, so there is nothing at runtime left to
//     intercept for this phase's own suites.
//   - NewPromptInjectionSuite (injection_suite.go): the prompt-injection
//     adversarial suite (task 3) -- a real corpus of adversarial
//     strings (role-override, instruction-smuggling via fake turn
//     markers, delimiter-breaking, data-exfiltration) run against
//     packages/threatmodel.DetectInjectionAttempt, asserting each
//     known-bad string IS flagged as its expected FindingKind, plus a
//     false-positive guard proving ordinary legal prose is NOT
//     flagged.
//   - NewDataLeakageSuite (dataleakage_suite.go): the data-leakage
//     suite (task 4) -- documents the real
//     packages/knowledgeisolation (cross-case) and packages/tenancy
//     (cross-tenant, RLS-enforced) isolation guarantees by name and
//     mechanism, then re-verifies the identical
//     reject-don't-filter/requireMatchingTenant invariant class
//     against this package's own tenant-scoped in-memory
//     repositories -- a real, executable assertion scoped to this
//     package's own fixtures rather than importing
//     packages/knowledgeisolation's heavy graph/irac/embedding
//     dependency chain for a single black-box call (see
//     dataleakage_suite.go's file-level comment for the full
//     dependency-weight rationale).
//   - NewAuthzBypassSuite (authzbypass_suite.go): the authz-bypass
//     suite (task 5) -- real, lightweight black-box calls against
//     identity.HasPermission and accessgovernance.Engine.Evaluate,
//     constructing wrong-tenant, expired-grant, and forged-role bypass
//     attempts and asserting each is correctly rejected (fail-closed
//     to EffectDeny / ErrCrossTenantAccess), not merely documented.
//   - NewAbuseCaseSuite (abusecase_suite.go): the abuse-case suite
//     (task 6) -- three concrete, real, testable abuse cases: intake
//     per-tenant daily/concurrent quota abuse (against
//     packages/intake's actual QuotaChecker), oversized-payload abuse
//     (against packages/intake.ValidateSizeMB), and RunRecord replay
//     (against this package's own append-only persistence guard, see
//     ErrDuplicateRunRecord).
//   - Finding / FindingStatus / CanTransitionFinding (finding.go,
//     types.go): findings tracking (task 7) -- a state machine
//     mirroring packages/threatmodel.MitigationStatus's and
//     packages/privacy.SARStatus's allowed-transitions-map shape,
//     conceptually mirroring the Finding/triage shape a sibling
//     phase's packages/vulnmanagement is expected to define for its
//     own (broader, dependency-scan-sourced) findings -- this package
//     does not import packages/vulnmanagement.
//   - Engine.VerifyRemediation (engine.go): remediation verification
//     (task 8) -- re-runs a Finding's originating Scenario and flips
//     Status to FindingVerifiedFixed ONLY when that re-run's Result is
//     OutcomePassed; a still-failing re-run leaves Status completely
//     unchanged and returns ErrRemediationNotVerified, so "verified
//     fixed" always means an actual passing re-run happened, never a
//     human's unverified assertion.
//   - identity.PermViewSecuritytesting / PermManageSecuritytesting
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewThreatmodel/PermManageThreatmodel precedent from
//     Phase 083.
//   - AuditSink (audit.go): records every suite run, finding
//     open/transition, and remediation verification into
//     packages/auditlog.Store -- the same durable, hash-chained sink
//     the rest of the platform already writes to and queries. No
//     second audit table.
//   - FindingRepository / RunRecordRepository and their in-memory /
//     Postgres / tenant-scoped implementations (repository.go,
//     inmemory_repository.go, postgres_repository.go,
//     tenant_scoped_repository.go): the usual three-layer persistence
//     pattern, tenant-scoped with RLS enabled in
//     packages/persistence/migrations/000028_create_securitytesting.up.sql
//     and 000029_enable_rls_securitytesting.up.sql.
//
// # Storage
//
// Two tables, both tenant-scoped with Row-Level Security enforced
// (see 000029_enable_rls_securitytesting.up.sql): securitytesting_run_records
// (append-only -- a unique index on id backs ErrDuplicateRunRecord,
// see abusecase_suite.go's ScenarioAuditReplayRejected) and
// securitytesting_findings (mutable -- Status transitions via
// Engine.TransitionFinding / VerifyRemediation). A Finding's
// source_run_id foreign-keys into securitytesting_run_records, so
// "what exactly did the scenario observe when this finding was opened"
// is always answerable without re-running anything.
//
// # Known gaps this suite found
//
// Building this suite's adversarial regression coverage
// (regression_suite.go) surfaced a real, narrow gap in
// packages/irac.ContainsVerdictLanguage (which
// packages/guardrail.CheckText wraps): its verdict/directive wordlist
// match is a plain, case-insensitive strings.Contains substring check.
// Mixed-casing and mid-paragraph-burial evasion attempts are correctly
// caught (verified in
// ScenarioGuardrailNotDefeatedByObfuscation), but collapsing or
// expanding the internal whitespace of an exact wordlist phrase (e.g.
// "shall pay" -> "shall   pay") defeats the exact-substring match
// entirely, since a literal match requires an identical run of
// characters. ScenarioGuardrailWhitespaceObfuscationKnownGap
// (regression_suite.go) documents and asserts this gap precisely, and
// is deliberately excluded from NewRegressionSuite/SeedHarness's
// default "everything must pass" composition -- a shared, foundational
// function used by packages/guardrail, packages/reasoningeval,
// packages/synthesisagent, and packages/uncertainty is out of scope
// for this phase to patch directly (the blast radius of changing its
// matching semantics belongs to a dedicated, reviewed change against
// packages/irac, not a security-testing phase's side effect). Once
// that gap is closed upstream, re-running
// ScenarioGuardrailWhitespaceObfuscationKnownGap through
// Engine.VerifyRemediation is exactly how a Finding opened against it
// would move to FindingVerifiedFixed.
//
// # What is explicitly reused, not duplicated
//
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/compliance's and packages/threatmodel's own
//     AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything security-testing-specific -- and identity.HasPermission
//     itself is one of this suite's own authz-bypass probe targets
//     (ScenarioWrongTenantHasPermissionNeverEscalates), not
//     reimplemented.
//   - packages/guardrail.CheckText / RequireDisclaimer (Phase 057)
//     remain the only non-binding-disclaimer/verdict-language
//     enforcement in this codebase; this package's regression suite
//     calls through them adversarially rather than reimplementing any
//     verdict-language or disclaimer check.
//   - packages/threatmodel.DetectInjectionAttempt (Phase 083) remains
//     the only prompt-injection detector in this codebase; this
//     package's prompt-injection suite calls through it with an
//     independently-written adversarial corpus rather than shipping a
//     second, competing detector.
//   - packages/accessgovernance.Engine.Evaluate (Phase 080) and
//     packages/intake's QuotaChecker/ValidateSizeMB (Phase 019) remain
//     the only attribute-based access-governance engine and per-tenant
//     intake quota enforcement in this codebase; this package's
//     authz-bypass and abuse-case suites drive real, lightweight
//     black-box calls against their public constructors rather than
//     reimplementing policy evaluation or quota accounting.
//   - packages/knowledgeisolation (Phase 047) and packages/tenancy
//     remain the only cross-case and cross-tenant isolation
//     enforcement in this codebase; this package's data-leakage suite
//     documents both by name and mechanism rather than importing
//     either's heavier dependency chain (see dataleakage_suite.go).
//
// See doc/security-testing.md for the full write-up, including the
// composition table and the pentest-scope model.
package securitytesting
