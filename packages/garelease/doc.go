// Package garelease is Phase 100: the FINAL phase of Verdex's
// 100-phase build. This is the capstone -- it aggregates readiness
// signals from across the entire platform into one release-readiness
// gate, and models the GA (general availability) cut itself.
//
// # The eight parts of the 100-phase plan, in one paragraph each
//
// This platform was built in eight parts across 100 phases:
//
//  1. Foundation & Provisioning -- repository bootstrap, configuration
//     framework, observability, persistence, and multi-tenancy
//     (packages/config, packages/observability, packages/persistence,
//     packages/tenancy), plus the first-run deployment-profile wizard
//     (packages/setup) and jurisdiction loading (packages/jurisdiction).
//  2. Provider Abstraction -- the LLMProvider interface no phase may
//     bypass (packages/provider), routing (packages/router), prompt
//     templates (packages/prompts), and embeddings (packages/embedding),
//     so no phase ever hardcodes a specific model vendor.
//  3. Ingestion & Transcribe-and-Discard -- speech-to-text and OCR
//     extraction (packages/stt, packages/ocr), multilingual
//     normalization and segmentation (packages/multilingual,
//     packages/segmentation), PII handling (packages/pii), and the
//     ingestion orchestrator that discards binary source material
//     immediately after extracting provenance-hashed text
//     (packages/ingestion, packages/provenance).
//  4. IRAC Reasoning Tree & Knowledge Layer -- the Issue/Rule/Application/
//     Conclusion reasoning-tree model and its own non-binding guardrail
//     primitives (packages/irac), the knowledge graph and statute/
//     precedent corpus (packages/graph, packages/statute,
//     packages/precedent, packages/ontology), tree assembly/validation/
//     indexing (packages/treeassembly, packages/treevalidation,
//     packages/treeindex), and hybrid/adaptive retrieval plus citation
//     (packages/hybridretrieval, packages/adaptiveretrieval,
//     packages/citation).
//  5. Reasoning & Adversarial Synthesis -- the multi-agent reasoning
//     framework (packages/agentframework, packages/issueagent,
//     packages/firstpartyagent, packages/secondpartyagent,
//     packages/synthesisagent), evidence weighing and law application
//     (packages/evidenceweighing, packages/lawapplication), uncertainty
//     quantification (packages/uncertainty), and -- critically -- the
//     project-wide non-binding-analysis guardrail every reasoning
//     output must pass through (packages/guardrail, Phase 057), plus
//     reasoning orchestration, tracing, grounding, and evaluation
//     (packages/reasoningorchestration, packages/reasoningtrace,
//     packages/grounding, packages/reasoningeval).
//  6. Case Management & Workflow -- the case lifecycle state machine and
//     human sign-off gate (packages/caselifecycle, packages/signoff),
//     search, annotations, versioning, notifications, and report export
//     (packages/casesearch, packages/annotations, packages/caseversioning,
//     packages/notifications, packages/reportexport), and analytics
//     (packages/analytics).
//  7. Security, Compliance & Sovereignty -- encryption and key
//     management (packages/encryption, packages/keymanagement), the
//     durable hash-chained audit trail (packages/auditlog, Phase 077),
//     data residency and air-gapped deployment (packages/dataresidency,
//     packages/airgapped), fine-grained access governance
//     (packages/accessgovernance), data-subject rights and compliance
//     mapping (packages/privacy, packages/compliance), threat modeling,
//     adversarial security testing, and vulnerability management
//     (packages/threatmodel, packages/securitytesting,
//     packages/vulnmanagement).
//  8. Integration, Operations & Hardening -- backup/disaster recovery
//     (packages/backupdr), external system integration and bulk import
//     (packages/integration, packages/bulkimport), localization
//     (packages/localization), performance budgeting and benchmarking
//     (packages/perf, Phase 091), corpus updates, scalability, and
//     reliability (packages/corpusupdater, packages/scalability,
//     packages/reliability), CI/CD hardening and infrastructure-as-code
//     (packages/cicdgate, packages/iac), alerting, documentation site,
//     full-journey E2E testing, and the controlled pilot deployment
//     (packages/alerting, packages/docsite, packages/e2e,
//     packages/pilot, Phase 099).
//
// # This package is where every signal converges
//
// packages/garelease is not a ninth part of the plan -- it is the
// capstone that sits on top of all eight, the way
// packages/accessgovernance (Phase 080) drew together every
// RBAC/tenancy/audit thread from the twenty-phase run that preceded it.
// Every governance, quality, and security signal the platform has
// produced across its entire history converges here into one go/no-go
// release-readiness gate:
//
//   - packages/vulnmanagement.Finding and packages/securitytesting.Finding
//     (Phase 084, Phase 086) -- dependency/SAST/container/license
//     findings and adversarial-probe findings -- converge into
//     DimensionCriticalFindings.
//   - packages/compliance.GapAnalysisReport (Phase 082) -- the final
//     compliance-control gap analysis -- converges into
//     DimensionComplianceGaps.
//   - packages/perf.Verdict (Phase 091) -- budget-vs-measurement
//     evaluation -- converges into DimensionPerfBudget.
//   - packages/e2e's full-journey suite (Phase 097), run as a CI gate --
//     converges into DimensionE2ERegression via the typed E2EResult
//     input (see input.go's doc comment for why this package does not
//     import packages/e2e directly).
//   - packages/guardrail's own verdict-language/label/disclaimer checks
//     (Phase 057) -- re-verified live, with a harness that can fail --
//     converge into DimensionGuardrailIntegrity (guardrail.go).
//   - packages/auditlog.Store's queryability and hash-chain integrity
//     (Phase 077) -- re-verified live against a Store-shaped dependency
//     -- converge into DimensionAuditCompleteness (audittrail.go).
//
// Engine.CheckReadiness aggregates all six dimensions into one
// ReleaseReadiness{Checks, Ready}: Ready is true only if every
// dimension passes. Fail closed -- exactly the discipline
// packages/guardrail.CanFinalize established for human sign-off state,
// applied here to the entire platform's release gate.
//
// # What is new here (mapped to Phase 100's ten tasks)
//
//   - DimensionName / ReadinessCheck / ReleaseReadiness / ReadinessInput
//     / Engine.CheckReadiness (types.go, input.go, readiness.go): tasks
//     1, 2, 3, 5 converge here as described above.
//   - Engine.VerifyGuardrails (guardrail.go): task 6's guardrail half --
//     a REAL harness calling packages/guardrail.CheckText,
//     RequireDisclaimer, HasDisclaimer, and ValidateLabeled against a
//     deliberately verdict-worded fixture (must fail) and a
//     properly-labeled draft-analysis fixture (must pass), mirroring
//     packages/pilot.ValidateNonBindingCompliance's and
//     packages/securitytesting/selftest.go's identical
//     vulnerable-fixture/fixed-fixture "harness that can fail" pattern.
//   - Engine.VerifyAuditTrail (audittrail.go): task 6's audit half -- a
//     real structural check (queryable + hash-chain intact) against an
//     AuditTrailStore-shaped interface a real *packages/auditlog.Store
//     satisfies directly.
//   - ReleaseCandidate / Engine.FreezeReleaseCandidate (types.go,
//     candidate.go): task 4 -- freezes a candidate ONLY when the
//     supplied ReleaseReadiness is Ready (ErrReadinessNotReady
//     otherwise, tested explicitly). Does not tag anything in git --
//     see "What this package does NOT do" below.
//   - Release / Engine.CutRelease (types.go, candidate.go): task 8 --
//     promotes a frozen, still-Ready ReleaseCandidate to a Release.
//     Also does not tag anything in git.
//   - BuildReleaseArtifactAttestation (artifact.go): task 7 -- real
//     composition with packages/cicdgate.ReleaseArtifact/
//     BuildAttestation/Verify (Phase 095); this package adds no
//     artifact/attestation validation logic of its own.
//   - PostReleaseChecklist / PostReleaseCheck / Engine.RunPostReleaseChecklist
//     (postrelease.go): task 9 -- mirrors packages/iac.DeploymentVerification's
//     Checklist/Check/RunDeploymentVerification shape (Phase 094)
//     exactly, applied to a Release instead of a deployment. The
//     guardrail-smoke-test check is real (backed directly by
//     Engine.VerifyGuardrails); the health-endpoint and
//     migration-version checks are caller-supplied Run functions, for
//     the same reason packages/iac's own Check.Run is always
//     caller-supplied (this package has no business dialing a live
//     endpoint or opening a live database connection).
//   - BuildReleaseNotes (releasenotes.go) and CHANGELOG.md: task 10 --
//     real composition with packages/cicdgate.GenerateReleaseNotes
//     (Phase 095), plus a human-authored CHANGELOG.md summarizing this
//     100-phase build at the eight-part level (not every commit).
//   - identity.PermViewRelease / identity.PermManageRelease
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewPilot/PermManagePilot precedent from Phase 099.
//   - AuditSink (audit.go): records every readiness check, candidate
//     freeze, release cut, and post-release checklist run into
//     packages/auditlog.Store -- the same durable, hash-chained sink
//     the rest of the platform already writes to and queries. No
//     second audit table.
//
// # What this package does NOT do
//
// FreezeReleaseCandidate and CutRelease model the SOFTWARE-SIDE record
// only. Neither this package, nor any function in it, ever shells out
// to `git tag`, `git push --tags`, or any other git plumbing. The
// actual git tag against a frozen candidate's CommitSHA is a separate,
// deliberate step a human or an orchestrating CI job performs AFTER
// this package's records (and the pull request introducing them) have
// merged -- freezing/cutting a software record and tagging a
// repository are two different actions with two different blast radii,
// and conflating them into one Go function call would make the
// irreversible git-history action (a pushed tag) an unreviewable side
// effect of calling a library function. See doc/ga-release.md for the
// full boundary discussion.
//
// Likewise, this package does not re-invoke packages/e2e's full-journey
// suite, does not dial any live HTTP endpoint, does not open any live
// database connection to check a migration version, and does not
// implement a second guardrail/audit-trail mechanism -- every one of
// those is a real, existing platform capability this package composes
// with by reference or by direct (lightweight) import, never
// duplicated.
//
// # Persistence: ReleaseCandidate/Release are platform-global, not
// tenant-scoped
//
// A software release is not a per-tenant concept: one ReleaseCandidate
// and one Release are shared by every tenant of a given deployment, the
// same way packages/compliance.Control's catalogue is shared reference
// data rather than forked per tenant (see
// packages/compliance.ControlRepository's doc comment, which this
// package's ReleaseCandidateRepository/ReleaseRepository mirror
// exactly). Accordingly, garelease_candidates and garelease_releases
// (packages/persistence/migrations/000046_create_garelease.up.sql)
// carry no tenant_id column and get no companion enable_rls migration
// -- there is no per-tenant row to isolate. The ReleaseReadiness
// snapshot a candidate was frozen against is stored inline (as JSONB)
// on the candidate row itself, rather than in a separate
// readiness-check-history table: "the readiness snapshot it was frozen
// against" is explicitly part of a ReleaseCandidate per this phase's
// design brief, and a candidate's readiness never changes after
// freezing (CutRelease re-reads the stored snapshot, it does not
// recompute one), so there is no independent history to track beyond
// what each candidate row already carries.
//
// This package's AuditSink still needs *some* TenantID to satisfy
// packages/auditlog.Store.Append's ErrEmptyTenantID guard (a real
// architectural seam between "some data is legitimately
// platform-global" and "packages/auditlog was designed tenant-first");
// see audit.go's platformScope doc comment for how that seam is
// resolved honestly (a fixed, deterministic, non-tenant UUID, not a
// workaround that silently treats platform data as belonging to
// whichever tenant happened to make the call).
//
// # What is explicitly reused, not duplicated
//
//   - packages/vulnmanagement.Finding/Status,
//     packages/securitytesting.Finding/FindingStatus,
//     packages/compliance.GapAnalysisReport, and packages/perf.Verdict
//     remain the only finding-tracking, compliance-gap-analysis, and
//     performance-budget-evaluation machinery in this codebase; this
//     package imports and reads them directly as ReadinessInput fields,
//     never reimplementing any of their own evaluation logic.
//   - packages/guardrail.CheckText/RequireDisclaimer/HasDisclaimer/
//     ValidateLabeled (Phase 057) remain the only non-binding-guardrail
//     enforcement in this codebase; VerifyGuardrails calls them
//     directly against this package's own fixtures rather than
//     reimplementing any verdict-language or label check.
//   - packages/auditlog.Store (Phase 077) remains the only durable,
//     hash-chained audit sink in this codebase; both AuditSink (the
//     write side) and VerifyAuditTrail (the read/verify side) compose
//     with it directly, never a second audit table or a parallel
//     hash-chain implementation.
//   - packages/cicdgate.ReleaseArtifact/BuildAttestation/Verify/
//     GenerateReleaseNotes (Phase 095) remain the only signed-build,
//     attestation, and release-notes-generation machinery in this
//     codebase; artifact.go and releasenotes.go call them directly.
//   - packages/iac.Checklist/Check/DeploymentVerificationReport (Phase
//     094) is the structural precedent PostReleaseChecklist/
//     PostReleaseCheck/PostReleaseReport follow, not a dependency --
//     this package does not import packages/iac, mirroring
//     packages/cicdgate's own "at the time this was written,
//     packages/iac did not yet exist" precedent of naming a shape by
//     reference rather than forcing an import.
//   - identity.Role/identity.Permission/identity.HasPermission (Phase
//     006) remain the coarse RBAC gate every Engine method calls
//     through authorizeManage/authorizeView before doing anything
//     release-readiness-specific.
//
// # What GA does NOT mean
//
// Cutting a GA release through this package's CutRelease does not
// relax, waive, or supersede the platform's single non-negotiable
// guarantee: every reasoning output this platform produces remains a
// non-binding DRAFT ANALYSIS, never a verdict, ruling, or judgment, and
// never a substitute for a qualified legal professional or a court.
// packages/guardrail's CheckText/RequireLabel/RequireDisclaimer
// enforcement (Phase 057) is completely unaffected by GA status --
// there is no "GA mode" flag anywhere in this codebase that loosens
// verdict-language rejection, disables the mandatory disclaimer, or
// exempts any code path from CanFinalize's fail-closed sign-off gate.
// If anything, GA hardening tightens the platform's confidence that
// this guarantee holds (DimensionGuardrailIntegrity re-verifies it
// live, as one of six things that must ALL pass before Ready is ever
// true) -- it does not, and structurally cannot, weaken it.
//
// GA also does not mean: zero remaining findings of any severity (only
// critical/high findings gate release -- see DimensionCriticalFindings);
// perfect performance under unbounded load (perf budgets are targets
// for this platform's benchmarked operations, not a guarantee against
// every conceivable workload); or that this package's own readiness
// gate is a substitute for human judgment about whether to actually
// release -- Engine.CheckReadiness reports Ready, it does not, and
// cannot, decide to release on anyone's behalf. A human still reviews
// the ReleaseReadiness snapshot, still decides to call
// FreezeReleaseCandidate and CutRelease, and a human (or a
// deliberately-triggered CI job) still performs the actual `git tag`
// this package never touches.
//
// See doc/ga-release.md for the full write-up, including the
// composition table and worked examples of CheckReadiness,
// FreezeReleaseCandidate, and CutRelease.
package garelease
