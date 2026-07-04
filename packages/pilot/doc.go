// Package pilot is Phase 099: a controlled pilot deployment with real
// users and a structured feedback loop, the second-to-last phase of
// this platform's 100-phase build. It composes the pilot-deployment
// provisioning concepts already established by Phase 008
// (packages/setup) and Phase 007 (packages/jurisdiction), runs
// supervised cases against real Phase 063 (packages/caselifecycle)
// Case records, collects structured expert feedback that extends
// Phase 062's (packages/reasoningeval) DimensionName/QualityScore
// shape, triages surfaced issues using Phase 084's
// (packages/vulnmanagement) Finding/status-machine pattern, validates
// the non-binding workflow by calling straight into Phase 057's
// (packages/guardrail) real verdict-language/label checks, and
// captures a final report mirroring Phase 080's
// (packages/accessgovernance) Certify/ExportReport shape -- composing
// all of it, rather than duplicating any of it.
//
// # What is new here
//
//   - PilotDeployment / DeploymentStatus (types.go): a tenant-scoped
//     controlled pilot referencing a jurisdiction by JurisdictionCode
//     string (the same reference convention already used by
//     packages/corpusupdater.CorpusUpdateJob, packages/issueagent, and
//     packages/reasoningeval -- see "Storage" below for why this
//     package does not import packages/jurisdiction directly),
//     tracking a Provisioning -> CorpusOnboarding -> Active ->
//     Concluded lifecycle (tasks 1-2).
//   - PilotCase (types.go): a supervised case run within a
//     PilotDeployment, referencing a real
//     packages/caselifecycle.Case by ID only -- never by value or
//     import -- mirroring packages/accessgovernance.CaseGrant's
//     CaseID-reference convention exactly (task 3).
//   - FeedbackEntry / DimensionName / DimensionRating / TrustRating
//     (feedback.go): structured expert feedback on a single
//     PilotCase -- reviewer identity, per-dimension [0,1] ratings, a
//     discrete 1-5 TrustRating, and free-text comments (task 4). See
//     "Why FeedbackEntry mirrors reasoningeval.DimensionName instead
//     of importing it" below.
//   - QualitySummary / Engine.AggregateQuality / BuildQualitySummary
//     (aggregate.go): a real aggregation over collected FeedbackEntry
//     records plus caller-supplied automated
//     packages/reasoningeval.QualityScore-shaped values, resolved
//     through the small QualityScoreLike interface rather than a
//     direct reasoningeval import (task 5).
//   - PilotFinding / Priority / FindingStatus (finding.go): an issue
//     surfaced during the pilot, sourced from one or more
//     FeedbackEntry IDs, mirroring
//     packages/vulnmanagement.Finding's status-machine shape (Open ->
//     Triaged -> InProgress -> Resolved/WontFix) with a Priority
//     instead of Severity (task 6).
//   - RefinementRecord (refinement.go): what was changed in response
//     to a PilotFinding, a VerifiedFixed bool + verification note, and
//     the real precondition -- enforced in engine.go against the
//     finding's live stored Status, not merely documented -- that a
//     refinement may only reference an already-triaged finding (task
//     7).
//   - ComplianceResult / Engine.ValidateNonBindingCompliance
//     (compliance.go): calls straight into packages/guardrail's own
//     real checks (CheckText, HasDisclaimer) against a pilot case's
//     opinion text, proven by compliance_test.go's harness-that-can-
//     fail tests: a deliberately verdict-worded fixture is flagged, a
//     properly-labeled draft-analysis fixture passes (task 8).
//   - Report / Period / ExportFormat / Engine.CaptureReport /
//     BuildReport / ExportReport (report.go): aggregates deployment
//     status, case count, quality summary, findings by priority,
//     refinements applied/verified, and non-binding-compliance rate,
//     mirroring packages/accessgovernance.Certify/ExportReport's
//     CSV/JSON export shape exactly (task 9).
//   - AuditSink (audit.go): records every deployment status change,
//     finding triage, and report capture into
//     packages/auditlog.Store -- the same durable, hash-chained sink
//     the rest of the platform already writes to and queries. No
//     second audit table.
//   - identity.PermViewPilot / identity.PermManagePilot
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewAlerting/PermManageAlerting precedent from Phase
//     096 -- genuinely missing before this phase.
//
// # Why FeedbackEntry mirrors reasoningeval.DimensionName instead of
// importing it
//
// packages/reasoningeval's own go.mod requires
// packages/grounding, packages/synthesisagent, packages/treeassembly,
// packages/vectorindex, packages/hybridretrieval, the neo4j Go
// driver, and a dozen more reasoning-stack packages -- the full
// dependency tree behind DimensionScorer/Rubric/Score, none of which
// this phase's feedback-collection surface needs. Importing
// reasoningeval here purely to reuse a three-field
// DimensionName/QualityScore shape would drag every one of those
// packages into packages/pilot's own module graph. Instead,
// DimensionName and its three shared constants (grounding, citation,
// coherence) are mirrored explicitly -- kept string-identical by
// convention -- and QualitySummary composes automated scores through
// the small QualityScoreLike resolver interface (aggregate.go),
// mirroring reasoningeval's own OpinionLike/GroundingReportLike
// decoupling precedent. A caller holding a real
// packages/reasoningeval.QualityScore adapts it via
// ReasoningEvalQualityScoreAdapter. Automated and human-reviewer
// scores are aggregated side by side in QualitySummary, never merged
// into a single number -- exactly the guarantee
// packages/reasoningeval.ExpertReview's own doc comment establishes.
//
// # Storage
//
// PilotDeployment, PilotCase, FeedbackEntry, PilotFinding, and
// RefinementRecord are all durable, tenant-scoped, RLS-protected
// Postgres tables (packages/persistence/migrations/000044_create_pilot
// and 000045_enable_rls_pilot) -- unlike packages/compliance's shared
// compliance_controls catalogue, every type this phase introduces is
// genuinely per-tenant, so every table carries a tenant_id column and
// gets a tenant_isolation RLS policy, mirroring
// packages/alerting's identical structure. Each type gets the usual
// three-layer repository: an interface (repository.go), an in-memory
// implementation for tests (inmemory_repository.go), and a
// PostgreSQL-backed pair -- Postgres*Repository (postgres_repository.go)
// plus TenantScopedDeploymentRepository et al.
// (tenant_scoped_repository.go), which wraps
// packages/tenancy.WithTenantScope so Row-Level Security enforces
// tenant isolation at the database layer in addition to the
// application-level requireMatchingTenant guard every repository
// method already applies.
//
// # What is explicitly reused, not duplicated
//
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/compliance's and packages/vulnmanagement's own
//     AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything pilot-specific.
//   - packages/guardrail.CheckText / HasDisclaimer (Phase 057) remain
//     the only non-binding-disclaimer and verdict-language enforcement
//     mechanisms in this codebase; ValidateNonBindingCompliance calls
//     them directly rather than reimplementing any part of that check.
//   - packages/caselifecycle.Case (Phase 063) remains the only case
//     record in this codebase; PilotCase references it by ID only,
//     mirroring packages/accessgovernance.CaseGrant's identical
//     convention -- this package does not import
//     packages/caselifecycle's Go types (caselifecycle stays an
//     indirect module dependency, resolved only because
//     packages/vulnmanagement's own go.mod graph already needs it,
//     exactly as packages/vulnmanagement itself keeps it indirect).
//   - packages/reasoningeval's DimensionName/QualityScore/ExpertReview
//     shape is the reference FeedbackEntry/QualitySummary follow, not
//     a dependency -- see "Why FeedbackEntry mirrors
//     reasoningeval.DimensionName instead of importing it" above.
//   - packages/vulnmanagement's Finding/TriageDecision/status-machine
//     shape is the reference PilotFinding/RefinementRecord follow, not
//     a dependency -- this package does not import
//     packages/vulnmanagement.
//   - packages/accessgovernance's Certify/ExportReport aggregation
//     shape is the reference Report/CaptureReport/ExportReport follow,
//     not a dependency -- this package does not import
//     packages/accessgovernance.
//   - packages/setup's deployment-provisioning wizard and
//     packages/jurisdiction's onboarding/lookup machinery are the
//     conceptual precedents PilotDeployment's
//     Provisioning/CorpusOnboarding states name, not dependencies --
//     provisioning infrastructure and onboarding a jurisdiction's
//     corpus are Phase 008/007 concerns this package does not
//     duplicate or import; PilotDeployment only tracks that a given
//     pilot has reached those stages.
//
// See doc/pilot.md for the full write-up.
package pilot
