# Pilot deployment & feedback loop (Phase 099)

This is the second-to-last phase of this platform's 100-phase build.
It composes eight earlier threads -- pilot deployment provisioning
concepts from Phase 008 (`packages/setup`) and jurisdiction onboarding
from Phase 007 (`packages/jurisdiction`), real case records from Phase
063 (`packages/caselifecycle`), the reasoning-quality
scoring/expert-review shape from Phase 062 (`packages/reasoningeval`),
the non-binding-workflow guardrail from Phase 057
(`packages/guardrail`), the Finding/triage/status-machine pattern from
Phase 084 (`packages/vulnmanagement`), the certification report shape
from Phase 080 (`packages/accessgovernance`), and the durable audit
trail from Phase 077 (`packages/auditlog`) -- into a single,
tenant-scoped, controlled pilot deployment with a structured feedback
loop: `packages/pilot`.

## Goal

Run a controlled pilot with real users and structured feedback:
provision a tenant-scoped pilot deployment, onboard a pilot
jurisdiction and its corpus, run supervised cases against real
`packages/caselifecycle.Case` records, collect structured expert
feedback, measure reasoning quality and trust, triage and prioritize
the issues that surface, apply and verify high-priority refinements,
validate that the non-binding workflow actually holds in practice, and
capture a final report of everything that happened.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/setup` (Phase 008) | Deployment-provisioning wizard/service shape | `PilotDeployment`'s `Provisioning` state names the conceptual precedent; this package does not import `packages/setup` |
| `packages/jurisdiction` (Phase 007) | Jurisdiction onboarding, lookup, and validation | `PilotDeployment.JurisdictionCode` references a jurisdiction by string code, mirroring the same reference convention already used by `packages/corpusupdater`, `packages/issueagent`, and `packages/reasoningeval`; `PilotDeployment`'s `CorpusOnboarding` state names the conceptual precedent |
| `packages/caselifecycle` (Phase 063) | The one real `Case` record in this codebase | `PilotCase.CaseID` references it by ID only, mirroring `packages/accessgovernance.CaseGrant`'s identical CaseID-reference convention |
| `packages/reasoningeval` (Phase 062) | `DimensionName`/`QualityScore`/`ExpertReview` -- automated scoring plus human review, stored side by side | `FeedbackEntry`/`DimensionRating` mirror the `DimensionName` shape explicitly (see below for why not imported); `QualitySummary`/`AggregateQuality` fold in automated scores via the small `QualityScoreLike` resolver interface |
| `packages/guardrail` (Phase 057) | `CheckText`/`HasDisclaimer` -- the only non-binding-disclaimer and verdict-language enforcement in this codebase | `Engine.ValidateNonBindingCompliance` calls them directly against real pilot-case opinion text, proven by a harness-that-can-fail test suite |
| `packages/vulnmanagement` (Phase 084) | `Finding`/`TriageDecision`/status-machine shape | `PilotFinding`/`RefinementRecord` mirror the same shape (Open -> Triaged -> InProgress -> Resolved/WontFix), applied to pilot-feedback-driven issues instead of security findings |
| `packages/accessgovernance` (Phase 080, by reference only) | `Certify`/`ExportReport` aggregation shape | `Report`/`CaptureReport`/`ExportReport` follow the identical CSV/JSON export shape, without importing `packages/accessgovernance` |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store | `AuditSink` projects every deployment status change, finding triage, and report capture into that same store; no parallel log |

## Pilot deployment lifecycle (tasks 1-2)

`PilotDeployment`/`DeploymentStatus` (types.go) is a tenant-scoped
controlled pilot: a `Name`, a `JurisdictionCode` (referencing a
`packages/jurisdiction` jurisdiction by string code -- see "Why
`JurisdictionCode` stays a string" below), a `Status`, and a
`StartDate`/`EndDate` window. `Status` follows a strictly linear
`Provisioning -> CorpusOnboarding -> Active -> Concluded` lifecycle,
enforced by `CanTransitionDeployment` and applied by
`Engine.TransitionDeployment` -- there is no sanctioned path backward;
a pilot that needs to re-onboard a corpus starts a new
`PilotDeployment` record entirely. `Engine.ProvisionDeployment` (task
1) always starts a fresh deployment at `DeploymentStatusProvisioning`
regardless of what a caller supplies. `Engine.TransitionDeployment`
(task 2's corpus-onboarding move and every later lifecycle transition)
records every attempt via `AuditSink` regardless of outcome, and fails
closed with `ErrIllegalStatusTransition` before any state change if
the requested move is not legal.

### Why `JurisdictionCode` stays a string

`packages/jurisdiction`'s own `Jurisdiction` type carries a `uuid.UUID`
primary key plus a rich `CourtLevel`/`LegalFamily`/`ProceduralRules`
structure this package has no need of -- a pilot deployment only needs
to *name* which jurisdiction it onboards, not resolve or validate the
full jurisdiction record itself. `JurisdictionCode string` mirrors the
identical reference convention already established across this
codebase (`packages/corpusupdater.CorpusUpdateJob.JurisdictionCode`,
`packages/issueagent`, `packages/reasoningeval.ScoreInput.JurisdictionCode`,
`packages/analytics`, `packages/alerting`) rather than taking on a
hard dependency on `packages/jurisdiction`'s Go types.

## Supervised pilot cases (task 3)

`PilotCase` (types.go) references a real `packages/caselifecycle.Case`
by ID only -- never by value or import -- mirroring
`packages/accessgovernance.CaseGrant`'s `CaseID`-reference convention
exactly. It tracks supervision metadata: an assigned
`SupervisorUserID` and an `OutcomeObserved` flag set via the
idempotent `Engine.MarkOutcomeObserved`. `Engine.AssignCase` requires
its target `PilotDeployment` to already be `DeploymentStatusActive` --
supervised case work only runs once a pilot's jurisdiction/corpus
onboarding has actually completed.

## Structured expert feedback (task 4)

`FeedbackEntry`/`DimensionName`/`DimensionRating`/`TrustRating`
(feedback.go) capture a reviewer's structured assessment of a single
`PilotCase`: reviewer identity, a set of `[0.0, 1.0]` per-dimension
ratings, a discrete `1`-`5` `TrustRating`, and free-text comments.

### Why `FeedbackEntry` mirrors `reasoningeval.DimensionName` instead of importing it

`packages/reasoningeval`'s own `go.mod` requires
`packages/grounding`, `packages/synthesisagent`, `packages/treeassembly`,
`packages/vectorindex`, `packages/hybridretrieval`, the neo4j Go
driver, and a dozen more reasoning-stack packages -- the full
dependency tree behind `DimensionScorer`/`Rubric`/`Score`, none of
which this phase's feedback-collection surface needs. Importing
`reasoningeval` here purely to reuse a three-field
`DimensionName`/`QualityScore` shape would drag every one of those
packages into `packages/pilot`'s own module graph for no behavioral
benefit. Instead, `DimensionName` and its three shared constants
(`grounding`, `citation`, `coherence`) are mirrored explicitly -- kept
string-identical by convention, plus a pilot-specific `usefulness`
axis with no `reasoningeval` counterpart -- and `TrustRating` is a
discrete `1`-`5` scale distinct from the continuous `[0,1]`
per-dimension scores, since a human answering "would you trust this"
reasons in a small number of buckets, not a float.

## Measuring reasoning quality and trust (task 5)

`QualitySummary`/`Engine.AggregateQuality`/`BuildQualitySummary`
(aggregate.go) perform a real aggregation over every collected
`FeedbackEntry` for a deployment: arithmetic means of overall score,
per-dimension score, and trust rating, plus a full trust-value
distribution. Automated scores compose through the small
`QualityScoreLike` resolver interface (`PilotCaseIDValue()`,
`OverallValue()`) -- mirroring `packages/reasoningeval.OpinionLike`/
`GroundingReportLike`'s own decoupling precedent -- rather than a
direct `reasoningeval.QualityScore` import; a caller holding a real one
adapts it via `ReasoningEvalQualityScoreAdapter`. Automated and
human-reviewer scores are aggregated *side by side* in the returned
`QualitySummary` (`AvgOverallFeedbackScore` vs. `AvgAutomatedOverall`),
never merged into a single number -- exactly the guarantee
`packages/reasoningeval.ExpertReview`'s own doc comment establishes for
why automated and human assessments are stored separately.

## Triage and prioritize fixes (task 6)

`PilotFinding`/`Priority`/`FindingStatus` (finding.go) mirror
`packages/vulnmanagement.Finding`'s status-machine shape -- `Open ->
Triaged -> InProgress -> Resolved/WontFix`, enforced by
`CanTransitionFinding` -- applied to pilot-feedback-driven issues
(sourced from one or more `FeedbackEntry` IDs via
`SourceFeedbackIDs`) rather than scanner findings, with a `Priority`
(`Critical`/`High`/`Medium`/`Low`) instead of a security `Severity`.
`Engine.RecordFinding` always starts a fresh finding at
`FindingStatusOpen`. `Engine.TriageFinding` mirrors
`packages/vulnmanagement.Engine.Triage`'s accountability pattern
exactly: a `Priority` + new `FindingStatus` + non-blank notes,
recorded via `AuditSink` regardless of outcome, rejecting an illegal
`CanTransitionFinding` move before any state change.

## Apply and verify high-priority refinements (task 7)

`RefinementRecord` (refinement.go) tracks what was changed, which
`PilotFinding` it addresses, and a `VerifiedFixed` bool + verification
note. The load-bearing guarantee here is real state tracking, not just
a comment: `Engine.RecordRefinement` requires the referenced
`PilotFinding` to have already reached at least `FindingStatusTriaged`
(`FindingStatus.IsAtLeastTriaged`), checked against the finding's live
stored `Status` -- an attempt against a still-`Open` finding fails with
`ErrFindingNotTriaged` before the refinement is ever persisted (see
`TestEngine_RecordRefinement_RejectsUntriaggedFinding`). Recording a
refinement against a `Triaged` finding moves it to `InProgress`; a
fresh `RefinementRecord` can never be pre-verified by the caller (the
verification fields are always reset on `RecordRefinement`).
`Engine.VerifyRefinement` requires a non-blank verification note and,
when the finding is still `InProgress`, moves it to `Resolved` --
verifying a refinement has a real consequence on its finding's state,
not a bare boolean flip with no further effect.

## Validate non-binding workflow in practice (task 8)

`ComplianceResult`/`Engine.ValidateNonBindingCompliance`
(compliance.go) call straight into `packages/guardrail`'s own real
checks -- `guardrail.CheckText` (verdict/directive-language rejection)
and `guardrail.HasDisclaimer` (mandatory disclaimer presence) -- rather
than reimplementing any label or verdict-language logic in this
package. This is proven, not merely asserted: `compliance_test.go`
mirrors Phase 086's security-testing harness principle -- a
deliberately verdict-worded fixture ("The court hereby rules that the
defendant is guilty...") is asserted to fail, and a properly-labeled
draft-analysis fixture (built with `guardrail.RequireDisclaimer`'s
actual disclaimer text) is asserted to pass. Both fixtures were also
independently exercised against `guardrail.CheckText`/`HasDisclaimer`
directly outside the test harness while writing this phase, to confirm
the pass/fail split reflects real guardrail behavior rather than a
coincidental match in the test assertions.

## Capture pilot report (task 9)

`Report`/`Period`/`ExportFormat`/`Engine.CaptureReport`/`BuildReport`/
`ExportReport` (report.go) mirror
`packages/accessgovernance.Certify`/`ExportReport`'s CSV/JSON export
shape exactly. `Engine.CaptureReport` aggregates, for one
`PilotDeployment` within a `Period`: deployment status, case count and
observed-outcome count, a full `QualitySummary` (task 5), findings by
`Priority` and `FindingStatus` (task 6), refinements applied/verified
(task 7), and a non-binding-compliance rate computed from
caller-supplied `ComplianceResult` values (task 8) -- since this
package does not persist the raw opinion texts
`ValidateNonBindingCompliance` was run against, a caller passes the
results it already collected rather than `CaptureReport` re-running
those checks itself. `BuildReport` is the pure aggregation function
operating on already-fetched data, mirroring
`packages/vulnmanagement.BuildReport`'s identical convention.
`ExportReport` renders CSV (one section per sub-aggregate: `summary`,
`findings_by_priority`, `findings_by_status`) or JSON.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact
`PermViewAlerting`/`PermManageAlerting` precedent from Phase 096:

- `pilot:view` (`identity.PermViewPilot`): read-only access to pilot
  deployments, cases, feedback, findings, refinements, and the
  captured report.
- `pilot:manage` (`identity.PermManagePilot`): provision/transition
  deployments, assign/observe cases, submit feedback, triage findings,
  record/verify refinements, and capture a report.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

Two new migration pairs, continuing directly after
`000043_enable_rls_alerting`:

- `packages/persistence/migrations/000044_create_pilot.up.sql` /
  `.down.sql` create five tables: `pilot_deployments`, `pilot_cases`,
  `pilot_feedback_entries`, `pilot_findings`, and
  `pilot_refinement_records`. Unlike `packages/compliance`'s shared
  `compliance_controls` catalogue, every type this phase introduces is
  genuinely per-tenant, so every table here carries a `tenant_id`
  column -- there is no shared-reference-data table in this phase.
- `packages/persistence/migrations/000045_enable_rls_pilot.up.sql` /
  `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on all five tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/compliance` and `packages/vulnmanagement`, with Row-Level
Security enforcing tenant isolation at the database layer in addition
to each repository's own application-level `requireMatchingTenant`
guard.

## What is explicitly reused, not duplicated

- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission` remain
  the coarse RBAC gate every `Engine` method calls through before
  doing anything pilot-specific.
- `packages/guardrail.CheckText`/`HasDisclaimer` remain the only
  non-binding-disclaimer and verdict-language enforcement mechanisms in
  this codebase; `ValidateNonBindingCompliance` calls them directly.
- `packages/caselifecycle.Case` remains the only case record in this
  codebase; `PilotCase` references it by ID only.
- `packages/reasoningeval`'s `DimensionName`/`QualityScore`/
  `ExpertReview` shape is the reference `FeedbackEntry`/
  `QualitySummary` follow, not a dependency -- see "Why `FeedbackEntry`
  mirrors `reasoningeval.DimensionName` instead of importing it"
  above.
- `packages/vulnmanagement`'s `Finding`/`TriageDecision`/status-machine
  shape is the reference `PilotFinding`/`RefinementRecord` follow, not
  a dependency -- this package does not import `packages/vulnmanagement`.
- `packages/accessgovernance`'s `Certify`/`ExportReport` aggregation
  shape is the reference `Report`/`CaptureReport`/`ExportReport`
  follow, not a dependency -- this package does not import
  `packages/accessgovernance`.
- `packages/setup`'s deployment-provisioning wizard and
  `packages/jurisdiction`'s onboarding/lookup machinery are the
  conceptual precedents `PilotDeployment`'s `Provisioning`/
  `CorpusOnboarding` states name, not dependencies -- this package does
  not import either.
